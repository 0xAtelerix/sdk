package apptypes

import (
	"cmp"
	"slices"
)

// CEXMarketLabel is one reported market label on an exchange. The market type
// is not part of it because it is not part of the id.
type CEXMarketLabel struct {
	ExchangeID CEXExchangeID
	Label      string
}

// CEXSymbolIDCollision is one id that two or more distinct labels on one
// exchange share. Every market using it is unavailable until an override row
// gives one label another id.
type CEXSymbolIDCollision struct {
	ExchangeID CEXExchangeID
	SymbolID   CEXSymbolID
	Labels     []string
}

// CEXSymbolIDCollisions returns every id shared by two or more distinct labels
// on one exchange, sorted by exchange and id, with labels sorted. The result
// depends only on the set of markets, not their order or repeats. Frozen ids
// cannot collide with each other (the table rejects it) or with derived ids
// (frozen ids are below CEXDerivedSymbolIDBase), so in practice only derived
// ids are reported.
func CEXSymbolIDCollisions(markets []CEXMarketLabel) []CEXSymbolIDCollision {
	type idKey struct {
		exchangeID CEXExchangeID
		symbolID   CEXSymbolID
	}

	labelsByID := make(map[idKey][]string, len(markets))

	for _, market := range markets {
		key := idKey{
			exchangeID: market.ExchangeID,
			symbolID:   SymbolID(market.ExchangeID, market.Label),
		}
		if !slices.Contains(labelsByID[key], market.Label) {
			labelsByID[key] = append(labelsByID[key], market.Label)
		}
	}

	var collisions []CEXSymbolIDCollision

	for key, labels := range labelsByID {
		if len(labels) < 2 {
			continue
		}

		slices.Sort(labels)
		collisions = append(collisions, CEXSymbolIDCollision{
			ExchangeID: key.exchangeID,
			SymbolID:   key.symbolID,
			Labels:     labels,
		})
	}

	slices.SortFunc(collisions, func(a, b CEXSymbolIDCollision) int {
		return cmp.Or(
			cmp.Compare(a.ExchangeID, b.ExchangeID),
			cmp.Compare(a.SymbolID, b.SymbolID),
		)
	})

	return collisions
}
