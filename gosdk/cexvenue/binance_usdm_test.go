package cexvenue

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestNormalizeBinanceUSDM(t *testing.T) {
	t.Parallel()

	raw := readFixture(
		t,
		filepath.Join("testdata", "binance_usdm_exchange_info_recorded_2026_10_30.json"),
	)
	out, err := NormalizeBinanceUSDM(raw, venueObservedAt)
	require.NoError(t, err)

	got := byLabel(out)
	require.Len(t, got, 5)
	require.Equal(t, apptypes.CEXMarketFacts{
		ExchangeID:          ExchangeBinance,
		MarketTypeID:        MarketTypePerp,
		Label:               "XAUUSDT",
		VenueCoin:           "XAUUSDT",
		Kind:                apptypes.CEXMarketKindBinanceTradFiPerp,
		Status:              apptypes.CEXMarketStatusTrading,
		VenueStatus:         "TRADING",
		BaseAsset:           "XAU",
		QuoteAsset:          "USDT",
		Category:            "COMMODITY",
		BaseAssetPrecision:  u8(8),
		QuoteAssetPrecision: u8(8),
		QuantityStep:        "0.001",
		PriceTick:           "0.01",
		MinNotional:         "5",
		ObservedAtUnixMs:    venueObservedAt,
	}, got["XAUUSDT"])
	require.Equal(t, apptypes.CEXMarketKindBinanceUSDMPerpetual, got["BTCUSDT"].Kind)
	require.Equal(t, "COIN", got["BTCUSDT"].Category)
	require.Equal(t, "EQUITY", got["TSLAUSDT"].Category)
	require.Equal(t, apptypes.CEXMarketStatusDelisted, got["OMGUSDT"].Status, "SETTLING")
	require.Equal(t, apptypes.CEXMarketStatusHalted, got["GAIBUSDT"].Status, "PENDING_TRADING")

	require.Len(t, out.Skipped, 2, "the dated quarterly and the non-ASCII symbol")
	require.Equal(t, "BTCUSDT_261225", out.Skipped[0].VenueCoin)

	for contract, kind := range map[string]apptypes.CEXMarketKind{
		"PERPETUAL":         apptypes.CEXMarketKindBinanceUSDMPerpetual,
		"TRADIFI_PERPETUAL": apptypes.CEXMarketKindBinanceTradFiPerp,
		"CURRENT_QUARTER":   apptypes.CEXMarketKindBinanceOtherFutures,
		"NEXT_QUARTER":      apptypes.CEXMarketKindBinanceOtherFutures,
		"":                  apptypes.CEXMarketKindBinanceOtherFutures,
	} {
		require.Equal(t, kind, binanceUSDMKind(contract), contract)
	}
}
