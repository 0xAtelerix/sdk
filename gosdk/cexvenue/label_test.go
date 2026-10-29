package cexvenue

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestVenueIDsMatchRegistry(t *testing.T) {
	t.Parallel()

	for label, want := range map[string]apptypes.CEXExchangeID{
		"mexc": ExchangeMEXC, "hyperliquid": ExchangeHyperliquid, "binance": ExchangeBinance,
	} {
		got, err := apptypes.DefaultOrderBookIDRegistry.ResolveExchangeID(label)
		require.NoError(t, err)
		require.Equal(t, want, got, label)
	}

	for label, want := range map[string]apptypes.CEXMarketTypeID{"spot": MarketTypeSpot, "perp": MarketTypePerp} {
		got, err := apptypes.DefaultOrderBookIDRegistry.ResolveMarketTypeID(label)
		require.NoError(t, err)
		require.Equal(t, want, got, label)
	}
}

func TestNewMarketLabel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		exchange  apptypes.CEXExchangeID
		market    apptypes.CEXMarketTypeID
		candidate string
		ok        bool
	}{
		{"new letters and digits", ExchangeBinance, MarketTypePerp, "NEW1USDT", true},
		{"lower case kept", ExchangeHyperliquid, MarketTypePerp, "kNEWUSDC", true},
		{"frozen parentheses", ExchangeMEXC, MarketTypeSpot, "GOLD(PAXG)USDT", true},
		{"frozen label on another exchange is not exempt", ExchangeBinance, MarketTypeSpot, "GOLD(PAXG)USDT", false},
		{"new parentheses", ExchangeMEXC, MarketTypeSpot, "GOLD(XAUT)EUR", false},
		{"dated future", ExchangeBinance, MarketTypePerp, "BTCUSDT_261225", false},
		{"non ascii letters", ExchangeBinance, MarketTypeSpot, "\u5e01\u5b89USDT", false},
		{"colon", ExchangeHyperliquid, MarketTypePerp, "xyz:NEWUSDC", false},
		{"padded frozen", ExchangeMEXC, MarketTypeSpot, " BTCUSDT", false},
		{"empty", ExchangeMEXC, MarketTypeSpot, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			label, ok := NewMarketLabel(tc.exchange, tc.market, tc.candidate)
			require.Equal(t, tc.ok, ok)

			if tc.ok {
				require.Equal(t, tc.candidate, label)
			} else {
				require.Empty(t, label)
			}
		})
	}
}

// Every frozen v9 label must keep its spelling under the label rule.
func TestNewMarketLabelKeepsEveryFrozenLabel(t *testing.T) {
	t.Parallel()

	var doc apptypes.OrderBookIdentityJSON
	require.NoError(t, json.Unmarshal(frozenIdentityJSON(t), &doc))
	require.NotEmpty(t, doc.Symbols)

	for _, row := range doc.Symbols {
		label, ok := NewMarketLabel(row.ExchangeID, row.MarketTypeID, row.Label)
		require.True(t, ok, row.Label)
		require.Equal(t, row.Label, label)
	}
}
