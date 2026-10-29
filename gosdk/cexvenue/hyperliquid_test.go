package cexvenue

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const hlObservedAt = int64(1_793_318_400_000)

func recordedHyperliquid(t *testing.T) HyperliquidInput {
	t.Helper()

	var doc struct {
		SpotMeta json.RawMessage `json:"spotMeta"`
	}

	raw := readFixture(t, filepath.Join("testdata", "hyperliquid_mainnet_recorded_2026_10_30.json"))
	require.NoError(t, json.Unmarshal(raw, &doc))

	return HyperliquidInput{
		Network:          "mainnet",
		SpotMeta:         doc.SpotMeta,
		ObservedAtUnixMs: hlObservedAt,
	}
}

func byAsset(out Normalized) map[apptypes.CEXMarketTypeID]map[uint32]apptypes.CEXMarketFacts {
	idx := map[apptypes.CEXMarketTypeID]map[uint32]apptypes.CEXMarketFacts{
		MarketTypeSpot: {},
		MarketTypePerp: {},
	}
	for _, f := range out.Markets {
		idx[f.MarketTypeID][f.VenueAssetID] = f
	}

	return idx
}

func TestNormalizeHyperliquidSpotFacts(t *testing.T) {
	t.Parallel()

	out, err := NormalizeHyperliquid(recordedHyperliquid(t))
	require.NoError(t, err)
	require.Empty(t, out.Skipped)

	spot := byAsset(out)[MarketTypeSpot]
	require.Len(t, spot, 347)
	require.Equal(t, "PURR/USDC", spot[10000].VenueCoin)
	require.Equal(t, apptypes.CEXMarketFacts{
		ExchangeID:   ExchangeHyperliquid,
		MarketTypeID: MarketTypeSpot,
		Label:        "HYPEUSDC",
		Network:      "mainnet",
		VenueCoin:    "@107",
		Kind:         apptypes.CEXMarketKindHyperliquidSpot,
		Status:       apptypes.CEXMarketStatusTrading,
		BaseAsset:    "HYPE",
		QuoteAsset:   "USDC",
		VenueAssetID: 10107,
		SizeDecimals: u8(2),
		BaseTokenIndex: u32(
			150,
		),
		QuoteTokenIndex:  u32(0),
		BaseWeiDecimals:  u8(8),
		QuoteWeiDecimals: u8(8),
		ObservedAtUnixMs: hlObservedAt,
	}, spot[10107])
}

func TestNormalizeHyperliquidRejectsBadInput(t *testing.T) {
	t.Parallel()

	inputs := map[string]func(*HyperliquidInput){
		"bad spot":      func(in *HyperliquidInput) { in.SpotMeta = []byte(`{"tokens":`) },
		"unknown token": func(in *HyperliquidInput) { in.SpotMeta = []byte(`{"tokens":[],"universe":[{"tokens":[1,0]}]}`) },
		"no network":    func(in *HyperliquidInput) { in.Network = "" },
	}
	for name, mutate := range inputs {
		in := recordedHyperliquid(t)
		mutate(&in)

		_, err := NormalizeHyperliquid(in)
		require.ErrorIs(t, err, ErrInvalidVenueResponse, name)
	}
}
