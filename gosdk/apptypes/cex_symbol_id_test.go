package apptypes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestSymbolIDResolvesEveryFrozenRow pins rule 1: a label in the frozen v9
// table on its exchange keeps its id, whatever market type the row has.
func TestSymbolIDResolvesEveryFrozenRow(t *testing.T) {
	t.Parallel()

	var doc OrderBookIdentityJSON
	require.NoError(t, json.Unmarshal(defaultOrderBookIdentityJSON, &doc))
	require.Len(t, doc.Symbols, 3992)

	for _, row := range doc.Symbols {
		require.Equal(t, row.SymbolID, SymbolID(row.ExchangeID, row.Label),
			"exchange_id=%d label=%q", row.ExchangeID, row.Label)
		require.Equal(t, row.SymbolID,
			DefaultOrderBookIDRegistry.SymbolID(row.ExchangeID, row.Label))
	}
}

// TestSymbolIDGoldenVectors pins the derived formula so it can never drift.
// The expected values were computed outside Go from the design formula.
func TestSymbolIDGoldenVectors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		exchangeID CEXExchangeID
		label      string
		want       CEXSymbolID
	}{
		{CEXExchangeIDBinance, "NEWUSDT", 0x87881b5a},
		{CEXExchangeIDHyperliquid, "XYZNIFTYUSDC", 0xf15eaaa7},
		{CEXExchangeIDMEXC, "NEWUSDT", 0xc2bd1784},
	}

	for _, tc := range cases {
		require.Equal(t, tc.want, SymbolID(tc.exchangeID, tc.label),
			"exchange_id=%d label=%q", tc.exchangeID, tc.label)
	}
}

// TestSymbolIDIgnoresMarketType: spot and perp NEWUSDT share one id because the
// market type is not an input.
func TestSymbolIDIgnoresMarketType(t *testing.T) {
	t.Parallel()

	_, spotErr := DefaultOrderBookIDRegistry.ResolveSymbolID(
		CEXExchangeIDBinance, CEXMarketTypeIDSpot, "NEWUSDT")
	require.Error(t, spotErr)

	_, perpErr := DefaultOrderBookIDRegistry.ResolveSymbolID(
		CEXExchangeIDBinance, CEXMarketTypeIDPerp, "NEWUSDT")
	require.Error(t, perpErr)

	require.Equal(t, CEXSymbolID(0x87881b5a), SymbolID(CEXExchangeIDBinance, "NEWUSDT"))
}

func TestSymbolIDEmptyLabelIsZero(t *testing.T) {
	t.Parallel()

	require.Zero(t, SymbolID(CEXExchangeIDBinance, ""))
}

// TestSymbolIDDerivedPropertyStableAndInRange: for any non-frozen label the id
// is stable and in the derived range.
func TestSymbolIDDerivedPropertyStableAndInRange(t *testing.T) {
	t.Parallel()

	registry, err := defaultOrderBookRegistry()
	require.NoError(t, err)

	rapid.Check(t, func(rt *rapid.T) {
		exchangeID := CEXExchangeID(rapid.Uint16().Draw(rt, "exchange_id"))
		label := rapid.StringMatching(`[A-Z0-9]{1,24}`).Draw(rt, "label")

		frozen := registry.legacyCandidates[cexLegacySymbolKey{
			exchangeID: exchangeID,
			label:      label,
		}]

		first := SymbolID(exchangeID, label)
		require.Equal(rt, first, SymbolID(exchangeID, label))

		if len(frozen) > 0 {
			require.Equal(rt, frozen[0].SymbolID, first)

			return
		}

		require.GreaterOrEqual(rt, first, CEXDerivedSymbolIDBase)
	})
}

// TestEmbeddedOrderBookIdentityIsFrozenV9: the embedded table is the frozen
// v9 table byte for byte. New markets get ids from SymbolID, not new rows.
func TestEmbeddedOrderBookIdentityIsFrozenV9(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256(defaultOrderBookIdentityJSON)
	require.Equal(t, CEXOrderBookIdentityV9SHA256, hex.EncodeToString(sum[:]),
		"cex_order_book_identity.json is the frozen v9 table and must not change; "+
			"new markets get ids from SymbolID")
}
