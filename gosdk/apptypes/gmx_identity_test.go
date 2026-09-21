package apptypes_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// NO_TEST_DOUBLE: literal inputs exercise the production embedded identity
// registry and immutable reference validators; no external owner is replaced.

func TestGMXPerpIdentityRegistryRoundTrip(t *testing.T) {
	t.Parallel()

	const expectedExchangeID = apptypes.CEXExchangeID(4)
	require.Equal(t, expectedExchangeID, apptypes.CEXExchangeIDGMX)

	exchangeID, err := apptypes.DefaultOrderBookIDRegistry.ResolveExchangeID("gmx")
	require.NoError(t, err)
	require.Equal(t, expectedExchangeID, exchangeID)

	exchangeLabel, ok := apptypes.DefaultOrderBookIDRegistry.ExchangeLabel(exchangeID)
	require.True(t, ok)
	require.Equal(t, "gmx", exchangeLabel)

	marketTypeID, err := apptypes.DefaultOrderBookIDRegistry.ResolveMarketTypeID("perps")
	require.NoError(t, err)
	require.Equal(t, apptypes.CEXMarketTypeIDPerp, marketTypeID)

	symbolID, err := apptypes.DefaultOrderBookIDRegistry.ResolveSymbolID(
		exchangeID,
		marketTypeID,
		"ETHUSDC",
	)
	require.NoError(t, err)
	require.Equal(t, apptypes.CEXSymbolID(1), symbolID)

	symbolLabel, ok := apptypes.DefaultOrderBookIDRegistry.SymbolLabel(
		exchangeID,
		marketTypeID,
		symbolID,
	)
	require.True(t, ok)
	require.Equal(t, "ETHUSDC", symbolLabel)

	baseAsset, quoteAsset, ok := apptypes.DefaultOrderBookIDRegistry.SymbolAssets(
		exchangeID,
		marketTypeID,
		symbolID,
	)
	require.True(t, ok)
	require.Equal(t, "ETH", baseAsset)
	require.Equal(t, "USDC", quoteAsset)

	legacySymbolID, err := apptypes.DefaultOrderBookIDRegistry.ResolveLegacySymbolID(
		exchangeID,
		"ETHUSDC",
	)
	require.NoError(t, err)
	require.Equal(t, symbolID, legacySymbolID)

	_, err = apptypes.DefaultOrderBookIDRegistry.ResolveSymbolID(
		exchangeID,
		apptypes.CEXMarketTypeIDSpot,
		"ETHUSDC",
	)
	require.Error(t, err)
}

func TestGMXPerpIdentityUsesExistingPublicDataReferenceContracts(t *testing.T) {
	t.Parallel()

	const (
		exchangeID = apptypes.CEXExchangeIDGMX
		symbolID   = apptypes.CEXSymbolID(1)
	)

	tradeRef := apptypes.CEXMarketTradeBatchRef{
		ExchangeID: exchangeID, MarketTypeID: apptypes.CEXMarketTypeIDPerp, SymbolID: symbolID,
		BatchID: 1, FirstSourceTimeMS: 1, LastSourceTimeMS: 2, TradeCount: 1,
		EncodedBytes: 1, PayloadSHA256: [32]byte{31: 1},
	}
	require.NoError(t, tradeRef.Validate())

	candleRef := apptypes.CEXCandleBatchRef{
		ExchangeID: exchangeID, MarketTypeID: apptypes.CEXMarketTypeIDPerp, SymbolID: symbolID,
		TimeframeMS: 60_000, PriceSource: apptypes.CEXCandlePriceSourceVenueAPI,
		Policy: apptypes.CEXCandlePolicyConfirmed, GenerationID: 1, BatchID: 1,
		BatchIndex: 0, BatchCount: 1, BarCount: 1, FirstBarStartMS: 60_000,
		LastBarCloseMS: 120_000, EncodedBytes: 1, PayloadSHA256: [32]byte{31: 1},
	}
	require.NoError(t, candleRef.Validate())
}
