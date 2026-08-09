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
	maxCEXMarketTradeBatchSQLiteInt uint64 = 1<<63 - 1
	maxCEXMarketTradeBatchExchange         = int64(^uint16(0))
	maxCEXMarketTradeBatchMarket           = int64(^uint8(0))
	maxCEXMarketTradeBatchSymbol           = int64(^uint32(0))
	maxCEXMarketTradeBatchCount            = int64(^uint32(0))

	cexOrderBookPairIDQuery = `
SELECT id
FROM cex_orderbook_pairs_v3
WHERE exchange_id = ? AND market_type_id = ? AND symbol_id = ?
LIMIT 1`
	cexOrderBookLegacyMarketTypeQuery = `
SELECT market_type_id
FROM cex_orderbook_pairs_v3
WHERE exchange_id = ? AND symbol_id = ?
ORDER BY market_type_id
LIMIT 2`
	cexOrderBookExactReadQuery = `
SELECT ob.last_update_id, ob.bids, ob.asks
FROM cex_orderbooks_v6 AS ob
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
	cexMarketTradeBatchExactReadQuery = `
SELECT exchange_id, market_type_id, symbol_id, first_source_time_ms,
       last_source_time_ms, trade_count, encoded_bytes, payload_sha256, payload
FROM cex_market_trade_batches_v1
WHERE id = ?
LIMIT 1`
)

type cexOrderBookFastReader struct {
	mu sync.Mutex

	conn       *zsqlite.Conn
	pair       *zsqlite.Stmt
	legacy     *zsqlite.Stmt
	exact      *zsqlite.Stmt
	older      *zsqlite.Stmt
	newer      *zsqlite.Stmt
	trade      *zsqlite.Stmt
	idRegistry *apptypes.OrderBookIDRegistry

	bidsBuf         []byte
	asksBuf         []byte
	tradeDigestBuf  []byte
	tradePayloadBuf []byte
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

	reader, err := prepareCEXOrderBookFastReader(ctx, conn)
	if err == nil {
		return reader, nil
	}

	if closeErr := conn.Close(); closeErr != nil {
		return nil, fmt.Errorf("%w; close sqlite after prepare failure: %w", err, closeErr)
	}

	return nil, err
}

func prepareCEXOrderBookFastReader(
	ctx context.Context,
	conn *zsqlite.Conn,
) (*cexOrderBookFastReader, error) {
	registry, err := apptypes.NewOrderBookIDRegistryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("load cex order-book identity registry: %w", err)
	}

	reader := &cexOrderBookFastReader{
		conn:       conn,
		idRegistry: registry,
	}

	if reader.pair, err = conn.Prepare(cexOrderBookPairIDQuery); err != nil {
		reader.finalizePrepared()

		return nil, fmt.Errorf("prepare cex order-book pair lookup: %w", err)
	}

	if reader.legacy, err = conn.Prepare(cexOrderBookLegacyMarketTypeQuery); err != nil {
		reader.finalizePrepared()

		return nil, fmt.Errorf("prepare cex order-book legacy market lookup: %w", err)
	}

	if reader.exact, err = conn.Prepare(cexOrderBookExactReadQuery); err != nil {
		reader.finalizePrepared()

		return nil, fmt.Errorf("prepare cex order-book exact read: %w", err)
	}

	if reader.older, err = conn.Prepare(cexOrderBookNearestOlderQuery); err != nil {
		reader.finalizePrepared()

		return nil, fmt.Errorf("prepare cex order-book older probe: %w", err)
	}

	if reader.newer, err = conn.Prepare(cexOrderBookNearestNewerQuery); err != nil {
		reader.finalizePrepared()

		return nil, fmt.Errorf("prepare cex order-book newer probe: %w", err)
	}

	return reader, nil
}

func (r *cexOrderBookFastReader) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}

	r.finalizePrepared()

	return r.conn.Close()
}

func (r *cexOrderBookFastReader) finalizePrepared() {
	if r == nil {
		return
	}

	for _, stmt := range []*zsqlite.Stmt{
		r.pair,
		r.legacy,
		r.exact,
		r.older,
		r.newer,
		r.trade,
	} {
		if stmt != nil {
			_ = stmt.Finalize()
		}
	}

	r.pair = nil
	r.legacy = nil
	r.exact = nil
	r.older = nil
	r.newer = nil
	r.trade = nil
}

type cexMarketTradeBatchRow struct {
	exchangeID        apptypes.CEXExchangeID
	marketTypeID      apptypes.CEXMarketTypeID
	symbolID          apptypes.CEXSymbolID
	firstSourceTimeMS uint64
	lastSourceTimeMS  uint64
	tradeCount        uint32
	encodedBytes      uint32
	digest            [32]byte
	payload           []byte
}

func (r *cexOrderBookFastReader) readCEXMarketTradeBatchRow(
	ctx context.Context,
	ref apptypes.CEXMarketTradeBatchRef,
) (*cexMarketTradeBatchRow, error) {
	// SQLite stores signed integers. A reference outside that domain cannot
	// identify a persisted row, and must not be narrowed before comparison.
	if ref.BatchID > maxCEXMarketTradeBatchSQLiteInt ||
		ref.FirstSourceTimeMS > maxCEXMarketTradeBatchSQLiteInt ||
		ref.LastSourceTimeMS > maxCEXMarketTradeBatchSQLiteInt {
		return nil, fmt.Errorf(
			"%w: ref exceeds SQLite integer range",
			ErrCEXMarketTradeBatchInvalid,
		)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.trade == nil {
		stmt, err := r.conn.Prepare(cexMarketTradeBatchExactReadQuery)
		if err != nil {
			return nil, fmt.Errorf("prepare cex market trade batch read: %w", err)
		}

		r.trade = stmt
	}

	oldDone := r.conn.SetInterrupt(ctx.Done())
	defer r.conn.SetInterrupt(oldDone)

	r.trade.BindInt64(1, int64(ref.BatchID))

	hasRow, err := r.trade.Step()
	if err != nil {
		_ = resetCEXOrderBookStmt(r.trade)

		return nil, err
	}

	if !hasRow {
		if err := resetCEXOrderBookStmt(r.trade); err != nil {
			return nil, err
		}

		return nil, ErrCEXMarketTradeBatchNotFound
	}

	digestRaw := readSQLiteColumnBytes(r.trade, 7, &r.tradeDigestBuf)
	if len(digestRaw) != 32 {
		_ = resetCEXOrderBookStmt(r.trade)

		return nil, fmt.Errorf("%w: stored digest length", ErrCEXMarketTradeBatchInvalid)
	}

	var digest [32]byte
	copy(digest[:], digestRaw)

	exchangeID := r.trade.ColumnInt64(0)
	marketTypeID := r.trade.ColumnInt64(1)
	symbolID := r.trade.ColumnInt64(2)
	firstSourceTimeMS := r.trade.ColumnInt64(3)
	lastSourceTimeMS := r.trade.ColumnInt64(4)
	tradeCount := r.trade.ColumnInt64(5)

	encodedBytes := r.trade.ColumnInt64(6)
	if exchangeID <= 0 || exchangeID > maxCEXMarketTradeBatchExchange ||
		marketTypeID <= 0 || marketTypeID > maxCEXMarketTradeBatchMarket ||
		symbolID <= 0 || symbolID > maxCEXMarketTradeBatchSymbol ||
		firstSourceTimeMS <= 0 || lastSourceTimeMS <= 0 ||
		tradeCount <= 0 || tradeCount > maxCEXMarketTradeBatchCount ||
		encodedBytes <= 0 || encodedBytes > maxCEXMarketTradeBatchCount {
		_ = resetCEXOrderBookStmt(r.trade)

		return nil, fmt.Errorf("%w: stored metadata range", ErrCEXMarketTradeBatchInvalid)
	}

	row := &cexMarketTradeBatchRow{
		exchangeID:        apptypes.CEXExchangeID(exchangeID),
		marketTypeID:      apptypes.CEXMarketTypeID(marketTypeID),
		symbolID:          apptypes.CEXSymbolID(symbolID),
		firstSourceTimeMS: uint64(firstSourceTimeMS),
		lastSourceTimeMS:  uint64(lastSourceTimeMS),
		tradeCount:        uint32(tradeCount),
		encodedBytes:      uint32(encodedBytes),
		digest:            digest,
		payload: append(
			[]byte(nil),
			readSQLiteColumnBytes(r.trade, 8, &r.tradePayloadBuf)...),
	}
	if err := resetCEXOrderBookStmt(r.trade); err != nil {
		return nil, err
	}

	return row, nil
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

	// resolveCEXOrderBookRefByLabels returns numeric ids only; carry the caller's
	// labels so a miss or query error reports names, not blank fields.
	ref.Exchange = exchange
	ref.Symbol = symbol

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
	exchangeID, err := r.idRegistry.ResolveExchangeID(exchange)
	if err != nil {
		return apptypes.CEXOrderBookRef{}, err
	}

	if marketType != "" {
		marketTypeID, marketTypeErr := r.idRegistry.ResolveMarketTypeID(marketType)
		if marketTypeErr != nil {
			return apptypes.CEXOrderBookRef{}, marketTypeErr
		}

		symbolID, symbolErr := r.idRegistry.ResolveSymbolID(exchangeID, marketTypeID, symbol)
		if symbolErr != nil {
			return apptypes.CEXOrderBookRef{}, symbolErr
		}

		return apptypes.CEXOrderBookRef{
			FetchedAt:    fetchedAt,
			ExchangeID:   exchangeID,
			MarketTypeID: marketTypeID,
			SymbolID:     symbolID,
		}, nil
	}

	symbolID, symbolErr := r.idRegistry.ResolveLegacySymbolID(exchangeID, symbol)
	if symbolErr != nil {
		return apptypes.CEXOrderBookRef{}, symbolErr
	}

	ref, ok, err := resolveUniqueCEXOrderBookMarket(r.legacy, exchangeID, symbolID, fetchedAt)
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

	if err := r.validateCEXOrderBookRefIdentity(ref); err != nil {
		return nil, time.Since(queryStart), err
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

	exchange := ref.Exchange
	if exchange == "" {
		if label, ok := r.idRegistry.ExchangeLabel(ref.ExchangeID); ok {
			exchange = label
		}
	}

	symbol := ref.Symbol
	if symbol == "" {
		if label, ok := r.idRegistry.SymbolLabel(
			ref.ExchangeID,
			ref.MarketTypeID,
			ref.SymbolID,
		); ok {
			symbol = label
		}
	}

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

func (r *cexOrderBookFastReader) validateCEXOrderBookRefIdentity(
	ref apptypes.CEXOrderBookRef,
) error {
	if ref.ExchangeID == 0 || ref.MarketTypeID == 0 || ref.SymbolID == 0 {
		return ErrCEXLegacyOrderBookRef
	}

	if _, ok := r.idRegistry.ExchangeLabel(ref.ExchangeID); !ok {
		return fmt.Errorf("%w: exchange_id=%d", errCEXOrderBookUnknownIdentity, ref.ExchangeID)
	}

	if _, ok := r.idRegistry.MarketTypeLabel(ref.MarketTypeID); !ok {
		return fmt.Errorf("%w: market_type_id=%d", errCEXOrderBookUnknownIdentity, ref.MarketTypeID)
	}

	if _, ok := r.idRegistry.SymbolLabel(ref.ExchangeID, ref.MarketTypeID, ref.SymbolID); !ok {
		return fmt.Errorf(
			"%w: exchange_id=%d market_type_id=%d symbol_id=%d",
			errCEXOrderBookUnknownIdentity,
			ref.ExchangeID,
			ref.MarketTypeID,
			ref.SymbolID,
		)
	}

	return nil
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

func resolveUniqueCEXOrderBookMarket(
	stmt *zsqlite.Stmt,
	exchangeID apptypes.CEXExchangeID,
	symbolID apptypes.CEXSymbolID,
	fetchedAt int64,
) (apptypes.CEXOrderBookRef, bool, error) {
	stmt.BindInt64(1, int64(exchangeID))
	stmt.BindInt64(2, int64(symbolID))

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

	marketTypeID := apptypes.CEXMarketTypeID(stmt.ColumnInt64(0))

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

	return apptypes.CEXOrderBookRef{
		FetchedAt:    fetchedAt,
		ExchangeID:   exchangeID,
		MarketTypeID: marketTypeID,
		SymbolID:     symbolID,
	}, true, nil
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
