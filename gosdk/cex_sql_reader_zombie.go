package gosdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	zsqlite "zombiezen.com/go/sqlite"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/internal/sqlitez"
)

const (
	cexOrderBookPairIDQuery = `
SELECT id
FROM cex_orderbook_pairs_v3
WHERE exchange_id = ? AND market_type_id = ? AND symbol_id = ?
LIMIT 1`
	cexOrderBookExactReadQuery = `
SELECT ob.last_update_id, ob.bids, ob.asks, ex.name, sym.symbol
FROM cex_orderbooks_v6 AS ob
JOIN cex_orderbook_pairs_v3 AS pair ON pair.id = ob.pair_id
JOIN cex_exchange_dim AS ex ON ex.id = pair.exchange_id
JOIN cex_symbol_dim AS sym ON sym.id = pair.symbol_id
WHERE ob.pair_id = ? AND ob.fetched_at = ?
LIMIT 1`
	cexOrderBookNearestOlderQuery = `
SELECT fetched_at
FROM cex_orderbooks_v6
WHERE pair_id = ? AND fetched_at < ?
ORDER BY fetched_at DESC
LIMIT 1`
	cexOrderBookNearestNewerQuery = `
SELECT fetched_at
FROM cex_orderbooks_v6
WHERE pair_id = ? AND fetched_at > ?
ORDER BY fetched_at ASC
LIMIT 1`
	cexOrderBookResolveUniqueMarketQuery = `
SELECT pair.exchange_id, pair.market_type_id, pair.symbol_id, ex.name, mt.name, sym.symbol
FROM cex_orderbook_pairs_v3 AS pair
JOIN cex_exchange_dim AS ex ON ex.id = pair.exchange_id
JOIN cex_market_type_dim AS mt ON mt.id = pair.market_type_id
JOIN cex_symbol_dim AS sym ON sym.id = pair.symbol_id
WHERE ex.name = ? AND sym.symbol = ?
ORDER BY pair.market_type_id
LIMIT 2`
	cexOrderBookResolveExplicitMarketQuery = `
SELECT pair.exchange_id, pair.market_type_id, pair.symbol_id, ex.name, mt.name, sym.symbol
FROM cex_orderbook_pairs_v3 AS pair
JOIN cex_exchange_dim AS ex ON ex.id = pair.exchange_id
JOIN cex_market_type_dim AS mt ON mt.id = pair.market_type_id
JOIN cex_symbol_dim AS sym ON sym.id = pair.symbol_id
WHERE ex.name = ? AND mt.name = ? AND sym.symbol = ?
LIMIT 1`
)

type cexOrderBookFastReader struct {
	mu sync.Mutex

	conn           *zsqlite.Conn
	pair           *zsqlite.Stmt
	exact          *zsqlite.Stmt
	older          *zsqlite.Stmt
	newer          *zsqlite.Stmt
	uniqueMarket   *zsqlite.Stmt
	explicitMarket *zsqlite.Stmt

	bidsBuf []byte
	asksBuf []byte
}

func openCEXOrderBookFastReader(
	ctx context.Context,
	dbPath string,
) (*cexOrderBookFastReader, error) {
	conn, err := sqlitez.OpenConn(ctx, dbPath, sqlitez.OpenOptions{
		QueryOnly:                true,
		DisableWALAutoCheckpoint: true,
	})
	if err != nil {
		return nil, err
	}

	return &cexOrderBookFastReader{
		conn:           conn,
		pair:           conn.Prep(cexOrderBookPairIDQuery),
		exact:          conn.Prep(cexOrderBookExactReadQuery),
		older:          conn.Prep(cexOrderBookNearestOlderQuery),
		newer:          conn.Prep(cexOrderBookNearestNewerQuery),
		uniqueMarket:   conn.Prep(cexOrderBookResolveUniqueMarketQuery),
		explicitMarket: conn.Prep(cexOrderBookResolveExplicitMarketQuery),
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

func (r *cexOrderBookFastReader) readCEXOrderBookByLabels(
	ctx context.Context,
	exchange string,
	marketType string,
	symbol string,
	fetchedAt int64,
) (*apptypes.CEXOrderBookSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldDone := r.conn.SetInterrupt(ctx.Done())
	defer r.conn.SetInterrupt(oldDone)

	ref, err := r.resolveCEXOrderBookRefByLabels(exchange, marketType, symbol, fetchedAt)
	if err != nil {
		diag := CEXOrderBookReadDiagnostic{
			Exchange:             exchange,
			Symbol:               symbol,
			RequestedFetchedAtNs: fetchedAt,
			RequestedFetchedAtMs: fetchedAt / int64(time.Millisecond),
			Result:               cexOrderBookReadResultQueryError,
		}

		return nil, wrapCEXOrderBookReadError(ctx, diag, err)
	}

	diag := newCEXOrderBookReadDiagnostic(ref, 0)
	row, queryDuration, queryErr := r.readCEXOrderBookRow(ref)

	diag.QueryDuration = queryDuration
	if queryErr != nil {
		diag.Result = cexOrderBookReadResultQueryError
		diag.TotalDuration = queryDuration

		return nil, wrapCEXOrderBookReadError(ctx, diag, queryErr)
	}

	if row == nil {
		return nil, r.readCEXOrderBookMiss(ctx, diag, ref)
	}

	return decodeCEXOrderBookRow(ctx, diag, *row)
}

func (r *cexOrderBookFastReader) resolveCEXOrderBookRefByLabels(
	exchange string,
	marketType string,
	symbol string,
	fetchedAt int64,
) (apptypes.CEXOrderBookRef, error) {
	if marketType != "" {
		ref, ok, err := resolveExplicitCEXOrderBookMarket(
			r.explicitMarket,
			exchange,
			marketType,
			symbol,
			fetchedAt,
		)
		if err != nil {
			return apptypes.CEXOrderBookRef{}, err
		}

		if !ok {
			return apptypes.CEXOrderBookRef{}, ErrCEXOrderBookNotFound
		}

		return ref, nil
	}

	ref, ok, err := resolveUniqueCEXOrderBookMarket(
		r.uniqueMarket,
		exchange,
		symbol,
		fetchedAt,
	)
	if err != nil {
		return apptypes.CEXOrderBookRef{}, err
	}

	if !ok {
		return apptypes.CEXOrderBookRef{}, ErrCEXOrderBookNotFound
	}

	return ref, nil
}

func (r *cexOrderBookFastReader) readCEXOrderBookRow(
	ref apptypes.CEXOrderBookRef,
) (*cexOrderBookRow, time.Duration, error) {
	queryStart := time.Now()

	if ref.ExchangeID == 0 || ref.MarketTypeID == 0 || ref.SymbolID == 0 {
		return nil, time.Since(queryStart), ErrCEXLegacyOrderBookRef
	}

	pairID, ok, err := readCEXOrderBookPairID(
		r.pair,
		ref.ExchangeID,
		ref.MarketTypeID,
		ref.SymbolID,
	)
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
	exchange := stmt.ColumnText(3)
	symbol := stmt.ColumnText(4)

	row := &cexOrderBookRow{
		key: cexOrderBookRefKey{
			exchange:     cexOrderBookExchange(exchange),
			symbol:       cexOrderBookSymbol(symbol),
			exchangeID:   ref.ExchangeID,
			marketTypeID: ref.MarketTypeID,
			symbolID:     ref.SymbolID,
			fetchedAt:    ref.FetchedAt,
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
	older, newer, probeErr := r.probeNearestOrderBookRows(
		ref.ExchangeID,
		ref.MarketTypeID,
		ref.SymbolID,
		ref.FetchedAt,
	)

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
	exchangeID apptypes.CEXExchangeID,
	marketTypeID apptypes.CEXMarketTypeID,
	symbolID apptypes.CEXSymbolID,
	fetchedAt int64,
) (older int64, newer int64, err error) {
	pairID, ok, err := readCEXOrderBookPairID(r.pair, exchangeID, marketTypeID, symbolID)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"lookup pair exchange_id=%d market_type_id=%d symbol_id=%d: %w",
			exchangeID,
			marketTypeID,
			symbolID,
			err,
		)
	}

	if !ok {
		return 0, 0, nil
	}

	older, err = probeCEXOrderBookTimestamp(r.older, pairID, fetchedAt)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"probe older row exchange_id=%d market_type_id=%d symbol_id=%d@%d: %w",
			exchangeID,
			marketTypeID,
			symbolID,
			fetchedAt,
			err,
		)
	}

	newer, err = probeCEXOrderBookTimestamp(r.newer, pairID, fetchedAt)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"probe newer row exchange_id=%d market_type_id=%d symbol_id=%d@%d: %w",
			exchangeID,
			marketTypeID,
			symbolID,
			fetchedAt,
			err,
		)
	}

	return older, newer, nil
}

func readCEXOrderBookPairID(
	stmt *zsqlite.Stmt,
	exchangeID apptypes.CEXExchangeID,
	marketTypeID apptypes.CEXMarketTypeID,
	symbolID apptypes.CEXSymbolID,
) (int64, bool, error) {
	stmt.BindInt64(1, int64(exchangeID))
	stmt.BindInt64(2, int64(marketTypeID))
	stmt.BindInt64(3, int64(symbolID))

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

func resolveExplicitCEXOrderBookMarket(
	stmt *zsqlite.Stmt,
	exchange string,
	marketType string,
	symbol string,
	fetchedAt int64,
) (apptypes.CEXOrderBookRef, bool, error) {
	stmt.BindText(1, exchange)
	stmt.BindText(2, marketType)
	stmt.BindText(3, symbol)

	hasRow, err := stmt.Step()
	if err != nil {
		_ = resetCEXOrderBookStmt(stmt)

		return apptypes.CEXOrderBookRef{}, false, err
	}

	if !hasRow {
		if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
			return apptypes.CEXOrderBookRef{}, false, resetErr
		}

		return apptypes.CEXOrderBookRef{}, false, nil
	}

	ref := readCEXOrderBookResolvedRef(stmt, fetchedAt)
	if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
		return apptypes.CEXOrderBookRef{}, false, resetErr
	}

	return ref, true, nil
}

func resolveUniqueCEXOrderBookMarket(
	stmt *zsqlite.Stmt,
	exchange string,
	symbol string,
	fetchedAt int64,
) (apptypes.CEXOrderBookRef, bool, error) {
	stmt.BindText(1, exchange)
	stmt.BindText(2, symbol)

	hasRow, err := stmt.Step()
	if err != nil {
		_ = resetCEXOrderBookStmt(stmt)

		return apptypes.CEXOrderBookRef{}, false, err
	}

	if !hasRow {
		if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
			return apptypes.CEXOrderBookRef{}, false, resetErr
		}

		return apptypes.CEXOrderBookRef{}, false, nil
	}

	ref := readCEXOrderBookResolvedRef(stmt, fetchedAt)
	hasSecondRow, err := stmt.Step()
	if err != nil {
		_ = resetCEXOrderBookStmt(stmt)

		return apptypes.CEXOrderBookRef{}, false, err
	}

	if hasSecondRow {
		if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
			return apptypes.CEXOrderBookRef{}, false, resetErr
		}

		return apptypes.CEXOrderBookRef{}, false, ErrCEXOrderBookAmbiguousMarket
	}

	if resetErr := resetCEXOrderBookStmt(stmt); resetErr != nil {
		return apptypes.CEXOrderBookRef{}, false, resetErr
	}

	return ref, true, nil
}

func readCEXOrderBookResolvedRef(stmt *zsqlite.Stmt, fetchedAt int64) apptypes.CEXOrderBookRef {
	return apptypes.CEXOrderBookRef{
		Exchange:     stmt.ColumnText(3),
		Symbol:       stmt.ColumnText(5),
		FetchedAt:    fetchedAt,
		ExchangeID:   apptypes.CEXExchangeID(stmt.ColumnInt64(0)),
		MarketTypeID: apptypes.CEXMarketTypeID(stmt.ColumnInt64(1)),
		SymbolID:     apptypes.CEXSymbolID(stmt.ColumnInt64(2)),
	}
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
