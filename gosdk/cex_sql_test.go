package gosdk

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestCEXDataAccessSQL_ReadCEXOrderBook_ClassifiesPrecisionMiss(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	requestedFetchedAt := int64(1_777_000_000_000_000_000)
	spxSymbolID := cexSymbolIDForTest(t, 1, 1, "SPXUSDT")
	err = insertCEXOrderBookV6(
		ctx,
		db,
		1,
		1,
		spxSymbolID,
		"mexc",
		"spot",
		"SPXUSDT",
		11,
		bids,
		asks,
		requestedFetchedAt+1,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	snapshots, errs := accessor.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{{
		ExchangeID:   1,
		MarketTypeID: 1,
		SymbolID:     apptypes.CEXSymbolID(spxSymbolID),
		FetchedAt:    requestedFetchedAt,
	}})
	require.Len(t, snapshots, 1)
	require.Len(t, errs, 1)
	require.Error(t, errs[0])
	require.ErrorIs(t, errs[0], ErrCEXOrderBookNotFound)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, errs[0], &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "precision_mismatch", readErr.Diagnostic.MissHint)
	require.Equal(t, requestedFetchedAt+1, readErr.Diagnostic.NearestNewerFetchedAt)
}

func TestStep159JCEXDataAccessSQLDoesNotWaitForV6SchemaReadiness(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.Error(t, err)
	require.Nil(t, accessor)
	require.Contains(t, err.Error(), "cex_orderbook_pairs_v3")
}

func TestCEXDataAccessSQL_ReadCEXOrderBook_ClassifiesTrueAbsence(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	spxSymbolID := cexSymbolIDForTest(t, 1, 1, "SPXUSDT")
	snapshots, errs := accessor.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{{
		ExchangeID:   1,
		MarketTypeID: 1,
		SymbolID:     apptypes.CEXSymbolID(spxSymbolID),
		FetchedAt:    time.Now().UnixNano(),
	}})
	require.Len(t, snapshots, 1)
	require.Len(t, errs, 1)
	require.Error(t, errs[0])
	require.ErrorIs(t, errs[0], ErrCEXOrderBookNotFound)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, errs[0], &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "true_absence", readErr.Diagnostic.MissHint)
}

func TestCEXDataAccessSQL_ReadCEXOrderBooks_ReadsPreparedPoints(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	spxSymbolID := cexSymbolIDForTest(t, 1, 1, "SPXUSDT")
	ethSymbolID := cexSymbolIDForTest(t, 1, 1, "ETHUSDT")
	err = insertCEXOrderBookV6(
		ctx,
		db,
		1,
		1,
		spxSymbolID,
		"mexc",
		"spot",
		"SPXUSDT",
		11,
		bids,
		asks,
		int64(100),
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	snapshots, errs := accessor.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{
		{
			ExchangeID:   1,
			MarketTypeID: 1,
			SymbolID:     apptypes.CEXSymbolID(spxSymbolID),
			FetchedAt:    100,
		},
		{
			ExchangeID:   1,
			MarketTypeID: 1,
			SymbolID:     apptypes.CEXSymbolID(ethSymbolID),
			FetchedAt:    200,
		},
	})
	require.Len(t, snapshots, 2)
	require.Len(t, errs, 2)
	require.NoError(t, errs[0])
	require.NotNil(t, snapshots[0])
	require.Equal(t, int64(11), snapshots[0].LastUpdateID)
	require.Error(t, errs[1])
	require.ErrorIs(t, errs[1], ErrCEXOrderBookNotFound)
	require.Nil(t, snapshots[1])
	require.Equal(t, []apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}}, snapshots[0].Bids)
	require.Equal(t, []apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}}, snapshots[0].Asks)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, errs[1], &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "true_absence", readErr.Diagnostic.MissHint)
}

func TestStep159VExactNumericReadRejectsUnknownRegistryIdentity(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	validSymbolID := cexSymbolIDForTest(t, 1, 1, "SPXUSDT")
	require.NoError(t, insertCEXOrderBookV6(
		ctx,
		db,
		1,
		1,
		validSymbolID,
		"mexc",
		"spot",
		"SPXUSDT",
		11,
		bids,
		asks,
		100,
	))
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	snapshots, readErrs := accessor.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{
		{
			ExchangeID: 1, MarketTypeID: 1,
			SymbolID: apptypes.CEXSymbolID(validSymbolID), FetchedAt: 100,
		},
		{ExchangeID: 65_535, MarketTypeID: 1, SymbolID: 1, FetchedAt: 100},
	})
	require.Len(t, snapshots, 2)
	require.Len(t, readErrs, 2)
	require.NoError(t, readErrs[0])
	require.NotNil(t, snapshots[0])
	require.Nil(t, snapshots[1])
	require.Error(t, readErrs[1])
	require.NotErrorIs(t, readErrs[1], ErrCEXOrderBookNotFound)
	require.ErrorIs(t, readErrs[1], errCEXOrderBookUnknownIdentity)

	var exactReadErr *CEXOrderBookReadError
	require.ErrorAs(t, readErrs[1], &exactReadErr)
	require.Equal(t, cexOrderBookReadResultQueryError, exactReadErr.Diagnostic.Result)
	require.Empty(t, exactReadErr.Diagnostic.MissHint)
	require.Zero(t, exactReadErr.Diagnostic.NearestProbeDuration)
}

func TestStep159JReadCEXOrderBooksUsesNumericIdentityAndRejectsLegacyRefs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	btcSpotSymbolID := cexSymbolIDForTest(t, 2, 1, "BTCUSDC")
	btcPerpsSymbolID := cexSymbolIDForTest(t, 2, 2, "BTCUSDC")
	err = insertCEXOrderBookV6(
		ctx,
		db,
		2,
		1,
		btcSpotSymbolID,
		"hyperliquid",
		"spot",
		"BTCUSDC",
		11,
		bids,
		asks,
		100,
	)
	require.NoError(t, err)
	err = insertCEXOrderBookV6(
		ctx,
		db,
		2,
		2,
		btcPerpsSymbolID,
		"hyperliquid",
		"perps",
		"BTCUSDC",
		22,
		bids,
		asks,
		100,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	snapshots, errs := accessor.ReadCEXOrderBooks(ctx, []apptypes.CEXOrderBookRef{
		{
			ExchangeID:   2,
			MarketTypeID: 1,
			SymbolID:     apptypes.CEXSymbolID(btcSpotSymbolID),
			FetchedAt:    100,
		},
		{
			ExchangeID:   2,
			MarketTypeID: 2,
			SymbolID:     apptypes.CEXSymbolID(btcPerpsSymbolID),
			FetchedAt:    100,
		},
		{Exchange: "hyperliquid", Symbol: "BTCUSDC", FetchedAt: 100},
	})
	require.Len(t, snapshots, 3)
	require.Len(t, errs, 3)
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, int64(11), snapshots[0].LastUpdateID)
	require.Equal(t, int64(22), snapshots[1].LastUpdateID)
	require.Equal(t, apptypes.CEXExchangeID(2), snapshots[0].ExchangeID)
	require.Equal(t, apptypes.CEXMarketTypeID(1), snapshots[0].MarketTypeID)
	require.Equal(t, apptypes.CEXSymbolID(btcSpotSymbolID), snapshots[0].SymbolID)
	require.Equal(t, apptypes.CEXExchangeID(2), snapshots[1].ExchangeID)
	require.Equal(t, apptypes.CEXMarketTypeID(2), snapshots[1].MarketTypeID)
	require.Equal(t, apptypes.CEXSymbolID(btcPerpsSymbolID), snapshots[1].SymbolID)
	require.Error(t, errs[2])
	require.ErrorIs(t, errs[2], ErrCEXLegacyOrderBookRef)
	require.Nil(t, snapshots[2])
}

func TestStep159JDeprecatedReadCEXOrderBookResolvesUnambiguousLabels(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	spxSymbolID := cexSymbolIDForTest(t, 1, 1, "SPXUSDT")
	err = insertCEXOrderBookV6(
		ctx,
		db,
		1,
		1,
		spxSymbolID,
		"mexc",
		"spot",
		"SPXUSDT",
		33,
		bids,
		asks,
		100,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	snapshot, err := accessor.ReadCEXOrderBook(ctx, "mexc", "SPXUSDT", 100)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, int64(33), snapshot.LastUpdateID)
	require.Equal(t, apptypes.CEXExchangeID(1), snapshot.ExchangeID)
	require.Equal(t, apptypes.CEXMarketTypeID(1), snapshot.MarketTypeID)
	require.Equal(t, apptypes.CEXSymbolID(spxSymbolID), snapshot.SymbolID)
}

func TestStep159JDeprecatedReadCEXOrderBookRejectsAmbiguousMarketLabels(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	btcSpotSymbolID := cexSymbolIDForTest(t, 2, 1, "BTCUSDC")
	btcPerpsSymbolID := cexSymbolIDForTest(t, 2, 2, "BTCUSDC")
	err = insertCEXOrderBookV6(
		ctx,
		db,
		2,
		1,
		btcSpotSymbolID,
		"hyperliquid",
		"spot",
		"BTCUSDC",
		33,
		bids,
		asks,
		100,
	)
	require.NoError(t, err)
	err = insertCEXOrderBookV6(
		ctx,
		db,
		2,
		2,
		btcPerpsSymbolID,
		"hyperliquid",
		"perps",
		"BTCUSDC",
		44,
		bids,
		asks,
		100,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	snapshot, err := accessor.ReadCEXOrderBook(ctx, "hyperliquid", "BTCUSDC", 100)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCEXOrderBookAmbiguousMarket)
	require.Nil(t, snapshot)
}

// The deprecated label reader resolves to numeric ids internally, but a miss
// diagnostic must still carry the exchange and symbol labels the caller passed,
// or operators reading the error see blank names.
func TestDeprecatedReadCEXOrderBookMissKeepsLabels(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	spxSymbolID := cexSymbolIDForTest(t, 1, 1, "SPXUSDT")
	// A row at 200 makes the pair resolvable; the read at 100 then misses on the
	// exact timestamp, exercising the resolve-succeeds-then-miss path.
	err = insertCEXOrderBookV6(
		ctx,
		db,
		1,
		1,
		spxSymbolID,
		"mexc",
		"spot",
		"SPXUSDT",
		11,
		bids,
		asks,
		200,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	_, err = accessor.ReadCEXOrderBook(ctx, "mexc", "SPXUSDT", 100)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCEXOrderBookNotFound)

	var readErr *CEXOrderBookReadError
	require.ErrorAs(t, err, &readErr)
	require.Equal(t, "no_row", readErr.Diagnostic.Result)
	require.Equal(t, "mexc", readErr.Diagnostic.Exchange)
	require.Equal(t, "SPXUSDT", readErr.Diagnostic.Symbol)
}

func TestStep159JReadCEXOrderBookForMarketReadsRequestedMarket(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	dbPath := filepath.Join(t.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(t, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(t, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(t, err)

	btcSpotSymbolID := cexSymbolIDForTest(t, 2, 1, "BTCUSDC")
	btcPerpsSymbolID := cexSymbolIDForTest(t, 2, 2, "BTCUSDC")
	err = insertCEXOrderBookV6(
		ctx,
		db,
		2,
		1,
		btcSpotSymbolID,
		"hyperliquid",
		"spot",
		"BTCUSDC",
		33,
		bids,
		asks,
		100,
	)
	require.NoError(t, err)
	err = insertCEXOrderBookV6(
		ctx,
		db,
		2,
		2,
		btcPerpsSymbolID,
		"hyperliquid",
		"perps",
		"BTCUSDC",
		44,
		bids,
		asks,
		100,
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(t, err)

	defer accessor.Close()

	spotSnapshot, err := accessor.ReadCEXOrderBookForMarket(
		ctx,
		"hyperliquid",
		"spot",
		"BTCUSDC",
		100,
	)
	require.NoError(t, err)
	require.NotNil(t, spotSnapshot)
	require.Equal(t, int64(33), spotSnapshot.LastUpdateID)
	require.Equal(t, apptypes.CEXMarketTypeID(1), spotSnapshot.MarketTypeID)
	require.Equal(t, apptypes.CEXSymbolID(btcSpotSymbolID), spotSnapshot.SymbolID)

	perpsSnapshot, err := accessor.ReadCEXOrderBookForMarket(
		ctx,
		"hyperliquid",
		"perps",
		"BTCUSDC",
		100,
	)
	require.NoError(t, err)
	require.NotNil(t, perpsSnapshot)
	require.Equal(t, int64(44), perpsSnapshot.LastUpdateID)
	require.Equal(t, apptypes.CEXMarketTypeID(2), perpsSnapshot.MarketTypeID)
	require.Equal(t, apptypes.CEXSymbolID(btcPerpsSymbolID), perpsSnapshot.SymbolID)
}

func TestStep159JDeprecatedCEXOrderBookWrapperStaysBoundaryOnly(t *testing.T) {
	t.Parallel()

	readerSource, err := os.ReadFile("cex_sql_reader_zombie.go")
	require.NoError(t, err)

	readerText := string(readerSource)
	require.Contains(t, readerText, "cexOrderBookLegacyMarketTypeQuery")
	require.Contains(t, readerText, "WHERE exchange_id = ? AND symbol_id = ?")
	require.Contains(t, readerText, "ORDER BY market_type_id")
	require.Contains(t, readerText, "LIMIT 2")
	require.NotContains(t, readerText, "cex_orderbooks_v5")
	require.NotContains(t, readerText, "cex_orderbook_dirty_pair_refs_v1")
	require.NotContains(t, readerText, "cex_pairs_v2")
	require.NotContains(t, readerText, "cexOrderBookSchemaRetryInterval")
	require.NotContains(t, readerText, "time.NewTimer")
	require.NotContains(t, readerText, "wait for cex order-book v6 schema")
	require.NotContains(t, readerText, "cex_exchange_dim")
	require.NotContains(t, readerText, "cex_market_type_dim")
	require.NotContains(t, readerText, "cex_symbol_dim")

	labelResolverBody := extractFunctionBodyForTest(
		t,
		readerText,
		"func (r *cexOrderBookFastReader) resolveCEXOrderBookRefByLabels(",
	)
	require.Contains(t, labelResolverBody, "ResolveLegacySymbolID")
	require.Contains(t, labelResolverBody, "resolveUniqueCEXOrderBookMarket(r.legacy")
	require.NotContains(t, labelResolverBody, "SymbolCandidates(")

	legacyResolverBody := extractFunctionBodyForTest(
		t,
		readerText,
		"func resolveUniqueCEXOrderBookMarket(",
	)
	require.Contains(t, legacyResolverBody, "stmt.BindInt64(1, int64(exchangeID))")
	require.Contains(t, legacyResolverBody, "stmt.BindInt64(2, int64(symbolID))")
	require.Contains(t, legacyResolverBody, "ErrCEXOrderBookAmbiguousMarket")
	require.NotContains(t, legacyResolverBody, "for _, candidate")
	require.NotContains(t, legacyResolverBody, "readCEXOrderBookPairID(")

	apiSource, err := os.ReadFile("cex_sql.go")
	require.NoError(t, err)

	apiText := string(apiSource)
	wrapperBody := extractFunctionBodyForTest(
		t,
		apiText,
		"func (c *CEXDataAccessSQL) ReadCEXOrderBook(",
	)
	require.NotContains(t, wrapperBody, "CEXOrderBookRef{")
	require.NotContains(t, wrapperBody, "ReadCEXOrderBooks(")
	require.Contains(t, wrapperBody, "readCEXOrderBookByLabels")

	eventSource, err := os.ReadFile("state_transition.go")
	require.NoError(t, err)
	require.NotContains(t, string(eventSource), ".ReadCEXOrderBook(")
}

func BenchmarkCEXDataAccessSQL_ReadCEXOrderBooks_PreparedPoints(b *testing.B) {
	ctx := b.Context()
	dbPath := filepath.Join(b.TempDir(), "cex.sqlite")
	db, err := openCEXTestDB(ctx, dbPath)
	require.NoError(b, err)

	bids, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "1", Quantity: "2"}})
	require.NoError(b, err)
	asks, err := cbor.Marshal([]apptypes.CEXPriceLevel{{Price: "3", Quantity: "4"}})
	require.NoError(b, err)

	spxSymbolID := cexSymbolIDForTest(b, 1, 1, "SPXUSDT")
	for i := range 100 {
		err = insertCEXOrderBookV6(
			ctx,
			db,
			1,
			1,
			spxSymbolID,
			"mexc",
			"spot",
			"SPXUSDT",
			int64(i),
			bids,
			asks,
			int64(1_000+i),
		)
		require.NoError(b, err)
	}

	require.NoError(b, db.Close())

	accessor, err := NewCEXDataAccessSQL(ctx, dbPath)
	require.NoError(b, err)

	defer accessor.Close()

	for _, refCount := range []int{1, 10, 100} {
		refs := make([]apptypes.CEXOrderBookRef, refCount)
		for i := range refs {
			refs[i] = apptypes.CEXOrderBookRef{
				ExchangeID:   1,
				MarketTypeID: 1,
				SymbolID:     apptypes.CEXSymbolID(spxSymbolID),
				FetchedAt:    int64(1_000 + i),
			}
		}

		b.Run("refs_"+strconv.Itoa(refCount), func(b *testing.B) {
			for range b.N {
				snapshots, errs := accessor.ReadCEXOrderBooks(ctx, refs)
				require.Len(b, snapshots, len(refs))

				for _, err := range errs {
					require.NoError(b, err)
				}
			}
		})
	}
}

func openCEXTestDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	db, err := openSQLite(ctx, dbPath, "rwc")
	if err != nil {
		return nil, err
	}

	if _, err = db.ExecContext(ctx, createCEXOrderBooksV6SQL); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}

		return nil, err
	}

	return db, nil
}

func insertCEXOrderBookV6(
	ctx context.Context,
	db *sql.DB,
	exchangeID uint16,
	marketTypeID uint8,
	symbolID uint32,
	exchange string,
	marketType string,
	symbol string,
	lastUpdateID int64,
	bids []byte,
	asks []byte,
	fetchedAt int64,
) error {
	_ = exchange
	_ = marketType
	_ = symbol

	if _, err := db.ExecContext(ctx, `
		INSERT INTO cex_orderbook_pairs_v3(exchange_id, market_type_id, symbol_id)
		VALUES(?, ?, ?)
		ON CONFLICT(exchange_id, market_type_id, symbol_id) DO NOTHING
	`, exchangeID, marketTypeID, symbolID); err != nil {
		return fmt.Errorf("insert pair: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO cex_orderbooks_v6(pair_id, last_update_id, bids, asks, fetched_at)
		SELECT id, ?, ?, ?, ?
		FROM cex_orderbook_pairs_v3
		WHERE exchange_id = ? AND market_type_id = ? AND symbol_id = ?
	`, lastUpdateID, bids, asks, fetchedAt, exchangeID, marketTypeID, symbolID); err != nil {
		return fmt.Errorf("insert orderbook: %w", err)
	}

	return nil
}

func cexSymbolIDForTest(
	tb testing.TB,
	exchangeID uint16,
	marketTypeID uint8,
	symbol string,
) uint32 {
	tb.Helper()

	symbolID, err := apptypes.DefaultOrderBookIDRegistry.ResolveSymbolID(
		apptypes.CEXExchangeID(exchangeID),
		apptypes.CEXMarketTypeID(marketTypeID),
		symbol,
	)
	require.NoError(tb, err)

	return uint32(symbolID)
}

func openSQLite(ctx context.Context, dbPath, mode string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=%s&cache=shared&uri=true", dbPath, mode)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}

		return nil, err
	}

	return db, nil
}

const createCEXOrderBooksV6SQL = `
CREATE TABLE cex_orderbook_pairs_v3 (
	id INTEGER PRIMARY KEY,
	exchange_id INTEGER NOT NULL,
	market_type_id INTEGER NOT NULL,
	symbol_id INTEGER NOT NULL,
	UNIQUE(exchange_id, market_type_id, symbol_id)
);

CREATE TABLE cex_orderbooks_v6 (
	id INTEGER PRIMARY KEY,
	pair_id INTEGER NOT NULL,
	last_update_id INTEGER NOT NULL,
	bids BLOB NOT NULL,
	asks BLOB NOT NULL,
	fetched_at INTEGER NOT NULL,
	consumed INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(pair_id) REFERENCES cex_orderbook_pairs_v3(id)
);

CREATE INDEX idx_cex_ob_v6_pair_fetched_id ON cex_orderbooks_v6(pair_id, fetched_at, id);
CREATE INDEX idx_cex_pairs_v3_exchange_symbol_market ON cex_orderbook_pairs_v3(exchange_id, symbol_id, market_type_id);
`

func extractFunctionBodyForTest(t *testing.T, source string, signature string) string {
	t.Helper()

	start := strings.Index(source, signature)
	require.NotEqual(t, -1, start)

	open := strings.Index(source[start:], "{")
	require.NotEqual(t, -1, open)
	bodyStart := start + open
	depth := 0

	for i := bodyStart; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[bodyStart : i+1]
			}
		default:
			continue
		}
	}

	t.Fatalf("function body not closed for %s", signature)

	return ""
}
