package gosdk

import (
	"context"
	"crypto/sha256"
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
	errCEXOrderBookUnknownIdentity       sdkerrors.SDKError = "unknown cex order book registry identity"
	// ErrCEXLegacyOrderBookRef is returned when a cutover SQLite reader receives
	// an old string-keyed CEX order-book ref instead of numeric identity.
	ErrCEXLegacyOrderBookRef sdkerrors.SDKError = "legacy cex order book ref"
	// ErrCEXOrderBookAmbiguousMarket is returned by deprecated label-based
	// boundary reads when exchange+symbol resolves to multiple market types.
	ErrCEXOrderBookAmbiguousMarket sdkerrors.SDKError = "ambiguous cex order book market"
	// ErrCEXOrderBookNotFound is returned through CEXOrderBookReadError when an
	// exact CEX order-book snapshot ref has no matching SQLite row.
	ErrCEXOrderBookNotFound        sdkerrors.SDKError = "cex order book not found"
	ErrCEXMarketTradeBatchNotFound sdkerrors.SDKError = "cex market trade batch not found"
	ErrCEXMarketTradeBatchInvalid  sdkerrors.SDKError = "invalid cex market trade batch"
	ErrCEXCandleBatchNotFound      sdkerrors.SDKError = "cex candle batch not found"
	ErrCEXCandleBatchInvalid       sdkerrors.SDKError = "invalid cex candle batch"
)

// CEXOrderBookReadDiagnostic captures one exact-order-book read attempt.
type CEXOrderBookReadDiagnostic struct {
	Exchange              string
	Symbol                string
	ExchangeID            apptypes.CEXExchangeID
	MarketTypeID          apptypes.CEXMarketTypeID
	SymbolID              apptypes.CEXSymbolID
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
	exchange     cexOrderBookExchange
	symbol       cexOrderBookSymbol
	exchangeID   apptypes.CEXExchangeID
	marketTypeID apptypes.CEXMarketTypeID
	symbolID     apptypes.CEXSymbolID
	fetchedAt    int64
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

// ReadCEXOrderBook reads a specific order book snapshot by exchange, symbol,
// and fetchedAt timestamp. It is a deprecated boundary wrapper for older
// callers: it resolves labels through the JSON-backed ID registry, requires the
// exchange+symbol pair to have exactly one active market type, then delegates to
// the numeric ref reader.
func (c *CEXDataAccessSQL) ReadCEXOrderBook(
	ctx context.Context,
	exchange string,
	symbol string,
	fetchedAt int64,
) (*apptypes.CEXOrderBookSnapshot, error) {
	if c == nil || c.reader == nil {
		diag := CEXOrderBookReadDiagnostic{
			Exchange:             exchange,
			Symbol:               symbol,
			RequestedFetchedAtNs: fetchedAt,
			RequestedFetchedAtMs: fetchedAt / int64(time.Millisecond),
			Result:               cexOrderBookReadResultQueryError,
		}

		return nil, wrapCEXOrderBookReadError(ctx, diag, errCEXSQLiteReaderNotInit)
	}

	return c.reader.readCEXOrderBookByLabels(ctx, exchange, "", symbol, fetchedAt)
}

// ReadCEXOrderBookForMarket reads a specific order book snapshot by label
// boundary fields when the caller already knows the market type. It resolves
// labels through the JSON-backed ID registry and delegates to the numeric ref
// reader.
func (c *CEXDataAccessSQL) ReadCEXOrderBookForMarket(
	ctx context.Context,
	exchange string,
	marketType string,
	symbol string,
	fetchedAt int64,
) (*apptypes.CEXOrderBookSnapshot, error) {
	if c == nil || c.reader == nil {
		diag := CEXOrderBookReadDiagnostic{
			Exchange:             exchange,
			Symbol:               symbol,
			RequestedFetchedAtNs: fetchedAt,
			RequestedFetchedAtMs: fetchedAt / int64(time.Millisecond),
			Result:               cexOrderBookReadResultQueryError,
		}

		return nil, wrapCEXOrderBookReadError(ctx, diag, errCEXSQLiteReaderNotInit)
	}

	return c.reader.readCEXOrderBookByLabels(ctx, exchange, marketType, symbol, fetchedAt)
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

// ReadCEXMarketTradeBatch performs one exact primary-key lookup and returns a
// fully validated immutable trade payload. Any mismatch returns no trades.
func (c *CEXDataAccessSQL) ReadCEXMarketTradeBatch(
	ctx context.Context,
	ref apptypes.CEXMarketTradeBatchRef,
) ([]apptypes.CEXMarketTrade, error) {
	if c == nil || c.reader == nil {
		return nil, errCEXSQLiteReaderNotInit
	}

	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("%w: ref: %w", ErrCEXMarketTradeBatchInvalid, err)
	}

	row, err := c.reader.readCEXMarketTradeBatchRow(ctx, ref)
	if err != nil {
		return nil, err
	}

	metadataMatches := row.exchangeID == ref.ExchangeID &&
		row.marketTypeID == ref.MarketTypeID &&
		row.symbolID == ref.SymbolID &&
		row.firstSourceTimeMS == ref.FirstSourceTimeMS &&
		row.lastSourceTimeMS == ref.LastSourceTimeMS &&
		row.tradeCount == ref.TradeCount &&
		row.encodedBytes == ref.EncodedBytes &&
		row.digest == ref.PayloadSHA256
	if !metadataMatches {
		return nil, fmt.Errorf("%w: row metadata mismatch", ErrCEXMarketTradeBatchInvalid)
	}

	if uint64(len(row.payload)) != uint64(ref.EncodedBytes) {
		return nil, fmt.Errorf("%w: row byte length mismatch", ErrCEXMarketTradeBatchInvalid)
	}

	if sha256.Sum256(row.payload) != ref.PayloadSHA256 {
		return nil, fmt.Errorf("%w: row digest mismatch", ErrCEXMarketTradeBatchInvalid)
	}

	trades, err := DecodeCEXMarketTrades(row.payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCEXMarketTradeBatchInvalid, err)
	}

	if uint32(len(trades)) != ref.TradeCount || trades[0].SourceTimeMS != ref.FirstSourceTimeMS ||
		trades[len(trades)-1].SourceTimeMS != ref.LastSourceTimeMS {
		return nil, fmt.Errorf("%w: decoded range mismatch", ErrCEXMarketTradeBatchInvalid)
	}

	return trades, nil
}

// ReadCEXCandleBatch performs one exact primary-key lookup and returns a
// fully validated immutable candle payload. Any mismatch returns no bars.
func (c *CEXDataAccessSQL) ReadCEXCandleBatch(
	ctx context.Context,
	ref apptypes.CEXCandleBatchRef,
) ([]apptypes.CEXCandleBar, error) {
	if c == nil || c.reader == nil {
		return nil, errCEXSQLiteReaderNotInit
	}

	if err := ref.Validate(); err != nil {
		return nil, fmt.Errorf("%w: ref: %w", ErrCEXCandleBatchInvalid, err)
	}

	row, err := c.reader.readCEXCandleBatchRow(ctx, ref)
	if err != nil {
		return nil, err
	}

	metadataMatches := row.exchangeID == ref.ExchangeID &&
		row.marketTypeID == ref.MarketTypeID &&
		row.symbolID == ref.SymbolID &&
		row.timeframeMS == ref.TimeframeMS &&
		row.priceSource == ref.PriceSource &&
		row.policy == ref.Policy &&
		row.generationID == ref.GenerationID &&
		row.batchIndex == ref.BatchIndex &&
		row.batchCount == ref.BatchCount &&
		row.barCount == ref.BarCount &&
		row.firstBarStartMS == ref.FirstBarStartMS &&
		row.lastBarCloseMS == ref.LastBarCloseMS &&
		row.encodedBytes == ref.EncodedBytes &&
		row.digest == ref.PayloadSHA256
	if !metadataMatches {
		return nil, fmt.Errorf("%w: row metadata mismatch", ErrCEXCandleBatchInvalid)
	}

	if uint64(len(row.payload)) != uint64(ref.EncodedBytes) {
		return nil, fmt.Errorf("%w: row byte length mismatch", ErrCEXCandleBatchInvalid)
	}

	if sha256.Sum256(row.payload) != ref.PayloadSHA256 {
		return nil, fmt.Errorf("%w: row digest mismatch", ErrCEXCandleBatchInvalid)
	}

	bars, err := DecodeCEXCandleBars(row.payload, ref.TimeframeMS)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCEXCandleBatchInvalid, err)
	}

	if uint32(len(bars)) != ref.BarCount ||
		bars[0].BarStartMS != ref.FirstBarStartMS ||
		bars[len(bars)-1].BarCloseMS != ref.LastBarCloseMS {
		return nil, fmt.Errorf("%w: decoded range mismatch", ErrCEXCandleBatchInvalid)
	}

	return bars, nil
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
		ExchangeID:   row.key.exchangeID,
		MarketTypeID: row.key.marketTypeID,
		SymbolID:     row.key.symbolID,
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
		ExchangeID:           ref.ExchangeID,
		MarketTypeID:         ref.MarketTypeID,
		SymbolID:             ref.SymbolID,
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
			"read order book exchange_id=%d market_type_id=%d symbol_id=%d %s/%s@%d: %w",
			diag.ExchangeID,
			diag.MarketTypeID,
			diag.SymbolID,
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
