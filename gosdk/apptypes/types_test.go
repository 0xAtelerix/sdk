package apptypes

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

type cexMarketTradeBatchRefValues struct {
	exchangeID        CEXExchangeID
	marketTypeID      CEXMarketTypeID
	symbolID          CEXSymbolID
	batchID           uint64
	firstSourceTimeMS uint64
	lastSourceTimeMS  uint64
	tradeCount        uint32
	encodedBytes      uint32
	payloadSHA256     [32]byte
}

func TestCEXMarketTradeBatchRefSchemaCases(t *testing.T) {
	t.Parallel()

	refType := cexMarketTradeBatchRefType(t)
	require.Equal(t, "CEXMarketTradeBatchRef", refType.Name())

	for _, want := range []struct {
		name string
		typ  reflect.Type
		json string
		cbor string
	}{
		{"ExchangeID", reflect.TypeFor[CEXExchangeID](), "exchangeId", "1,keyasint"},
		{"MarketTypeID", reflect.TypeFor[CEXMarketTypeID](), "marketTypeId", "2,keyasint"},
		{"SymbolID", reflect.TypeFor[CEXSymbolID](), "symbolId", "3,keyasint"},
		{"BatchID", reflect.TypeFor[uint64](), "batchId", "4,keyasint"},
		{"FirstSourceTimeMS", reflect.TypeFor[uint64](), "firstSourceTimeMs", "5,keyasint"},
		{"LastSourceTimeMS", reflect.TypeFor[uint64](), "lastSourceTimeMs", "6,keyasint"},
		{"TradeCount", reflect.TypeFor[uint32](), "tradeCount", "7,keyasint"},
		{"EncodedBytes", reflect.TypeFor[uint32](), "encodedBytes", "8,keyasint"},
		{"PayloadSHA256", reflect.TypeFor[[32]byte](), "payloadSha256", "9,keyasint"},
	} {
		t.Run(want.name, func(t *testing.T) {
			t.Parallel()

			field, ok := refType.FieldByName(want.name)
			require.True(t, ok)
			require.Equal(t, want.typ, field.Type)
			require.Equal(t, want.json, field.Tag.Get("json"))
			require.Equal(t, want.cbor, field.Tag.Get("cbor"))
		})
	}

	require.Equal(t, 9, refType.NumField())

	for fieldIndex := range refType.NumField() {
		field := refType.Field(fieldIndex)
		require.Falsef(
			t,
			field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Uint8,
			"%s must remain a compact immutable reference, not payload bytes",
			field.Name,
		)
	}

	batchType := reflect.TypeFor[Batch[testAllMidsTx, testAllMidsReceipt]]()
	batchRefs := mustStructField(t, batchType, "CEXMarketTradeBatchRefs")
	require.Equal(t, "9,keyasint", batchRefs.Tag.Get("cbor"))
	require.Equal(t, reflect.Slice, batchRefs.Type.Kind())
	require.Equal(t, refType, batchRefs.Type.Elem())

	eventType := reflect.TypeFor[Event]()
	eventRefs := mustStructField(t, eventType, "CEXMarketTradeBatchRefs")
	require.Equal(t, "cexMarketTradeBatchRefs", eventRefs.Tag.Get("json"))
	require.Equal(t, "10,keyasint", eventRefs.Tag.Get("cbor"))
	require.Equal(t, reflect.Slice, eventRefs.Type.Kind())
	require.Equal(t, refType, eventRefs.Type.Elem())

	require.Equal(
		t,
		"8,keyasint",
		mustStructField(t, batchType, "HyperliquidAllMidsRefs").Tag.Get("cbor"),
	)
	require.Equal(
		t,
		"9,keyasint",
		mustStructField(t, eventType, "HyperliquidAllMidsRefs").Tag.Get("cbor"),
	)
}

func TestCEXMarketTradeBatchRefValidationCases(t *testing.T) {
	t.Parallel()

	valid := validCEXMarketTradeBatchRefValues(t)
	require.NoError(t, validateCEXMarketTradeBatchRef(t, newCEXMarketTradeBatchRef(t, valid)))

	for _, tc := range []struct {
		name   string
		mutate func(*cexMarketTradeBatchRefValues)
	}{
		{"zero-exchange", func(ref *cexMarketTradeBatchRefValues) { ref.exchangeID = 0 }},
		{"unknown-exchange", func(ref *cexMarketTradeBatchRefValues) { ref.exchangeID = CEXExchangeID(99) }},
		{"zero-market-type", func(ref *cexMarketTradeBatchRefValues) { ref.marketTypeID = 0 }},
		{"unknown-market-type", func(ref *cexMarketTradeBatchRefValues) {
			ref.marketTypeID = CEXMarketTypeID(99)
		}},
		{"zero-symbol", func(ref *cexMarketTradeBatchRefValues) { ref.symbolID = 0 }},
		{"unknown-symbol", func(ref *cexMarketTradeBatchRefValues) {
			ref.symbolID = CEXSymbolID(999_999)
		}},
		{"zero-batch-id", func(ref *cexMarketTradeBatchRefValues) { ref.batchID = 0 }},
		{"zero-first-source-time", func(ref *cexMarketTradeBatchRefValues) {
			ref.firstSourceTimeMS = 0
		}},
		{"zero-last-source-time", func(ref *cexMarketTradeBatchRefValues) {
			ref.lastSourceTimeMS = 0
		}},
		{"reversed-source-time-range", func(ref *cexMarketTradeBatchRefValues) {
			ref.firstSourceTimeMS = ref.lastSourceTimeMS + 1
		}},
		{"zero-trade-count", func(ref *cexMarketTradeBatchRefValues) { ref.tradeCount = 0 }},
		{"zero-encoded-bytes", func(ref *cexMarketTradeBatchRefValues) {
			ref.encodedBytes = 0
		}},
		{"zero-payload-sha256", func(ref *cexMarketTradeBatchRefValues) {
			ref.payloadSHA256 = [32]byte{}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := valid
			tc.mutate(&got)
			require.Error(t, validateCEXMarketTradeBatchRef(t, newCEXMarketTradeBatchRef(t, got)))
		})
	}
}

func TestCEXMarketTradeBatchRefsCBORCompatibilityCases(t *testing.T) {
	t.Parallel()

	ref := newCEXMarketTradeBatchRef(t, validCEXMarketTradeBatchRefValues(t))
	orderBookRefs := []CEXOrderBookRef{{
		Exchange:  "mexc",
		Symbol:    "BTCUSDC",
		FetchedAt: 99,
	}}
	allMidsRefs := []HyperliquidAllMidsRef{{
		Kind:              HyperliquidPublicDataKindAllMids,
		Network:           HyperliquidNetworkMainnet,
		MarketType:        HyperliquidMarketTypePerp,
		AssetID:           1,
		Symbol:            "BTCUSDC",
		MidPriceQuoteUnit: "105000.125",
		FetchedAtUnixMS:   1_706_000_000_000,
	}}

	t.Run("batch-new-and-old-readers", func(t *testing.T) {
		t.Parallel()

		batchType := reflect.TypeFor[Batch[testAllMidsTx, testAllMidsReceipt]]()
		batchValue := reflect.New(batchType).Elem()
		batchValue.FieldByName("CEXOrderBookRefs").Set(reflect.ValueOf(orderBookRefs))
		batchValue.FieldByName("HyperliquidAllMidsRefs").Set(reflect.ValueOf(allMidsRefs))
		setCEXMarketTradeBatchRefSlice(t, batchValue, ref)

		raw, err := cbor.Marshal(batchValue.Interface())
		require.NoError(t, err)

		var decoded Batch[testAllMidsTx, testAllMidsReceipt]
		require.NoError(t, cbor.Unmarshal(raw, &decoded))
		require.Equal(t, orderBookRefs, decoded.CEXOrderBookRefs)
		require.Equal(t, allMidsRefs, decoded.HyperliquidAllMidsRefs)
		requireTradeBatchRefSliceEqual(t, ref, reflect.ValueOf(decoded))

		var old legacyCEXBatch
		require.NoError(t, cbor.Unmarshal(raw, &old))
		require.Equal(t, orderBookRefs, old.CEXOrderBookRefs)
		require.Equal(t, allMidsRefs, old.HyperliquidAllMidsRefs)

		legacyRaw, err := cbor.Marshal(legacyCEXBatch{
			CEXOrderBookRefs:       orderBookRefs,
			HyperliquidAllMidsRefs: allMidsRefs,
		})
		require.NoError(t, err)
		// CBOR leaves an absent key untouched on a reused Go value. Decode the
		// legacy payload into a fresh destination to prove old writers omit key 9.
		decoded = Batch[testAllMidsTx, testAllMidsReceipt]{}
		require.NoError(t, cbor.Unmarshal(legacyRaw, &decoded))
		requireTradeBatchRefSliceEmpty(t, reflect.ValueOf(decoded))
	})

	t.Run("event-new-and-old-readers", func(t *testing.T) {
		t.Parallel()

		eventValue := reflect.New(reflect.TypeFor[Event]()).Elem()
		eventValue.FieldByName("CEXOrderBookRefs").Set(reflect.ValueOf(orderBookRefs))
		eventValue.FieldByName("HyperliquidAllMidsRefs").Set(reflect.ValueOf(allMidsRefs))
		setCEXMarketTradeBatchRefSlice(t, eventValue, ref)

		bytesMethod := eventValue.MethodByName("Bytes")
		require.True(t, bytesMethod.IsValid())
		results := bytesMethod.Call(nil)
		require.Len(t, results, 2)
		require.True(t, results[1].IsNil())
		raw, ok := results[0].Interface().([]byte)
		require.True(t, ok)

		var decoded Event
		require.NoError(t, cbor.Unmarshal(raw, &decoded))
		require.Equal(t, orderBookRefs, decoded.CEXOrderBookRefs)
		require.Equal(t, allMidsRefs, decoded.HyperliquidAllMidsRefs)
		requireTradeBatchRefSliceEqual(t, ref, reflect.ValueOf(decoded))

		jsonRaw, err := json.Marshal(eventValue.Interface())
		require.NoError(t, err)

		var eventJSON map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(jsonRaw, &eventJSON))
		tradeRefsJSON, ok := eventJSON["cexMarketTradeBatchRefs"]
		require.True(t, ok)

		jsonRefs := reflect.New(reflect.SliceOf(ref.Type()))
		require.NoError(t, json.Unmarshal(tradeRefsJSON, jsonRefs.Interface()))
		require.Len(t, jsonRefs.Elem().Interface(), 1)
		require.True(t, reflect.DeepEqual(ref.Interface(), jsonRefs.Elem().Index(0).Interface()))

		var old legacyCEXEvent
		require.NoError(t, cbor.Unmarshal(raw, &old))
		require.Equal(t, orderBookRefs, old.CEXOrderBookRefs)
		require.Equal(t, allMidsRefs, old.HyperliquidAllMidsRefs)

		legacyRaw, err := cbor.Marshal(legacyCEXEvent{
			CEXOrderBookRefs:       orderBookRefs,
			HyperliquidAllMidsRefs: allMidsRefs,
		})
		require.NoError(t, err)
		// CBOR leaves an absent key untouched on a reused Go value. Decode the
		// legacy payload into a fresh destination to prove old writers omit key 10.
		decoded = Event{}
		require.NoError(t, cbor.Unmarshal(legacyRaw, &decoded))
		requireTradeBatchRefSliceEmpty(t, reflect.ValueOf(decoded))
	})
}

func TestCEXMarketTradeBatchRefWireSizeCases(t *testing.T) {
	t.Parallel()

	maximum := validCEXMarketTradeBatchRefValues(t)
	maximum.batchID = ^uint64(0)
	maximum.lastSourceTimeMS = ^uint64(0)
	maximum.tradeCount = ^uint32(0)
	maximum.encodedBytes = ^uint32(0)
	maximum.payloadSHA256 = [32]byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}

	ref := newCEXMarketTradeBatchRef(t, maximum)
	require.NoError(t, validateCEXMarketTradeBatchRef(t, ref))

	raw, err := cbor.Marshal(ref.Interface())
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), 128)
}

type legacyCEXBatch struct {
	CEXOrderBookRefs       []CEXOrderBookRef       `cbor:"7,keyasint"`
	HyperliquidAllMidsRefs []HyperliquidAllMidsRef `cbor:"8,keyasint"`
}

type legacyCEXEvent struct {
	CEXOrderBookRefs       []CEXOrderBookRef       `cbor:"8,keyasint"`
	HyperliquidAllMidsRefs []HyperliquidAllMidsRef `cbor:"9,keyasint"`
}

func cexMarketTradeBatchRefType(t *testing.T) reflect.Type {
	t.Helper()

	for _, container := range []reflect.Type{
		reflect.TypeFor[Batch[testAllMidsTx, testAllMidsReceipt]](),
		reflect.TypeFor[Event](),
	} {
		field, ok := container.FieldByName("CEXMarketTradeBatchRefs")
		if !ok {
			continue
		}

		if field.Type.Kind() != reflect.Slice {
			t.Fatalf("%s.CEXMarketTradeBatchRefs is not a slice", container.Name())
		}

		return field.Type.Elem()
	}

	t.Fatal("CEXMarketTradeBatchRefs is absent from both Batch and Event")

	return nil
}

func mustStructField(t *testing.T, typ reflect.Type, name string) reflect.StructField {
	t.Helper()

	field, ok := typ.FieldByName(name)
	require.Truef(t, ok, "%s.%s is absent", typ.Name(), name)

	return field
}

func validCEXMarketTradeBatchRefValues(t *testing.T) cexMarketTradeBatchRefValues {
	t.Helper()

	symbolID, err := DefaultOrderBookIDRegistry.ResolveSymbolID(
		CEXExchangeIDBinance,
		CEXMarketTypeIDSpot,
		"BTCUSDT",
	)
	require.NoError(t, err)

	return cexMarketTradeBatchRefValues{
		exchangeID:        CEXExchangeIDBinance,
		marketTypeID:      CEXMarketTypeIDSpot,
		symbolID:          symbolID,
		batchID:           17,
		firstSourceTimeMS: 1_706_000_000_000,
		lastSourceTimeMS:  1_706_000_001_000,
		tradeCount:        23,
		encodedBytes:      456,
		payloadSHA256: [32]byte{
			1, 2, 3, 4, 5, 6, 7, 8,
			9, 10, 11, 12, 13, 14, 15, 16,
			17, 18, 19, 20, 21, 22, 23, 24,
			25, 26, 27, 28, 29, 30, 31, 32,
		},
	}
}

func newCEXMarketTradeBatchRef(
	t *testing.T,
	values cexMarketTradeBatchRefValues,
) reflect.Value {
	t.Helper()

	ref := reflect.New(cexMarketTradeBatchRefType(t)).Elem()
	for _, field := range []struct {
		name  string
		value any
	}{
		{"ExchangeID", values.exchangeID},
		{"MarketTypeID", values.marketTypeID},
		{"SymbolID", values.symbolID},
		{"BatchID", values.batchID},
		{"FirstSourceTimeMS", values.firstSourceTimeMS},
		{"LastSourceTimeMS", values.lastSourceTimeMS},
		{"TradeCount", values.tradeCount},
		{"EncodedBytes", values.encodedBytes},
		{"PayloadSHA256", values.payloadSHA256},
	} {
		target := ref.FieldByName(field.name)
		require.Truef(t, target.IsValid(), "CEXMarketTradeBatchRef.%s is absent", field.name)
		target.Set(reflect.ValueOf(field.value).Convert(target.Type()))
	}

	return ref
}

func validateCEXMarketTradeBatchRef(t *testing.T, ref reflect.Value) error {
	t.Helper()

	method := ref.MethodByName("Validate")
	if !method.IsValid() && ref.CanAddr() {
		method = ref.Addr().MethodByName("Validate")
	}

	if !method.IsValid() {
		t.Fatal("CEXMarketTradeBatchRef.Validate is absent")
	}

	results := method.Call(nil)
	if len(results) != 1 {
		t.Fatalf("CEXMarketTradeBatchRef.Validate returned %d values; want 1 error", len(results))
	}

	if !results[0].Type().Implements(reflect.TypeFor[error]()) {
		t.Fatalf(
			"CEXMarketTradeBatchRef.Validate return type %s does not implement error",
			results[0].Type(),
		)
	}

	if results[0].IsNil() {
		return nil
	}

	err, ok := results[0].Interface().(error)
	require.True(t, ok)

	return err
}

func setCEXMarketTradeBatchRefSlice(t *testing.T, container, ref reflect.Value) {
	t.Helper()

	field := container.FieldByName("CEXMarketTradeBatchRefs")
	require.True(
		t,
		field.IsValid(),
		"%s.CEXMarketTradeBatchRefs is absent",
		container.Type().Name(),
	)
	require.Equal(t, reflect.Slice, field.Kind())
	require.Equal(t, ref.Type(), field.Type().Elem())

	refs := reflect.MakeSlice(field.Type(), 1, 1)
	refs.Index(0).Set(ref)
	field.Set(refs)
}

func requireTradeBatchRefSliceEqual(t *testing.T, want reflect.Value, container reflect.Value) {
	t.Helper()

	field := container.FieldByName("CEXMarketTradeBatchRefs")
	require.True(
		t,
		field.IsValid(),
		"%s.CEXMarketTradeBatchRefs is absent",
		container.Type().Name(),
	)
	require.Len(t, field.Interface(), 1)
	require.True(t, reflect.DeepEqual(want.Interface(), field.Index(0).Interface()))
}

func requireTradeBatchRefSliceEmpty(t *testing.T, container reflect.Value) {
	t.Helper()

	field := container.FieldByName("CEXMarketTradeBatchRefs")
	require.True(
		t,
		field.IsValid(),
		"%s.CEXMarketTradeBatchRefs is absent",
		container.Type().Name(),
	)
	require.Zero(t, field.Len())
}

func TestOrderBookIDRegistryCoversCompleteStudioMarketCatalogSnapshot(t *testing.T) {
	t.Parallel()

	var snapshot OrderBookIdentityJSON
	require.NoError(t, json.Unmarshal(defaultOrderBookIdentityJSON, &snapshot))

	type marketKey struct {
		exchangeID   CEXExchangeID
		marketTypeID CEXMarketTypeID
	}

	counts := make(map[marketKey]int)
	for _, symbol := range snapshot.Symbols {
		counts[marketKey{symbol.ExchangeID, symbol.MarketTypeID}]++
	}

	require.Equal(t, 748, counts[marketKey{CEXExchangeIDBinance, CEXMarketTypeIDSpot}])
	require.Equal(t, 550, counts[marketKey{CEXExchangeIDBinance, CEXMarketTypeIDPerp}])
	require.Equal(t, 1985, counts[marketKey{CEXExchangeIDMEXC, CEXMarketTypeIDSpot}])
	require.Equal(t, 324, counts[marketKey{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot}])
	require.Equal(t, 184, counts[marketKey{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp}])

	for _, tc := range []struct {
		exchangeID   CEXExchangeID
		marketTypeID CEXMarketTypeID
		symbol       string
	}{
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "AAPLUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "QQQXUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "SKHYXUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "MUXUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "FLOCKUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "ATOMUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "POLUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, "ATOMUSDC"},
	} {
		_, err := DefaultOrderBookIDRegistry.ResolveSymbolID(
			tc.exchangeID,
			tc.marketTypeID,
			tc.symbol,
		)
		require.NoError(t, err, "%d/%d/%s", tc.exchangeID, tc.marketTypeID, tc.symbol)
	}
}

type orderBookIdentityJSONLoaderFunc func(context.Context) (OrderBookIdentityJSON, error)

func (f orderBookIdentityJSONLoaderFunc) LoadOrderBookIdentity(
	ctx context.Context,
) (OrderBookIdentityJSON, error) {
	return f(ctx)
}

func TestOrderBookIDRegistryCoversConfiguredE2EMarkets(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		exchangeID   CEXExchangeID
		marketTypeID CEXMarketTypeID
		symbol       string
	}{
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, "BTCUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, "SPXUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, "PEPEUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, "FLOKIUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, "LINKUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, "UNIUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "PEPEUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "FUNUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, "MOGUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "ETHUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "DOGEUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "LINKUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "AAVEUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "UNIUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "SPXUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "NEARUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "TAOUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "POLUSDC"},
		{CEXExchangeIDBinance, CEXMarketTypeIDSpot, "BTCUSDT"},
		{CEXExchangeIDBinance, CEXMarketTypeIDPerp, "BTCUSDT"},
		{CEXExchangeIDBinance, CEXMarketTypeIDPerp, "SPXUSDT"},
	} {
		name := fmt.Sprintf("%d/%d/%s", tc.exchangeID, tc.marketTypeID, tc.symbol)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			symbolID, err := DefaultOrderBookIDRegistry.ResolveSymbolID(
				tc.exchangeID,
				tc.marketTypeID,
				tc.symbol,
			)
			require.NoError(t, err)
			require.NotZero(t, symbolID)

			label, ok := DefaultOrderBookIDRegistry.SymbolLabel(
				tc.exchangeID,
				tc.marketTypeID,
				symbolID,
			)
			require.True(t, ok)
			require.Equal(t, tc.symbol, label)
		})
	}
}

func TestSymbolAssetsResolvesBaseQuoteAndFailsClosed(t *testing.T) {
	t.Parallel()

	reg := DefaultOrderBookIDRegistry

	// Found: a binance dual-listed pair resolves to explicit base/quote.
	btcSpotID, err := reg.ResolveSymbolID(CEXExchangeIDBinance, CEXMarketTypeIDSpot, "BTCUSDT")
	require.NoError(t, err)

	base, quote, ok := reg.SymbolAssets(CEXExchangeIDBinance, CEXMarketTypeIDSpot, btcSpotID)
	require.True(t, ok)
	require.Equal(t, "BTC", base)
	require.Equal(t, "USDT", quote)

	// Not found: an unregistered symbol id fails closed (ok=false, empty).
	base, quote, ok = reg.SymbolAssets(
		CEXExchangeIDBinance,
		CEXMarketTypeIDSpot,
		CEXSymbolID(999999),
	)
	require.False(t, ok)
	require.Empty(t, base)
	require.Empty(t, quote)

	// Empty asset: a hyperliquid coin-only perp row carries no quote asset, so
	// SymbolAssets fails closed rather than returning a half-populated pair.
	btcPerpID, err := reg.ResolveSymbolID(CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, "BTC")
	require.NoError(t, err)

	_, _, ok = reg.SymbolAssets(CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, btcPerpID)
	require.False(t, ok)
}

func TestStep159JOrderBookIDRegistryScopesSymbolsByExchangeAndMarketType(t *testing.T) {
	t.Parallel()

	btcSpotUSDC, err := DefaultOrderBookIDRegistry.ResolveSymbolID(
		CEXExchangeIDHyperliquid,
		CEXMarketTypeIDSpot,
		"BTCUSDC",
	)
	require.NoError(t, err)

	btcPerpUSDC, err := DefaultOrderBookIDRegistry.ResolveSymbolID(
		CEXExchangeIDHyperliquid,
		CEXMarketTypeIDPerp,
		"BTCUSDC",
	)
	require.NoError(t, err)
	require.Equal(t, btcSpotUSDC, btcPerpUSDC)

	legacyBTCUSDC, err := DefaultOrderBookIDRegistry.ResolveLegacySymbolID(
		CEXExchangeIDHyperliquid,
		"BTCUSDC",
	)
	require.NoError(t, err)
	require.Equal(t, btcSpotUSDC, legacyBTCUSDC)

	_, err = DefaultOrderBookIDRegistry.ResolveSymbolID(
		CEXExchangeIDHyperliquid,
		CEXMarketTypeIDSpot,
		"BTC",
	)
	require.Error(t, err)

	btcPerpBase, err := DefaultOrderBookIDRegistry.ResolveSymbolID(
		CEXExchangeIDHyperliquid,
		CEXMarketTypeIDPerp,
		"BTC",
	)
	require.NoError(t, err)
	require.NotEqual(t, legacyBTCUSDC, btcPerpBase)

	spotLabel, ok := DefaultOrderBookIDRegistry.SymbolLabel(
		CEXExchangeIDHyperliquid,
		CEXMarketTypeIDSpot,
		legacyBTCUSDC,
	)
	require.True(t, ok)
	require.Equal(t, "BTCUSDC", spotLabel)

	perpLabel, ok := DefaultOrderBookIDRegistry.SymbolLabel(
		CEXExchangeIDHyperliquid,
		CEXMarketTypeIDPerp,
		legacyBTCUSDC,
	)
	require.True(t, ok)
	require.Equal(t, "BTCUSDC", perpLabel)
}

func TestStep159JOrderBookIDRegistryLoadsJSONOnlySymbol(t *testing.T) {
	t.Parallel()

	registry, err := NewOrderBookIDRegistryFromJSON(OrderBookIdentityJSON{
		Version: 1,
		Exchanges: []OrderBookExchangeJSON{
			{ID: CEXExchangeIDMEXC, Label: cexExchangeLabelMEXC},
		},
		MarketTypes: []OrderBookMarketJSON{
			{ID: CEXMarketTypeIDSpot, Label: cexMarketTypeLabelSpot},
		},
		Symbols: []OrderBookSymbolJSON{
			{
				ExchangeID:   CEXExchangeIDMEXC,
				MarketTypeID: CEXMarketTypeIDSpot,
				SymbolID:     77,
				Label:        "JSONONLYUSDC",
				BaseAsset:    "JSONONLY",
				QuoteAsset:   "USDC",
			},
		},
	})
	require.NoError(t, err)

	symbolID, err := registry.ResolveSymbolID(
		CEXExchangeIDMEXC,
		CEXMarketTypeIDSpot,
		"JSONONLYUSDC",
	)
	require.NoError(t, err)
	require.Equal(t, CEXSymbolID(77), symbolID)

	label, ok := registry.SymbolLabel(CEXExchangeIDMEXC, CEXMarketTypeIDSpot, symbolID)
	require.True(t, ok)
	require.Equal(t, "JSONONLYUSDC", label)
}

func TestStep159JOrderBookIDRegistryLoaderReturnsErrorsWithoutPanic(t *testing.T) {
	t.Parallel()

	_, err := NewOrderBookIDRegistryFromLoader(context.Background(), nil)
	require.ErrorContains(t, err, "nil order-book identity loader")

	_, err = NewOrderBookIDRegistryFromLoader(
		context.Background(),
		orderBookIdentityJSONLoaderFunc(func(context.Context) (OrderBookIdentityJSON, error) {
			return OrderBookIdentityJSON{}, nil
		}),
	)
	require.ErrorContains(t, err, "validate cex order-book identity")
	require.ErrorContains(t, err, "version is zero")
}

func TestStep159JOrderBookIDRegistryRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	base := func() OrderBookIdentityJSON {
		return OrderBookIdentityJSON{
			Version: 1,
			Exchanges: []OrderBookExchangeJSON{
				{ID: CEXExchangeIDMEXC, Label: cexExchangeLabelMEXC},
			},
			MarketTypes: []OrderBookMarketJSON{
				{ID: CEXMarketTypeIDSpot, Label: cexMarketTypeLabelSpot},
			},
			Symbols: []OrderBookSymbolJSON{
				{
					ExchangeID:   CEXExchangeIDMEXC,
					MarketTypeID: CEXMarketTypeIDSpot,
					SymbolID:     1,
					Label:        "BTCUSDC",
					BaseAsset:    "BTC",
					QuoteAsset:   "USDC",
				},
			},
		}
	}

	for _, tc := range []struct {
		name    string
		mutate  func(*OrderBookIdentityJSON)
		wantErr string
	}{
		{
			name: "duplicate exchange id",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Exchanges = append(doc.Exchanges, OrderBookExchangeJSON{ID: CEXExchangeIDMEXC, Label: "mexc-copy"})
			},
			wantErr: "duplicate order-book exchange id",
		},
		{
			name: "duplicate exchange label",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Exchanges = append(doc.Exchanges, OrderBookExchangeJSON{ID: 9, Label: cexExchangeLabelMEXC})
			},
			wantErr: "duplicate order-book exchange label",
		},
		{
			name: "duplicate market id",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.MarketTypes = append(doc.MarketTypes, OrderBookMarketJSON{ID: CEXMarketTypeIDSpot, Label: "spot-copy"})
			},
			wantErr: "duplicate order-book market_type id",
		},
		{
			name: "duplicate market label",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.MarketTypes = append(doc.MarketTypes, OrderBookMarketJSON{ID: 9, Label: cexMarketTypeLabelSpot})
			},
			wantErr: "duplicate order-book market_type label",
		},
		{
			name: "duplicate symbol id",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Symbols = append(doc.Symbols, OrderBookSymbolJSON{
					ExchangeID:   CEXExchangeIDMEXC,
					MarketTypeID: CEXMarketTypeIDSpot,
					SymbolID:     1,
					Label:        "ETHUSDC",
				})
			},
			wantErr: "duplicate order-book symbol id",
		},
		{
			name: "duplicate symbol label",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Symbols = append(doc.Symbols, OrderBookSymbolJSON{
					ExchangeID:   CEXExchangeIDMEXC,
					MarketTypeID: CEXMarketTypeIDSpot,
					SymbolID:     2,
					Label:        "BTCUSDC",
				})
			},
			wantErr: "duplicate order-book symbol label",
		},
		{
			name: "zero id",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Symbols[0].SymbolID = 0
			},
			wantErr: "non-zero",
		},
		{
			name: "empty label",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Symbols[0].Label = " "
			},
			wantErr: "symbol label is empty",
		},
		{
			name: "unknown exchange",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Symbols[0].ExchangeID = 42
			},
			wantErr: "unknown exchange_id",
		},
		{
			name: "unknown market type",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.Symbols[0].MarketTypeID = 42
			},
			wantErr: "unknown market_type_id",
		},
		{
			name: "conflicting metadata",
			mutate: func(doc *OrderBookIdentityJSON) {
				doc.MarketTypes = append(doc.MarketTypes, OrderBookMarketJSON{
					ID:    CEXMarketTypeIDPerp,
					Label: cexMarketTypeLabelPerp,
				})
				doc.Symbols = append(doc.Symbols, OrderBookSymbolJSON{
					ExchangeID:   CEXExchangeIDMEXC,
					MarketTypeID: CEXMarketTypeIDPerp,
					SymbolID:     1,
					Label:        "BTCUSDC",
					BaseAsset:    "WBTC",
					QuoteAsset:   "USDC",
				})
			},
			wantErr: "conflicting order-book symbol metadata",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			doc := base()
			tc.mutate(&doc)
			_, err := NewOrderBookIDRegistryFromJSON(doc)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCEXPriceLevelCBORUsesCompactArray(t *testing.T) {
	t.Parallel()

	encMode, err := cbor.CanonicalEncOptions().EncMode()
	require.NoError(t, err)

	payload, err := encMode.Marshal(CEXPriceLevel{
		Price:    "0.40987654",
		Quantity: "1234.56789012",
	})
	require.NoError(t, err)

	var asArray []string
	require.NoError(t, cbor.Unmarshal(payload, &asArray))
	require.Equal(t, []string{"0.40987654", "1234.56789012"}, asArray)

	var asMap map[uint64]string
	require.Error(t, cbor.Unmarshal(payload, &asMap))
}

func BenchmarkCEXPriceLevelCBORShape(b *testing.B) {
	type mapLevel struct {
		Price    string `cbor:"1,keyasint"`
		Quantity string `cbor:"2,keyasint"`
	}

	arrayLevels := []CEXPriceLevel{
		{Price: "0.40987654", Quantity: "1234.56789012"},
		{Price: "0.40987655", Quantity: "2345.67890123"},
		{Price: "0.40987656", Quantity: "3456.78901234"},
		{Price: "0.40987657", Quantity: "4567.89012345"},
	}
	mapLevels := []mapLevel{
		{Price: "0.40987654", Quantity: "1234.56789012"},
		{Price: "0.40987655", Quantity: "2345.67890123"},
		{Price: "0.40987656", Quantity: "3456.78901234"},
		{Price: "0.40987657", Quantity: "4567.89012345"},
	}

	arrayPayload, err := cbor.Marshal(arrayLevels)
	require.NoError(b, err)
	mapPayload, err := cbor.Marshal(mapLevels)
	require.NoError(b, err)

	b.Run("array_encode", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, err := cbor.Marshal(arrayLevels)
			require.NoError(b, err)
		}
	})

	b.Run("map_encode", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			_, err := cbor.Marshal(mapLevels)
			require.NoError(b, err)
		}
	})

	b.Run("array_decode", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			var levels []CEXPriceLevel
			require.NoError(b, cbor.Unmarshal(arrayPayload, &levels))
		}
	})

	b.Run("map_decode", func(b *testing.B) {
		b.ReportAllocs()

		for range b.N {
			var levels []mapLevel
			require.NoError(b, cbor.Unmarshal(mapPayload, &levels))
		}
	})
}

// The DefaultOrderBookIDRegistry handle must resolve against a memoized
// registry. Rebuilding it per call re-parses the embedded JSON and every map,
// a cost core pays on its per-pair order-book identity path.
//
//nolint:paralleltest // AllocsPerRun measures the heap; a parallel peer would skew it
func TestDefaultRegistryHandleReusesMemoizedRegistry(t *testing.T) {
	// Warm the memoized registry so the measured runs exclude first-build cost.
	id, err := DefaultOrderBookIDRegistry.ResolveExchangeID(cexExchangeLabelMEXC)
	require.NoError(t, err)
	require.Equal(t, CEXExchangeIDMEXC, id)

	allocs := testing.AllocsPerRun(200, func() {
		_, _ = DefaultOrderBookIDRegistry.ResolveExchangeID(cexExchangeLabelMEXC)
	})

	require.LessOrEqualf(t, allocs, float64(1),
		"handle rebuilds the registry per call: %.0f allocs/op", allocs)
}

// One (exchange, symbol label) must map to a single symbol id across market
// types, or the deprecated single-market label reader cannot disambiguate. The
// registry must reject a document that violates this at load time.
func TestRegistryRejectsDivergentLegacySymbolID(t *testing.T) {
	t.Parallel()

	_, err := NewOrderBookIDRegistryFromJSON(OrderBookIdentityJSON{
		Version: 1,
		Exchanges: []OrderBookExchangeJSON{
			{ID: CEXExchangeIDMEXC, Label: cexExchangeLabelMEXC},
		},
		MarketTypes: []OrderBookMarketJSON{
			{ID: CEXMarketTypeIDSpot, Label: cexMarketTypeLabelSpot},
			{ID: CEXMarketTypeIDPerp, Label: cexMarketTypeLabelPerp},
		},
		Symbols: []OrderBookSymbolJSON{
			{
				ExchangeID: CEXExchangeIDMEXC, MarketTypeID: CEXMarketTypeIDSpot,
				SymbolID: 50, Label: "FOOUSDC", BaseAsset: "FOO", QuoteAsset: "USDC",
			},
			{
				ExchangeID: CEXExchangeIDMEXC, MarketTypeID: CEXMarketTypeIDPerp,
				SymbolID: 77, Label: "FOOUSDC", BaseAsset: "FOO", QuoteAsset: "USDC",
			},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "legacy symbol id")
}
