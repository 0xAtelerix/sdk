package apptypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsKnownOrDerivedSymbolID(t *testing.T) {
	t.Parallel()

	derived := SymbolID(CEXExchangeIDBinance, "NEWUSDT")

	cases := []struct {
		name         string
		exchangeID   CEXExchangeID
		marketTypeID CEXMarketTypeID
		symbolID     CEXSymbolID
		want         bool
	}{
		{"frozen", CEXExchangeIDBinance, CEXMarketTypeIDPerp, 1, true},
		{"derived", CEXExchangeIDBinance, CEXMarketTypeIDPerp, derived, true},
		{"derived spot", CEXExchangeIDBinance, CEXMarketTypeIDSpot, derived, true},
		{"derived base", CEXExchangeIDMEXC, CEXMarketTypeIDSpot, CEXDerivedSymbolIDBase, true},
		{"between frozen and derived", CEXExchangeIDBinance, CEXMarketTypeIDPerp, 40_000, false},
		{"just below derived", CEXExchangeIDBinance, CEXMarketTypeIDPerp, 0x7fffffff, false},
		{"zero", CEXExchangeIDBinance, CEXMarketTypeIDPerp, 0, false},
		{"unknown exchange derived", 99, CEXMarketTypeIDPerp, derived, false},
		{"unknown exchange frozen", 99, CEXMarketTypeIDPerp, 1, false},
		{"unknown market type", CEXExchangeIDBinance, 99, derived, false},
	}

	for _, tc := range cases {
		require.Equal(t, tc.want,
			IsKnownOrDerivedSymbolID(tc.exchangeID, tc.marketTypeID, tc.symbolID), tc.name)
	}
}

func TestCEXCandleBatchRefValidateAcceptsDerivedSymbolID(t *testing.T) {
	t.Parallel()

	ref := validCEXCandleBatchRef()
	ref.SymbolID = SymbolID(CEXExchangeIDBinance, "NEWUSDT")
	require.NoError(t, ref.Validate())

	ref.SymbolID = 40_000
	require.ErrorIs(t, ref.Validate(), ErrCEXCandleBatchRefInvalid)
}

func TestCEXMarketTradeBatchRefValidateAcceptsDerivedSymbolID(t *testing.T) {
	t.Parallel()

	values := validCEXMarketTradeBatchRefValues(t)
	values.symbolID = SymbolID(CEXExchangeIDBinance, "NEWUSDT")
	require.NoError(t, validateCEXMarketTradeBatchRef(t, newCEXMarketTradeBatchRef(t, values)))

	values.symbolID = 40_000
	require.Error(t, validateCEXMarketTradeBatchRef(t, newCEXMarketTradeBatchRef(t, values)))
}
