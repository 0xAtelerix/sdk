package apptypes

import (
	"testing"
)

// Hyperliquid spells its scaled tickers with a lowercase prefix — kPEPE is
// 1000 PEPE — and the registry stores that exact spelling. Callers that carry a
// symbol through an uppercasing identity layer then arrive with KPEPEUSDC and
// miss, which is silent: the order-book key construction fails, the mark cannot
// resolve, and the position marks at its own entry price with zero unrealized
// PnL forever.
//
// Six labels in the registry contain a lowercase letter — kBONKUSDC,
// kFLOKIUSDC, kLUNCUSDC, kNEIROUSDC, kPEPEUSDC, kSHIBUSDC — and no two labels
// anywhere collide case-insensitively, so resolving by folded label is
// unambiguous.
func TestSymbolLookupIsCaseInsensitive(t *testing.T) {
	registry := DefaultOrderBookIDRegistry

	exchangeID, err := registry.ResolveExchangeID("hyperliquid")
	if err != nil {
		t.Fatalf("resolve hyperliquid: %v", err)
	}

	marketTypeID, err := registry.ResolveMarketTypeID("perp")
	if err != nil {
		t.Fatalf("resolve perp: %v", err)
	}

	canonical, err := registry.ResolveSymbolID(exchangeID, marketTypeID, "kPEPEUSDC")
	if err != nil {
		t.Fatalf("canonical kPEPEUSDC must resolve: %v", err)
	}

	for _, spelling := range []string{"KPEPEUSDC", "kpepeusdc", "kPePeUsDc", "  kPEPEUSDC  "} {
		t.Run(spelling, func(t *testing.T) {
			got, resolveErr := registry.ResolveSymbolID(exchangeID, marketTypeID, spelling)
			if resolveErr != nil {
				t.Fatalf("ResolveSymbolID(%q) = %v, want it to resolve like the canonical spelling", spelling, resolveErr)
			}

			if got != canonical {
				t.Fatalf("ResolveSymbolID(%q) = %d, want %d", spelling, got, canonical)
			}

			if candidates := registry.SymbolCandidates(exchangeID, spelling); len(candidates) == 0 {
				t.Fatalf("SymbolCandidates(%q) returned none", spelling)
			}
		})
	}

	// Folding the lookup must not change what the registry reports back: the
	// venue's own spelling is what logs, order-book keys and UI must show.
	t.Run("canonical spelling is preserved", func(t *testing.T) {
		label, ok := registry.SymbolLabel(exchangeID, marketTypeID, canonical)
		if !ok {
			t.Fatal("SymbolLabel missing for a symbol that resolves")
		}

		if label != "kPEPEUSDC" {
			t.Fatalf("SymbolLabel = %q, want the venue spelling kPEPEUSDC", label)
		}
	})

	// An uppercase-only market must keep working exactly as before.
	t.Run("uppercase markets are unaffected", func(t *testing.T) {
		want, ferr := registry.ResolveSymbolID(exchangeID, marketTypeID, "FARTCOINUSDC")
		if ferr != nil {
			t.Fatalf("FARTCOINUSDC must resolve: %v", ferr)
		}

		got, gerr := registry.ResolveSymbolID(exchangeID, marketTypeID, "fartcoinusdc")
		if gerr != nil {
			t.Fatalf("lowercase FARTCOINUSDC: %v", gerr)
		}

		if got != want {
			t.Fatalf("case-folded FARTCOINUSDC = %d, want %d", got, want)
		}
	})
}
