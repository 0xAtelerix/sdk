package apptypes

import (
	"context"
	"fmt"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

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
