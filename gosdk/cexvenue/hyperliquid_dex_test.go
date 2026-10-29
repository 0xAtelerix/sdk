package cexvenue

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Every frozen v9 Hyperliquid market with a mainnet asset id gets its frozen
// label and assets back from the recorded venue metadata.
func TestNormalizeHyperliquidReproducesFrozenLabels(t *testing.T) {
	t.Parallel()

	out, err := NormalizeHyperliquid(recordedHyperliquid(t))
	require.NoError(t, err)
	require.Empty(t, out.Skipped)

	var doc apptypes.OrderBookIdentityJSON
	require.NoError(t, json.Unmarshal(frozenIdentityJSON(t), &doc))

	idx, checked := byAsset(out), 0

	for _, row := range doc.Symbols {
		assetID, ok := row.VenueAssetIDs["mainnet"]
		if row.ExchangeID != ExchangeHyperliquid || !ok {
			continue
		}

		got := idx[row.MarketTypeID][assetID]
		require.Equal(t, row.Label, got.Label, assetID)
		require.Equal(t, row.BaseAsset, got.BaseAsset, row.Label)
		require.Equal(t, row.QuoteAsset, got.QuoteAsset, row.Label)

		checked++
	}

	require.Equal(t, 624, checked)
}

func TestNormalizeHyperliquidDexFacts(t *testing.T) {
	t.Parallel()

	out, err := NormalizeHyperliquid(recordedHyperliquid(t))
	require.NoError(t, err)

	perp := byAsset(out)[MarketTypePerp]
	require.Equal(t, "BTCUSDC", perp[0].Label)
	require.Equal(t, apptypes.CEXMarketStatusDelisted, perp[3].Status, "MATIC")

	tsla := perp[110001]
	require.Equal(t, "XYZTSLAUSDC", tsla.Label)
	require.Equal(t, "xyz:TSLA", tsla.VenueCoin)
	require.Equal(t, "xyz", tsla.BuilderDEX)
	require.Equal(t, "stocks", tsla.Category)
	require.Equal(t, apptypes.CEXMarketKindHyperliquidBuilder, tsla.Kind)

	missing := recordedHyperliquid(t)
	delete(missing.PerpMetas, "flx")

	_, err = NormalizeHyperliquid(missing)
	require.ErrorContains(t, err, `meta missing for dex "flx"`)

	flx := perp[120000]
	require.Equal(
		t,
		"FLXTSLAUSDH",
		flx.Label,
		"builder perps are quoted in the dex collateral token",
	)
	require.Equal(t, "USDH", flx.QuoteAsset)
	require.Equal(t, apptypes.CEXMarketStatusDelisted, flx.Status)
}
