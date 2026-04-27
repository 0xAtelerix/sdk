package gosdk

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/goccy/go-json"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const cexOrderBookMissPrecisionThresholdNs = int64(time.Millisecond)

// CEXOrderBookReadDiagnostic captures one exact-order-book read attempt.
type CEXOrderBookReadDiagnostic struct {
	Exchange              string
	Symbol                string
	RequestedFetchedAtNs  int64
	RequestedFetchedAtMs  int64
	Result                string
	MissHint              string
	QueryDuration         time.Duration
	BidsUnmarshalDuration time.Duration
	AsksUnmarshalDuration time.Duration
	NearestProbeDuration  time.Duration
	NearestOlderFetchedAt int64
	NearestNewerFetchedAt int64
	NearestOlderDeltaNs   int64
	NearestNewerDeltaNs   int64
	TotalDuration         time.Duration
}

// CEXOrderBookReadError preserves typed diagnostic context while remaining compatible
// with errors.Is/errors.As.
type CEXOrderBookReadError struct {
	Diagnostic CEXOrderBookReadDiagnostic
	Err        error
}

func (e *CEXOrderBookReadError) Error() string {
	if e == nil || e.Err == nil {
		return "cex order book read error"
	}

	return e.Err.Error()
}

func (e *CEXOrderBookReadError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// CEXDataAccessSQL implements CEXDataAccessor using SQLite
type CEXDataAccessSQL struct {
	db *sql.DB
}

type cexOrderBookExchange string

func (e cexOrderBookExchange) String() string {
	return string(e)
}

type cexOrderBookSymbol string

func (s cexOrderBookSymbol) String() string {
	return string(s)
}

type cexOrderBookRefKey struct {
	exchange  cexOrderBookExchange
	symbol    cexOrderBookSymbol
	fetchedAt int64
}

type cexOrderBookRow struct {
	key          cexOrderBookRefKey
	lastUpdateID int64
	bidsRaw      []byte
	asksRaw      []byte
}

// NewCEXDataAccessSQL opens the CEX SQLite database in read-only mode.
func NewCEXDataAccessSQL(ctx context.Context, dbPath string) (*CEXDataAccessSQL, error) {
	db, err := openSQLite(ctx, dbPath, "ro")
	if err != nil {
		return nil, fmt.Errorf("open cex sqlite %s: %w", dbPath, err)
	}

	return &CEXDataAccessSQL{db: db}, nil
}

// ReadCEXOrderBook reads a specific order book snapshot by exchange, symbol, and fetchedAt timestamp.
func (c *CEXDataAccessSQL) ReadCEXOrderBook(
	ctx context.Context,
	exchange string,
	symbol string,
	fetchedAt int64,
) (*apptypes.CEXOrderBookSnapshot, error) {
	snapshots, errs := c.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{{
		Exchange:  exchange,
		Symbol:    symbol,
		FetchedAt: fetchedAt,
	}})
	if len(errs) == 0 {
		return nil, fmt.Errorf(
			"read order book %s/%s@%d: empty read result",
			exchange,
			symbol,
			fetchedAt,
		)
	}

	return snapshots[0], errs[0]
}

// ReadCEXOrderBooks reads a batch of exact order-book refs inside one read-only SQLite tx.
func (c *CEXDataAccessSQL) ReadCEXOrderBooks(
	ctx context.Context,
	refs []apptypes.CEXOrderBookRef,
) ([]*apptypes.CEXOrderBookSnapshot, []error) {
	snapshots := make([]*apptypes.CEXOrderBookSnapshot, len(refs))

	errs := make([]error, len(refs))
	if len(refs) == 0 {
		return snapshots, errs
	}

	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		for i, ref := range refs {
			errs[i] = fmt.Errorf(
				"begin cex read tx %s/%s@%d: %w",
				ref.Exchange,
				ref.Symbol,
				ref.FetchedAt,
				err,
			)
		}

		return snapshots, errs
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil &&
			!errors.Is(rollbackErr, sql.ErrTxDone) {
			log.Ctx(ctx).Warn().Err(rollbackErr).Msg("rollback cex sqlite read tx")
		}
	}()

	rowsByRef, queryDuration, queryErr := c.readCEXOrderBookRowsTx(ctx, tx, refs)
	for i, ref := range refs {
		diag := newCEXOrderBookReadDiagnostic(ref, queryDuration)
		if queryErr != nil {
			diag.Result = "query_error"
			diag.TotalDuration = queryDuration
			errs[i] = c.wrapCEXOrderBookReadError(ctx, diag, queryErr)

			continue
		}

		row, ok := rowsByRef[cexOrderBookRefKey{
			exchange:  cexOrderBookExchange(ref.Exchange),
			symbol:    cexOrderBookSymbol(ref.Symbol),
			fetchedAt: ref.FetchedAt,
		}]
		if !ok {
			snapshots[i], errs[i] = c.readCEXOrderBookMissTx(ctx, tx, diag, ref)

			continue
		}

		snapshots[i], errs[i] = c.decodeCEXOrderBookRow(ctx, diag, row)
	}

	return snapshots, errs
}

func (c *CEXDataAccessSQL) decodeCEXOrderBookRow(
	ctx context.Context,
	diag CEXOrderBookReadDiagnostic,
	row cexOrderBookRow,
) (*apptypes.CEXOrderBookSnapshot, error) {
	start := time.Now()
	snapshot := &apptypes.CEXOrderBookSnapshot{
		Exchange:     row.key.exchange.String(),
		Symbol:       row.key.symbol.String(),
		LastUpdateID: row.lastUpdateID,
		FetchedAt:    row.key.fetchedAt,
	}

	bidsStart := time.Now()

	if err := json.Unmarshal(row.bidsRaw, &snapshot.Bids); err != nil {
		diag.Result = "decode_error"
		diag.BidsUnmarshalDuration = time.Since(bidsStart)
		diag.TotalDuration = diag.QueryDuration + time.Since(start)
		logCEXOrderBookRead(ctx, diag, err)

		return nil, &CEXOrderBookReadError{
			Diagnostic: diag,
			Err: fmt.Errorf(
				"unmarshal bids %s/%s: %w",
				row.key.exchange.String(),
				row.key.symbol.String(),
				err,
			),
		}
	}

	diag.BidsUnmarshalDuration = time.Since(bidsStart)

	asksStart := time.Now()

	if err := json.Unmarshal(row.asksRaw, &snapshot.Asks); err != nil {
		diag.Result = "decode_error"
		diag.AsksUnmarshalDuration = time.Since(asksStart)
		diag.TotalDuration = diag.QueryDuration + time.Since(start)
		logCEXOrderBookRead(ctx, diag, err)

		return nil, &CEXOrderBookReadError{
			Diagnostic: diag,
			Err: fmt.Errorf(
				"unmarshal asks %s/%s: %w",
				row.key.exchange.String(),
				row.key.symbol.String(),
				err,
			),
		}
	}

	diag.AsksUnmarshalDuration = time.Since(asksStart)
	diag.Result = "hit"
	diag.MissHint = "n/a"
	diag.TotalDuration = diag.QueryDuration + time.Since(start)
	logCEXOrderBookRead(ctx, diag, nil)

	return snapshot, nil
}

func (c *CEXDataAccessSQL) readCEXOrderBookRowsTx(
	ctx context.Context,
	tx *sql.Tx,
	refs []apptypes.CEXOrderBookRef,
) (map[cexOrderBookRefKey]cexOrderBookRow, time.Duration, error) {
	rowsByRef := make(map[cexOrderBookRefKey]cexOrderBookRow, len(refs))
	if len(refs) == 0 {
		return rowsByRef, 0, nil
	}

	conditions := make(sq.Or, 0, len(refs))

	seen := make(map[cexOrderBookRefKey]struct{}, len(refs))
	for _, ref := range refs {
		key := cexOrderBookRefKey{
			exchange:  cexOrderBookExchange(ref.Exchange),
			symbol:    cexOrderBookSymbol(ref.Symbol),
			fetchedAt: ref.FetchedAt,
		}
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		conditions = append(conditions, sq.And{
			sq.Eq{"exchange": key.exchange.String()},
			sq.Eq{"symbol": key.symbol.String()},
			sq.Eq{"fetched_at": ref.FetchedAt},
		})
	}

	query, args, err := sq.
		Select("exchange", "symbol", "fetched_at", "last_update_id", "bids", "asks").
		From("cex_orderbooks_v3").
		Where(conditions).
		ToSql()
	if err != nil {
		return rowsByRef, 0, fmt.Errorf("build cex orderbook batch read query: %w", err)
	}

	queryStart := time.Now()

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return rowsByRef, time.Since(queryStart), err
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Ctx(ctx).Warn().Err(closeErr).Msg("close cex orderbook batch rows")
		}
	}()

	for rows.Next() {
		var row cexOrderBookRow
		if scanErr := rows.Scan(
			&row.key.exchange,
			&row.key.symbol,
			&row.key.fetchedAt,
			&row.lastUpdateID,
			&row.bidsRaw,
			&row.asksRaw,
		); scanErr != nil {
			return rowsByRef, time.Since(queryStart), fmt.Errorf(
				"scan cex orderbook batch row: %w",
				scanErr,
			)
		}

		rowsByRef[row.key] = row
	}

	if err := rows.Err(); err != nil {
		return rowsByRef, time.Since(queryStart), fmt.Errorf(
			"iterate cex orderbook batch rows: %w",
			err,
		)
	}

	return rowsByRef, time.Since(queryStart), nil
}

func (c *CEXDataAccessSQL) readCEXOrderBookMissTx(
	ctx context.Context,
	tx *sql.Tx,
	diag CEXOrderBookReadDiagnostic,
	ref apptypes.CEXOrderBookRef,
) (*apptypes.CEXOrderBookSnapshot, error) {
	diag.Result = "no_row"
	probeStart := time.Now()
	older, newer, probeErr := c.probeNearestOrderBookRowsTx(
		ctx,
		tx,
		ref.Exchange,
		ref.Symbol,
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

	return nil, c.wrapCEXOrderBookReadError(ctx, diag, sql.ErrNoRows)
}

func newCEXOrderBookReadDiagnostic(
	ref apptypes.CEXOrderBookRef,
	queryDuration time.Duration,
) CEXOrderBookReadDiagnostic {
	return CEXOrderBookReadDiagnostic{
		Exchange:             ref.Exchange,
		Symbol:               ref.Symbol,
		RequestedFetchedAtNs: ref.FetchedAt,
		RequestedFetchedAtMs: ref.FetchedAt / int64(time.Millisecond),
		QueryDuration:        queryDuration,
	}
}

func (c *CEXDataAccessSQL) wrapCEXOrderBookReadError(
	ctx context.Context,
	diag CEXOrderBookReadDiagnostic,
	err error,
) *CEXOrderBookReadError {
	logCEXOrderBookRead(ctx, diag, err)

	return &CEXOrderBookReadError{
		Diagnostic: diag,
		Err: fmt.Errorf(
			"read order book %s/%s@%d: %w",
			diag.Exchange,
			diag.Symbol,
			diag.RequestedFetchedAtNs,
			err,
		),
	}
}

func (c *CEXDataAccessSQL) probeNearestOrderBookRowsTx(
	ctx context.Context,
	tx *sql.Tx,
	exchange string,
	symbol string,
	fetchedAt int64,
) (older int64, newer int64, err error) {
	if err = tx.QueryRowContext(ctx, `
		SELECT fetched_at
		FROM cex_orderbooks_v3
		WHERE exchange = ? AND symbol = ? AND fetched_at < ?
		ORDER BY fetched_at DESC
		LIMIT 1
	`, exchange, symbol, fetchedAt).Scan(&older); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("probe older row %s/%s@%d: %w", exchange, symbol, fetchedAt, err)
	}

	if err = tx.QueryRowContext(ctx, `
		SELECT fetched_at
		FROM cex_orderbooks_v3
		WHERE exchange = ? AND symbol = ? AND fetched_at > ?
		ORDER BY fetched_at ASC
		LIMIT 1
	`, exchange, symbol, fetchedAt).Scan(&newer); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("probe newer row %s/%s@%d: %w", exchange, symbol, fetchedAt, err)
	}

	return older, newer, nil
}

func classifyCEXOrderBookMiss(diag CEXOrderBookReadDiagnostic) string {
	nearestDelta := int64(0)

	switch {
	case diag.NearestOlderDeltaNs > 0 && diag.NearestNewerDeltaNs > 0:
		nearestDelta = min(diag.NearestOlderDeltaNs, diag.NearestNewerDeltaNs)
	case diag.NearestOlderDeltaNs > 0:
		nearestDelta = diag.NearestOlderDeltaNs
	case diag.NearestNewerDeltaNs > 0:
		nearestDelta = diag.NearestNewerDeltaNs
	}

	switch {
	case diag.NearestOlderFetchedAt == 0 && diag.NearestNewerFetchedAt == 0:
		return "true_absence"
	case nearestDelta > 0 && nearestDelta <= cexOrderBookMissPrecisionThresholdNs:
		return "precision_mismatch"
	default:
		return "visibility_or_reference_gap"
	}
}

func logCEXOrderBookRead(ctx context.Context, diag CEXOrderBookReadDiagnostic, err error) {
	event := log.Ctx(ctx).Info()
	if err != nil || diag.TotalDuration > 50*time.Millisecond {
		event = log.Ctx(ctx).Warn()
	}

	event.
		Str("path", "cex_snapshot_read_attempt").
		Str("exchange", diag.Exchange).
		Str("symbol", diag.Symbol).
		Int64("requested_fetched_at_ns", diag.RequestedFetchedAtNs).
		Int64("requested_fetched_at_ms", diag.RequestedFetchedAtMs).
		Str("result", diag.Result).
		Str("miss_hint", diag.MissHint).
		Int64("query_duration_ms", diag.QueryDuration.Milliseconds()).
		Int64("bids_unmarshal_ms", diag.BidsUnmarshalDuration.Milliseconds()).
		Int64("asks_unmarshal_ms", diag.AsksUnmarshalDuration.Milliseconds()).
		Int64("nearest_probe_duration_ms", diag.NearestProbeDuration.Milliseconds()).
		Int64("nearest_older_fetched_at_ns", diag.NearestOlderFetchedAt).
		Int64("nearest_newer_fetched_at_ns", diag.NearestNewerFetchedAt).
		Int64("nearest_older_delta_ns", diag.NearestOlderDeltaNs).
		Int64("nearest_newer_delta_ns", diag.NearestNewerDeltaNs).
		Int64("total_duration_ms", diag.TotalDuration.Milliseconds()).
		Err(err).
		Msg("cex_snapshot_read")
}

// Close closes the underlying SQLite database.
func (c *CEXDataAccessSQL) Close() {
	if c.db != nil {
		c.db.Close()
	}
}
