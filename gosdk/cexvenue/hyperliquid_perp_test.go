package cexvenue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestHyperliquidPerpFacts(t *testing.T) {
	t.Parallel()

	btc, base := hlPerpFacts("", 0, 0, &hlPerpAsset{Name: "BTC", SzDecimals: 5, MaxLeverage: 40})
	require.Equal(t, "BTC", base)
	require.Equal(t, apptypes.CEXMarketFacts{
		MarketTypeID: MarketTypePerp,
		VenueCoin:    "BTC",
		Kind:         apptypes.CEXMarketKindHyperliquidCorePerp,
		Status:       apptypes.CEXMarketStatusTrading,
		BaseAsset:    "BTC",
		SizeDecimals: u8(5),
		MaxLeverage:  40,
	}, btc)

	kpepe, base := hlPerpFacts("", 0, 15, &hlPerpAsset{Name: "kPEPE"})
	require.Equal(t, "kPEPE", base, "the label keeps the venue spelling")
	require.Equal(t, "KPEPE", kpepe.BaseAsset)
	require.Equal(t, uint32(15), kpepe.VenueAssetID)

	gone, _ := hlPerpFacts("", 0, 33, &hlPerpAsset{
		Name: "HPOS", IsDelisted: true, OnlyIsolated: true, MarginMode: "strictIsolated",
	})
	require.Equal(t, apptypes.CEXMarketStatusDelisted, gone.Status)
	require.Equal(t, "isDelisted", gone.VenueStatus)
	require.True(t, gone.OnlyIsolated)
	require.Equal(t, "strictIsolated", gone.MarginMode)

	tsla, base := hlPerpFacts("xyz", 1, 1, &hlPerpAsset{Name: "xyz:TSLA"})
	require.Equal(t, "XYZTSLA", base)
	require.Equal(t, uint32(110001), tsla.VenueAssetID)
	require.Equal(t, "xyz", tsla.BuilderDEX)
	require.Equal(t, apptypes.CEXMarketKindHyperliquidBuilder, tsla.Kind)

	_, base = hlPerpFacts("xyz", 1, 2, &hlPerpAsset{Name: "abc:TSLA"})
	require.Equal(
		t,
		"XYZabc:TSLA",
		base,
		"a coin outside its dex keeps the colon and fails the label rule",
	)
}
