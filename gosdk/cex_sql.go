package gosdk

import (
	"context"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	cexOrderBookMissPrecisionThresholdNs                    = int64(time.Millisecond)
	cexOrderBookDecodeError                                 = "decode_error"
	cexOrderBookReadResultQueryError                        = "query_error"
	errEmptyCEXOrderBookReadResult       sdkerrors.SDKError = "empty cex order book read result"
	errCEXSQLiteReaderNotInit            sdkerrors.SDKError = "cex sqlite reader is not initialized"
	// ErrCEXOrderBookNotFound is returned through CEXOrderBookReadError when an
	// exact CEX order-book snapshot ref has no matching SQLite row.
	ErrCEXOrderBookNotFound sdkerrors.SDKError = "cex order book not found"
)

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
	reader *cexOrderBookFastReader
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
	reader, err := openCEXOrderBookFastReader(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cex sqlite fast reader %s: %w", dbPath, err)
	}

	return &CEXDataAccessSQL{reader: reader}, nil
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
			"read order book %s/%s@%d: %w",
			exchange,
			symbol,
			fetchedAt,
			errEmptyCEXOrderBookReadResult,
		)
	}

	return snapshots[0], errs[0]
}

// ReadCEXOrderBooks reads a batch of exact order-book refs from the CEX SQLite DB
// through fixed prepared point statements.
func (c *CEXDataAccessSQL) ReadCEXOrderBooks(
	ctx context.Context,
	refs []apptypes.CEXOrderBookRef,
) ([]*apptypes.CEXOrderBookSnapshot, []error) {
	if c == nil || c.reader == nil {
		snapshots := make([]*apptypes.CEXOrderBookSnapshot, len(refs))

		errs := make([]error, len(refs))
		for i, ref := range refs {
			diag := newCEXOrderBookReadDiagnostic(ref, 0)
			diag.Result = cexOrderBookReadResultQueryError
			errs[i] = wrapCEXOrderBookReadError(
				ctx,
				diag,
				errCEXSQLiteReaderNotInit,
			)
		}

		return snapshots, errs
	}

	return c.reader.readCEXOrderBooks(ctx, refs)
}

// Close closes the underlying SQLite database.
func (c *CEXDataAccessSQL) Close() {
	if c != nil && c.reader != nil {
		if err := c.reader.Close(); err != nil {
			log.Warn().Err(err).Msg("close cex sqlite fast reader")
		}
	}
}

func decodeCEXOrderBookRow(
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

	if err := cbor.Unmarshal(row.bidsRaw, &snapshot.Bids); err != nil {
		diag.Result = cexOrderBookDecodeError
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

	if err := cbor.Unmarshal(row.asksRaw, &snapshot.Asks); err != nil {
		diag.Result = cexOrderBookDecodeError
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

func wrapCEXOrderBookReadError(
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

func classifyCEXOrderBookMiss(diag CEXOrderBookReadDiagnostic) string {
	var nearestDelta int64

	switch {
	case diag.NearestOlderDeltaNs > 0 && diag.NearestNewerDeltaNs > 0:
		nearestDelta = min(diag.NearestOlderDeltaNs, diag.NearestNewerDeltaNs)
	case diag.NearestOlderDeltaNs > 0:
		nearestDelta = diag.NearestOlderDeltaNs
	case diag.NearestNewerDeltaNs > 0:
		nearestDelta = diag.NearestNewerDeltaNs
	default:
		nearestDelta = 0
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
	event := log.Ctx(ctx).Debug()
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
