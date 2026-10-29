package cexvenue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func hlFact(mt apptypes.CEXMarketTypeID, label, coin string) apptypes.CEXMarketFacts {
	return apptypes.CEXMarketFacts{
		ExchangeID:   ExchangeHyperliquid,
		MarketTypeID: mt,
		Network:      "mainnet",
		Label:        label,
		VenueCoin:    coin,
	}
}

func TestFinalizeRefusesDuplicateLabels(t *testing.T) {
	t.Parallel()

	facts := []apptypes.CEXMarketFacts{
		hlFact(MarketTypePerp, "KMXUSDC", "KMX"),
		hlFact(MarketTypePerp, "KMXUSDC", "km:X"),
		hlFact(MarketTypeSpot, "KMXUSDC", "@9"),
		hlFact(MarketTypePerp, "BTCUSDC", "BTC"),
	}
	ruleSkip := SkippedMarket{
		MarketTypeID: MarketTypePerp,
		VenueCoin:    "a:b",
		Reason:       SkipReasonLabelRule,
	}

	out := finalize(facts, []SkippedMarket{ruleSkip})
	require.Len(t, out.Markets, 2)
	require.Equal(t, "@9", out.Markets[0].VenueCoin)
	require.Equal(t, "BTCUSDC", out.Markets[1].Label)

	dup := func(coin string) SkippedMarket {
		return SkippedMarket{
			MarketTypeID: MarketTypePerp,
			Network:      "mainnet",
			VenueCoin:    coin,
			Reason:       SkipReasonDuplicateLabel,
		}
	}
	require.Equal(t, []SkippedMarket{dup("KMX"), ruleSkip, dup("km:X")}, out.Skipped)
}
