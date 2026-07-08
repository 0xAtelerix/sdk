package apptypes

import (
	"fmt"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

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
		t.Run(fmt.Sprintf("%d/%d/%s", tc.exchangeID, tc.marketTypeID, tc.symbol), func(t *testing.T) {
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
