package apptypes

import (
	"testing"
)

// TestResolveVenueAssetSymbolIDMapsHyperliquidAssetIDs proves the deterministic
// deploy-time mapping that PINE-BARS-10H3 needs. Authored strategies name a
// Hyperliquid market by its venue asset id, and the registry must resolve it to
// the shared symbol id without loading runtime venue metadata.
func TestResolveVenueAssetSymbolIDMapsHyperliquidAssetIDs(t *testing.T) {
	t.Parallel()

	registry := DefaultOrderBookIDRegistry

	exchangeID, err := registry.ResolveExchangeID("hyperliquid")
	if err != nil {
		t.Fatalf("resolve exchange: %v", err)
	}

	perpID, err := registry.ResolveMarketTypeID("perp")
	if err != nil {
		t.Fatalf("resolve market type: %v", err)
	}

	// Asset id zero is the real Hyperliquid BTC perp, which is why the schema
	// keeps the field nullable instead of treating zero as absent.
	symbolID, err := registry.ResolveVenueAssetSymbolID(exchangeID, perpID, "mainnet", 0)
	if err != nil {
		t.Fatalf("resolve venue asset 0: %v", err)
	}

	label, ok := registry.SymbolLabel(exchangeID, perpID, symbolID)
	if !ok {
		t.Fatalf("symbol %d has no label", symbolID)
	}

	if label != "BTCUSDC" {
		t.Fatalf("venue asset 0 resolved to %q, want BTCUSDC", label)
	}

	if _, err = registry.ResolveVenueAssetSymbolID(exchangeID, perpID, "mainnet", 4_000_000); err == nil {
		t.Fatal("an unknown venue asset id must be rejected")
	}

	// The same number means a different market per network, which is the whole
	// reason the mapping is network-scoped: asset id 4 is DYDX on mainnet and ETH
	// on testnet.
	mainnet4, err := registry.ResolveVenueAssetSymbolID(exchangeID, perpID, "mainnet", 4)
	if err != nil {
		t.Fatalf("resolve mainnet asset 4: %v", err)
	}

	testnet4, err := registry.ResolveVenueAssetSymbolID(exchangeID, perpID, "testnet", 4)
	if err != nil {
		t.Fatalf("resolve testnet asset 4: %v", err)
	}

	if mainnet4 == testnet4 {
		t.Fatal("mainnet and testnet asset id 4 must not resolve to one symbol")
	}

	// The same number also means a different market per product, so spot must not
	// silently answer with the perp market.
	spotID, err := registry.ResolveMarketTypeID("spot")
	if err != nil {
		t.Fatalf("resolve spot: %v", err)
	}

	spotSymbolID, err := registry.ResolveVenueAssetSymbolID(exchangeID, spotID, "mainnet", 0)
	if err == nil && spotSymbolID == symbolID {
		t.Fatal("spot and perp must not share one venue asset id mapping")
	}
}
