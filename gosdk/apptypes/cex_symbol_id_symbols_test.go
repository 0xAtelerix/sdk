package apptypes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSymbolsEnumeratesFrozenRowsSortedByID(t *testing.T) {
	t.Parallel()

	var doc OrderBookIdentityJSON
	require.NoError(t, json.Unmarshal(defaultOrderBookIdentityJSON, &doc))

	type marketKey struct {
		exchangeID   CEXExchangeID
		marketTypeID CEXMarketTypeID
	}

	want := map[marketKey]map[CEXSymbolID]CEXSymbolEntry{}

	for _, row := range doc.Symbols {
		key := marketKey{row.ExchangeID, row.MarketTypeID}
		if want[key] == nil {
			want[key] = map[CEXSymbolID]CEXSymbolEntry{}
		}

		want[key][row.SymbolID] = CEXSymbolEntry{
			SymbolID:   row.SymbolID,
			Label:      row.Label,
			BaseAsset:  row.BaseAsset,
			QuoteAsset: row.QuoteAsset,
			Canonical:  row.Canonical,
		}
	}

	require.Len(t, want[marketKey{CEXExchangeIDMEXC, CEXMarketTypeIDSpot}], 1985)
	require.Len(t, want[marketKey{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot}], 338)
	require.Len(t, want[marketKey{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp}], 301)
	require.Len(t, want[marketKey{CEXExchangeIDBinance, CEXMarketTypeIDSpot}], 799)
	require.Len(t, want[marketKey{CEXExchangeIDBinance, CEXMarketTypeIDPerp}], 569)

	for key, rows := range want {
		got := DefaultOrderBookIDRegistry.Symbols(key.exchangeID, key.marketTypeID)
		require.Len(t, got, len(rows))

		for i, entry := range got {
			require.Equal(t, rows[entry.SymbolID], entry)

			if i > 0 {
				require.Less(t, got[i-1].SymbolID, entry.SymbolID)
			}
		}

		// The result is a copy: changing it never changes the registry.
		got[0].Label = "CHANGED"
		require.NotEqual(t, "CHANGED",
			DefaultOrderBookIDRegistry.Symbols(key.exchangeID, key.marketTypeID)[0].Label)
	}

	require.Empty(t, DefaultOrderBookIDRegistry.Symbols(CEXExchangeIDMEXC, CEXMarketTypeIDPerp))
	require.Empty(t, DefaultOrderBookIDRegistry.Symbols(99, CEXMarketTypeIDSpot))
}
