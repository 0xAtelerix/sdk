package cexvenue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestCollectorAppliesLabelRuleAndCommonFields(t *testing.T) {
	t.Parallel()

	c := &collector{exchangeID: ExchangeBinance, observedAt: 7}
	c.add(&apptypes.CEXMarketFacts{MarketTypeID: MarketTypePerp, VenueCoin: "BTCUSDT"}, "BTCUSDT")
	c.add(
		&apptypes.CEXMarketFacts{MarketTypeID: MarketTypePerp, VenueCoin: "BTCUSDT_261225"},
		"BTCUSDT_261225",
	)

	require.Equal(t, []apptypes.CEXMarketFacts{
		{
			ExchangeID:       ExchangeBinance,
			MarketTypeID:     MarketTypePerp,
			Label:            "BTCUSDT",
			VenueCoin:        "BTCUSDT",
			ObservedAtUnixMs: 7,
		},
	}, c.facts)
	require.Equal(t, []SkippedMarket{
		{MarketTypeID: MarketTypePerp, VenueCoin: "BTCUSDT_261225", Reason: SkipReasonLabelRule},
	}, c.skipped)

	var out struct{ A int }
	require.ErrorIs(t, decode("x", []byte(`{"A":"s"}`), &out), ErrInvalidVenueResponse)
}
