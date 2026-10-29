package cexvenue

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestNormalizeMEXCSpot(t *testing.T) {
	t.Parallel()

	raw := readFixture(
		t,
		filepath.Join("testdata", "mexc_spot_exchange_info_recorded_2026_10_30.json"),
	)
	out, err := NormalizeMEXCSpot(raw, venueObservedAt)
	require.NoError(t, err)

	got := byLabel(out)
	require.Len(t, got, 6)
	require.Equal(t, apptypes.CEXMarketFacts{
		ExchangeID:          ExchangeMEXC,
		MarketTypeID:        MarketTypeSpot,
		Label:               "BTCUSDT",
		VenueCoin:           "BTCUSDT",
		Kind:                apptypes.CEXMarketKindMEXCSpot,
		Status:              apptypes.CEXMarketStatusTrading,
		VenueStatus:         "1",
		BaseAsset:           "BTC",
		QuoteAsset:          "USDT",
		BaseAssetPrecision:  u8(8),
		QuoteAssetPrecision: u8(2),
		MinNotional:         "1",
		ObservedAtUnixMs:    venueObservedAt,
	}, got["BTCUSDT"])

	paxg := got["GOLD(PAXG)USDT"]
	require.Equal(t, "GOLD(PAXG)", paxg.BaseAsset, "a frozen label keeps its spelling")
	require.Equal(
		t,
		apptypes.CEXMarketStatusHalted,
		got["PREDICTUSDT"].Status,
		"status 2 is paused",
	)
	require.Equal(t, "2", got["PREDICTUSDT"].VenueStatus)

	require.Len(t, out.Skipped, 2, "a new label with parentheses and a non-ASCII symbol")
	require.Equal(t, "GOLD(XAUT)EUR", out.Skipped[0].VenueCoin)

	_, err = NormalizeMEXCSpot([]byte(`[]`), venueObservedAt)
	require.ErrorIs(t, err, ErrInvalidVenueResponse)
}
