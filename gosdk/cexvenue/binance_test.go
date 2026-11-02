package cexvenue

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const venueObservedAt = int64(1_793_318_400_000)

func byLabel(out Normalized) map[string]apptypes.CEXMarketFacts {
	idx := make(map[string]apptypes.CEXMarketFacts, len(out.Markets))
	for _, f := range out.Markets {
		idx[f.Label] = f
	}

	return idx
}

func TestNormalizeBinanceSpot(t *testing.T) {
	t.Parallel()

	raw := readFixture(
		t,
		filepath.Join("testdata", "binance_spot_exchange_info_recorded_2026_10_30.json"),
	)
	out, err := NormalizeBinanceSpot(raw, venueObservedAt)
	require.NoError(t, err)

	got := byLabel(out)
	require.Len(t, got, 4)
	require.Equal(t, apptypes.CEXMarketFacts{
		ExchangeID:          ExchangeBinance,
		MarketTypeID:        MarketTypeSpot,
		Label:               "BTCUSDT",
		VenueCoin:           "BTCUSDT",
		Kind:                apptypes.CEXMarketKindBinanceSpot,
		Status:              apptypes.CEXMarketStatusTrading,
		VenueStatus:         "TRADING",
		BaseAsset:           "BTC",
		QuoteAsset:          "USDT",
		BaseAssetPrecision:  u8(8),
		QuoteAssetPrecision: u8(8),
		QuantityStep:        "0.00001000",
		PriceTick:           "0.01000000",
		MinNotional:         "5.00000000",
		ObservedAtUnixMs:    venueObservedAt,
	}, got["BTCUSDT"])
	require.Equal(t, apptypes.CEXMarketStatusDelisted, got["ICXUSDT"].Status)
	require.Equal(t, "BREAK", got["ICXUSDT"].VenueStatus)
	require.Equal(t, "ETH", got["ETHBTC"].BaseAsset)

	require.Equal(t, []SkippedMarket{
		{
			MarketTypeID: MarketTypeSpot,
			VenueCoin:    string([]rune{0x5e01, 0x5b89, 0x4eba, 0x751f}) + "USDT",
			Reason:       SkipReasonLabelRule,
		},
	}, out.Skipped)

	_, err = NormalizeBinanceSpot([]byte(`{"symbols":{}}`), venueObservedAt)
	require.ErrorIs(t, err, ErrInvalidVenueResponse)
}
