package cexidentity

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestStep159VRegistryEvolutionRejectsRelabelRemovalReuseAndVersionDrift(t *testing.T) {
	t.Parallel()

	base := step159VRegistryDocument()

	tests := map[string]func(apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON{
		"exchange relabel": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.Exchanges[0].Label = "renamed"

			return doc
		},
		"market removal": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.MarketTypes = doc.MarketTypes[:1]
			doc.Symbols = doc.Symbols[:1]

			return doc
		},
		"numeric id reuse": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.Symbols[0].Label = "ETHUSDT"

			return doc
		},
		"metadata mutation": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.Symbols[0].BaseAsset = "WBTC"

			return doc
		},
		"canonical mutation": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.Symbols[0].Canonical = false

			return doc
		},
		"version-only bump": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.Version++

			return doc
		},
		"addition skips version": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.Version += 2
			doc.Symbols = append(doc.Symbols, step159VAddedSymbol(2))

			return doc
		},
		"addition without version bump": func(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
			doc.Symbols = append(doc.Symbols, step159VAddedSymbol(2))

			return doc
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, ValidateEvolution(base, mutate(step159VCloneDocument(base))))
		})
	}

	unchanged := step159VCloneDocument(base)
	require.NoError(t, ValidateEvolution(base, unchanged))

	reordered := step159VCloneDocument(base)
	reordered.Exchanges[0], reordered.Exchanges[1] = reordered.Exchanges[1], reordered.Exchanges[0]
	reordered.MarketTypes[0], reordered.MarketTypes[1] = reordered.MarketTypes[1], reordered.MarketTypes[0]
	reordered.Symbols[0], reordered.Symbols[1] = reordered.Symbols[1], reordered.Symbols[0]
	require.NoError(t, ValidateEvolution(base, reordered))

	added := step159VCloneDocument(base)
	added.Version++
	added.Symbols = append(added.Symbols, step159VAddedSymbol(2))
	require.NoError(t, ValidateEvolution(base, added))

	require.NoError(t, ValidateInitial(base))
	invalidInitialVersion := step159VCloneDocument(base)
	invalidInitialVersion.Version = 2
	require.Error(t, ValidateInitial(invalidInitialVersion))
}

func TestStep159VRegistryEvolutionProperties(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		base := step159VRegistryDocument()
		current := step159VCloneDocument(base)
		additionID := apptypes.CEXSymbolID(
			rapid.Uint32Range(2, 10_000).Draw(rt, "addition_symbol_id"),
		)

		switch rapid.IntRange(0, 5).Draw(rt, "evolution_case") {
		case 0:
			current.Version++
			current.Symbols = append(current.Symbols, step159VAddedSymbol(additionID))
			require.NoError(rt, ValidateEvolution(base, current))
		case 1:
			current.Exchanges[0].Label += "-relabelled"
			require.Error(rt, ValidateEvolution(base, current))
		case 2:
			current.Symbols = current.Symbols[1:]
			require.Error(rt, ValidateEvolution(base, current))
		case 3:
			current.Symbols[0].QuoteAsset += "X"
			require.Error(rt, ValidateEvolution(base, current))
		case 4:
			current.Version += rapid.Uint32Range(1, 100).Draw(rt, "version_only_delta")
			require.Error(rt, ValidateEvolution(base, current))
		case 5:
			current.Version += 2
			current.Symbols = append(current.Symbols, step159VAddedSymbol(additionID))
			require.Error(rt, ValidateEvolution(base, current))
		default:
			rt.Fatalf("generated evolution case is outside the bounded domain")
		}
	})
}

func step159VRegistryDocument() apptypes.OrderBookIdentityJSON {
	return apptypes.OrderBookIdentityJSON{
		Version: 1,
		Exchanges: []apptypes.OrderBookExchangeJSON{
			{ID: 1, Label: "mexc"},
			{ID: 2, Label: "hyperliquid"},
		},
		MarketTypes: []apptypes.OrderBookMarketJSON{
			{ID: 1, Label: "spot"},
			{ID: 2, Label: "perp"},
		},
		Symbols: []apptypes.OrderBookSymbolJSON{
			{
				ExchangeID: 1, MarketTypeID: 1, SymbolID: 1,
				Label: "BTCUSDT", BaseAsset: "BTC", QuoteAsset: "USDT", Canonical: true,
			},
			{
				ExchangeID: 2, MarketTypeID: 2, SymbolID: 1,
				Label: "BTCUSDC", BaseAsset: "BTC", QuoteAsset: "USDC", Canonical: true,
			},
		},
	}
}

func step159VAddedSymbol(id apptypes.CEXSymbolID) apptypes.OrderBookSymbolJSON {
	return apptypes.OrderBookSymbolJSON{
		ExchangeID: 1, MarketTypeID: 1, SymbolID: id,
		Label: "ETHUSDT", BaseAsset: "ETH", QuoteAsset: "USDT", Canonical: true,
	}
}

func step159VCloneDocument(doc apptypes.OrderBookIdentityJSON) apptypes.OrderBookIdentityJSON {
	doc.Exchanges = append([]apptypes.OrderBookExchangeJSON(nil), doc.Exchanges...)
	doc.MarketTypes = append([]apptypes.OrderBookMarketJSON(nil), doc.MarketTypes...)
	doc.Symbols = append([]apptypes.OrderBookSymbolJSON(nil), doc.Symbols...)

	return doc
}
