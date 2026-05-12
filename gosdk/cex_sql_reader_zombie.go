package gosdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/internal/sqlitez"
	zsqlite "zombiezen.com/go/sqlite"
)

const (
	cexOrderBookPairIDQuery = `
SELECT id
FROM cex_pairs_v2
WHERE exchange = ? AND symbol = ?
LIMIT 1`
	cexOrderBookExactReadQuery = `
SELECT last_update_id, bids, asks
FROM cex_orderbooks_v5
WHERE pair_id = ? AND fetched_at = ?
LIMIT 1`
	cexOrderBookNearestOlderQuery = `
SELECT fetched_at
FROM cex_orderbooks_v5
WHERE pair_id = ? AND fetched_at < ?
ORDER BY fetched_at DESC
LIMIT 1`
	cexOrderBookNearestNewerQuery = `
SELECT fetched_at
FROM cex_orderbooks_v5
WHERE pair_id = ? AND fetched_at > ?
ORDER BY fetched_at ASC
LIMIT 1`
)

type cexOrderBookFastReader struct {
	mu sync.Mutex

	conn  *zsqlite.Conn
	pair  *zsqlite.Stmt
	exact *zsqlite.Stmt
	older *zsqlite.Stmt
	newer *zsqlite.Stmt

	bidsBuf []byte
	asksBuf []byte
}

func openCEXOrderBookFastReader(ctx context.Context, dbPath string) (*cexOrderBookFastReader, error) {
	conn, err := sqlitez.OpenConn(ctx, dbPath, "ro", sqlitez.OpenOptions{
		QueryOnly:                true,
		DisableWALAutoCheckpoint: true,
	})
	if err != nil {
		return nil, err
	}

	return &cexOrderBookFastReader{
		conn:  conn,
		pair:  conn.Prep(cexOrderBookPairIDQuery),
		exact: conn.Prep(cexOrderBookExactReadQuery),
		older: conn.Prep(cexOrderBookNearestOlderQuery),
		newer: conn.Prep(cexOrderBookNearestNewerQuery),
	}, nil
}

func (r *cexOrderBookFastReader) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}

	return r.conn.Close()
}

func (r *cexOrderBookFastReader) readCEXOrderBooks(
	ctx context.Context,
	refs []apptypes.CEXOrderBookRef,
) ([]*apptypes.CEXOrderBookSnapshot, []error) {
	snapshots := make([]*apptypes.CEXOrderBookSnapshot, len(refs))
	errs := make([]error, len(refs))
	if len(refs) == 0 {
		return snapshots, errs
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	oldDone := r.conn.SetInterrupt(ctx.Done())
	defer r.conn.SetInterrupt(oldDone)

	for i, ref := range refs {
		diag := newCEXOrderBookReadDiagnostic(ref, 0)
		row, queryDuration, queryErr := r.readCEXOrderBookRow(ref)
		diag.QueryDuration = queryDuration
		if queryErr != nil {
			diag.Result = "query_error"
			diag.TotalDuration = queryDuration
			errs[i] = wrapCEXOrderBookReadError(ctx, diag, queryErr)

			continue
		}

		if row == nil {
			errs[i] = r.readCEXOrderBookMiss(ctx, diag, ref)

			continue
		}

		snapshots[i], errs[i] = decodeCEXOrderBookRow(ctx, diag, *row)
	}

	return snapshots, errs
}

func (r *cexOrderBookFastReader) readCEXOrderBookRow(
	ref apptypes.CEXOrderBookRef,
) (*cexOrderBookRow, time.Duration, error) {
	queryStart := time.Now()
	pairID, ok, err := readCEXOrderBookPairID(r.pair, ref.Exchange, ref.Symbol)
	if err != nil {
		return nil, time.Since(queryStart), err
	}

	if !ok {
		return nil, time.Since(queryStart), nil
	}

	stmt := r.exact
	stmt.BindInt64(1, pairID)
	stmt.BindInt64(2, ref.FetchedAt)

	hasRow, err := stmt.Step()
	queryDuration := time.Since(queryStart)
	if err != nil {
		_ = resetCEXOrderBookStmt(stmt)

		return nil, queryDuration, err
	}

	if !hasRow {
		if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
			return nil, queryDuration, resetErr
		}

		return nil, queryDuration, nil
	}

	bidsRaw := readSQLiteColumnBytes(stmt, 1, &r.bidsBuf)
	asksRaw := readSQLiteColumnBytes(stmt, 2, &r.asksBuf)

	row := &cexOrderBookRow{
		key: cexOrderBookRefKey{
			exchange:  cexOrderBookExchange(ref.Exchange),
			symbol:    cexOrderBookSymbol(ref.Symbol),
			fetchedAt: ref.FetchedAt,
		},
		lastUpdateID: stmt.ColumnInt64(0),
		bidsRaw:      bidsRaw,
		asksRaw:      asksRaw,
	}

	if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
		return nil, queryDuration, resetErr
	}

	return row, queryDuration, nil
}

func (r *cexOrderBookFastReader) readCEXOrderBookMiss(
	ctx context.Context,
	diag CEXOrderBookReadDiagnostic,
	ref apptypes.CEXOrderBookRef,
) error {
	diag.Result = "no_row"
	probeStart := time.Now()
	older, newer, probeErr := r.probeNearestOrderBookRows(ref.Exchange, ref.Symbol, ref.FetchedAt)
	diag.NearestProbeDuration = time.Since(probeStart)
	if probeErr == nil {
		diag.NearestOlderFetchedAt = older
		diag.NearestNewerFetchedAt = newer
		if older > 0 && ref.FetchedAt >= older {
			diag.NearestOlderDeltaNs = ref.FetchedAt - older
		}

		if newer > 0 && newer >= ref.FetchedAt {
			diag.NearestNewerDeltaNs = newer - ref.FetchedAt
		}

		diag.MissHint = classifyCEXOrderBookMiss(diag)
	} else {
		diag.MissHint = "nearest_probe_failed"
	}

	diag.TotalDuration = diag.QueryDuration + diag.NearestProbeDuration

	return wrapCEXOrderBookReadError(ctx, diag, ErrCEXOrderBookNotFound)
}

func (r *cexOrderBookFastReader) probeNearestOrderBookRows(
	exchange string,
	symbol string,
	fetchedAt int64,
) (older int64, newer int64, err error) {
	pairID, ok, err := readCEXOrderBookPairID(r.pair, exchange, symbol)
	if err != nil {
		return 0, 0, fmt.Errorf("lookup pair %s/%s: %w", exchange, symbol, err)
	}

	if !ok {
		return 0, 0, nil
	}

	older, err = probeCEXOrderBookTimestamp(r.older, pairID, fetchedAt)
	if err != nil {
		return 0, 0, fmt.Errorf("probe older row %s/%s@%d: %w", exchange, symbol, fetchedAt, err)
	}

	newer, err = probeCEXOrderBookTimestamp(r.newer, pairID, fetchedAt)
	if err != nil {
		return 0, 0, fmt.Errorf("probe newer row %s/%s@%d: %w", exchange, symbol, fetchedAt, err)
	}

	return older, newer, nil
}

func readCEXOrderBookPairID(stmt *zsqlite.Stmt, exchange string, symbol string) (int64, bool, error) {
	stmt.BindText(1, exchange)
	stmt.BindText(2, symbol)

	hasRow, err := stmt.Step()
	if err != nil {
		_ = resetCEXOrderBookStmt(stmt)

		return 0, false, err
	}

	if !hasRow {
		if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
			return 0, false, resetErr
		}

		return 0, false, nil
	}

	value := stmt.ColumnInt64(0)
	if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
		return 0, false, resetErr
	}

	return value, true, nil
}

func probeCEXOrderBookTimestamp(
	stmt *zsqlite.Stmt,
	pairID int64,
	fetchedAt int64,
) (int64, error) {
	stmt.BindInt64(1, pairID)
	stmt.BindInt64(2, fetchedAt)

	hasRow, err := stmt.Step()
	if err != nil {
		_ = resetCEXOrderBookStmt(stmt)

		return 0, err
	}

	if !hasRow {
		if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
			return 0, resetErr
		}

		return 0, nil
	}

	value := stmt.ColumnInt64(0)
	if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
		return 0, resetErr
	}

	return value, nil
}

func readSQLiteColumnBytes(stmt *zsqlite.Stmt, col int, buf *[]byte) []byte {
	size := stmt.ColumnLen(col)
	if cap(*buf) < size {
		*buf = make([]byte, size)
	}

	*buf = (*buf)[:size]
	stmt.ColumnBytes(col, *buf)

	return *buf
}

func resetCEXOrderBookStmt(stmt *zsqlite.Stmt) error {
	if resetErr := stmt.Reset(); resetErr != nil {
		_ = stmt.ClearBindings()

		return resetErr
	}

	return stmt.ClearBindings()
}
