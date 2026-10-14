package apptypes_test

import (
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/stretchr/testify/require"
	"testing"
)

// NO_TEST_DOUBLE: the production embedded registry resolves literal identities
// from the complete 2026-09-30 Arbitrum listed-perp index inventory. Registration
// is not volume eligibility, a pool/collateral selection, or execution admission.
func TestGMXCompleteIndexInventoryUsesUnderlyingTokenIdentities(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		base string
		id   apptypes.CEXSymbolID
	}{
		{"ETH", 1},
		{"0G", 2},
		{"AAVE", 3},
		{"ADA", 4},
		{"AERO", 5},
		{"AIXBT", 6},
		{"ALGO", 7},
		{"ANIME", 8},
		{"APE", 9},
		{"APT", 10},
		{"AR", 11},
		{"ARB", 12},
		{"ASTER", 13},
		{"ATOM", 14},
		{"AVAX", 15},
		{"AVNT", 16},
		{"BCH", 17},
		{"BERA", 18},
		{"BNB", 19},
		{"BONK", 20},
		{"BRENTOIL", 21},
		{"BTC", 22},
		{"CAKE", 23},
		{"CC", 24},
		{"CHZ", 25},
		{"CRO", 26},
		{"CRV", 27},
		{"CVX", 28},
		{"DASH", 29},
		{"DOGE", 30},
		{"DOLO", 31},
		{"DOT", 32},
		{"DYDX", 33},
		{"EIGEN", 34},
		{"ENA", 35},
		{"FARTCOIN", 36},
		{"FET", 37},
		{"FIL", 38},
		{"FLOKI", 39},
		{"GMX", 40},
		{"GOLD", 41},
		{"HBAR", 42},
		{"HYPE", 43},
		{"ICP", 44},
		{"INJ", 45},
		{"JTO", 46},
		{"JUP", 47},
		{"KAS", 48},
		{"LDO", 49},
		{"LINEA", 50},
		{"LINK", 51},
		{"LIT", 52},
		{"LTC", 53},
		{"MEGA", 54},
		{"MET", 55},
		{"MNT", 56},
		{"MON", 57},
		{"MOODENG", 58},
		{"MORPHO", 59},
		{"NATGAS", 60},
		{"NEAR", 61},
		{"OKB", 62},
		{"ONDO", 63},
		{"OP", 64},
		{"ORDI", 65},
		{"PENDLE", 66},
		{"PENGU", 67},
		{"PEPE", 68},
		{"POL", 69},
		{"PUMP", 70},
		{"QQQ", 71},
		{"RENDER", 72},
		{"S", 73},
		{"SEI", 74},
		{"SHIB", 75},
		{"SILVER", 76},
		{"SKY", 77},
		{"SOL", 78},
		{"SPCX", 79},
		{"SPX6900", 80},
		{"SPY", 81},
		{"STX", 82},
		{"SUI", 83},
		{"SYRUP", 84},
		{"TAO", 85},
		{"TIA", 86},
		{"TRUMP", 87},
		{"TRX", 88},
		{"UNI", 89},
		{"VIRTUAL", 90},
		{"VVV", 91},
		{"WIF", 92},
		{"WLD", 93},
		{"WLFI", 94},
		{"WTIOIL", 95},
		{"XAUT", 96},
		{"XLM", 97},
		{"XMR", 98},
		{"XPL", 99},
		{"XRP", 100},
		{"ZEC", 101},
		{"ZORA", 102},
		{"ZRO", 103},
	} {
		t.Run(test.base, func(t *testing.T) {
			label := test.base + "USDC"
			id, err := apptypes.DefaultOrderBookIDRegistry.ResolveSymbolID(apptypes.CEXExchangeIDGMX, apptypes.CEXMarketTypeIDPerp, label)
			require.NoError(t, err)
			require.Equal(t, test.id, id)
			base, quote, ok := apptypes.DefaultOrderBookIDRegistry.SymbolAssets(apptypes.CEXExchangeIDGMX, apptypes.CEXMarketTypeIDPerp, id)
			require.True(t, ok)
			require.Equal(t, test.base, base)
			require.Equal(t, "USDC", quote)
			resolved, ok := apptypes.DefaultOrderBookIDRegistry.SymbolLabel(apptypes.CEXExchangeIDGMX, apptypes.CEXMarketTypeIDPerp, id)
			require.True(t, ok)
			require.Equal(t, label, resolved)
			_, err = apptypes.DefaultOrderBookIDRegistry.ResolveSymbolID(apptypes.CEXExchangeIDGMX, apptypes.CEXMarketTypeIDSpot, label)
			require.Error(t, err)
		})
	}
	// The "k" prefix is display metadata, not a thousand-unit execution asset.
	for _, displayLabel := range []string{"KPEPEUSDC", "KBONKUSDC", "KFLOKIUSDC", "KSHIBUSDC", "UNKNOWNUSDC"} {
		_, err := apptypes.DefaultOrderBookIDRegistry.ResolveSymbolID(apptypes.CEXExchangeIDGMX, apptypes.CEXMarketTypeIDPerp, displayLabel)
		require.Error(t, err)
	}
}
