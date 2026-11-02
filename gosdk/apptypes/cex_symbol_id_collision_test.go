package apptypes

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// A real 31-bit collision on Binance, found by scanning "COLL<n>USDT" labels
// with an FNV-1a implementation written outside Go.
const (
	collidingLabelA            = "COLL122881USDT"
	collidingLabelB            = "COLL714990USDT"
	collidingSymbolID          = CEXSymbolID(0xfa5adec8)
	collisionNoiseLabelPattern = `[A-Z]{2,8}USDT`
)

func TestCEXSymbolIDCollisionsGoldenPairCollides(t *testing.T) {
	t.Parallel()

	require.Equal(t, collidingSymbolID, SymbolID(CEXExchangeIDBinance, collidingLabelA))
	require.Equal(t, collidingSymbolID, SymbolID(CEXExchangeIDBinance, collidingLabelB))

	got := CEXSymbolIDCollisions([]CEXMarketLabel{
		{ExchangeID: CEXExchangeIDBinance, Label: collidingLabelB},
		{ExchangeID: CEXExchangeIDBinance, Label: "BTCUSDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: "NEWUSDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: collidingLabelA},
	})
	require.Equal(t, []CEXSymbolIDCollision{{
		ExchangeID: CEXExchangeIDBinance,
		SymbolID:   collidingSymbolID,
		Labels:     []string{collidingLabelA, collidingLabelB},
	}}, got)
}

func TestCEXSymbolIDCollisionsIgnoresRepeatsAndOtherExchanges(t *testing.T) {
	t.Parallel()

	got := CEXSymbolIDCollisions([]CEXMarketLabel{
		// One label listed as spot and as perp is one market id, not a collision.
		{ExchangeID: CEXExchangeIDBinance, Label: "NEWUSDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: "NEWUSDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: "BTCUSDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: "BTCUSDT"},
		// The colliding pair on two different exchanges does not collide.
		{ExchangeID: CEXExchangeIDBinance, Label: collidingLabelA},
		{ExchangeID: CEXExchangeIDMEXC, Label: collidingLabelB},
	})
	require.Empty(t, got)
	require.Empty(t, CEXSymbolIDCollisions(nil))
}

// TestCEXSymbolIDCollisionsPropertyOrderIndependent: both colliding labels are
// marked in every arrival order, and no other label is.
func TestCEXSymbolIDCollisionsPropertyOrderIndependent(t *testing.T) {
	t.Parallel()

	want := []CEXSymbolIDCollision{{
		ExchangeID: CEXExchangeIDBinance,
		SymbolID:   collidingSymbolID,
		Labels:     []string{collidingLabelA, collidingLabelB},
	}}

	rapid.Check(t, func(rt *rapid.T) {
		noise := rapid.SliceOfN(
			rapid.StringMatching(collisionNoiseLabelPattern), 0, 32,
		).Draw(rt, "noise")

		markets := make([]CEXMarketLabel, 0, 2+len(noise))
		markets = append(markets,
			CEXMarketLabel{ExchangeID: CEXExchangeIDBinance, Label: collidingLabelA},
			CEXMarketLabel{ExchangeID: CEXExchangeIDBinance, Label: collidingLabelB},
		)

		for _, label := range noise {
			markets = append(markets, CEXMarketLabel{
				ExchangeID: CEXExchangeIDBinance,
				Label:      label,
			})
		}

		permuted := rapid.Permutation(markets).Draw(rt, "markets")
		require.Equal(rt, want, CEXSymbolIDCollisions(permuted))
	})
}

// TestCEXSymbolIDCollisionsSortedByExchangeThenID uses three more real
// collisions (two on Binance, one on MEXC) to pin the output order.
func TestCEXSymbolIDCollisionsSortedByExchangeThenID(t *testing.T) {
	t.Parallel()

	got := CEXSymbolIDCollisions([]CEXMarketLabel{
		{ExchangeID: CEXExchangeIDBinance, Label: "PAIR805080USDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: "PAIR805081USDT"},
		{ExchangeID: CEXExchangeIDMEXC, Label: "COLL816080USDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: "PAIR251191USDT"},
		{ExchangeID: CEXExchangeIDBinance, Label: "PAIR251190USDT"},
		{ExchangeID: CEXExchangeIDMEXC, Label: "COLL224191USDT"},
	})
	require.Equal(t, []CEXSymbolIDCollision{
		{
			ExchangeID: CEXExchangeIDMEXC,
			SymbolID:   0xcc686459,
			Labels:     []string{"COLL224191USDT", "COLL816080USDT"},
		},
		{
			ExchangeID: CEXExchangeIDBinance,
			SymbolID:   0x93ec60c2,
			Labels:     []string{"PAIR251190USDT", "PAIR805081USDT"},
		},
		{
			ExchangeID: CEXExchangeIDBinance,
			SymbolID:   0xf9cdb619,
			Labels:     []string{"PAIR251191USDT", "PAIR805080USDT"},
		},
	}, got)
}
