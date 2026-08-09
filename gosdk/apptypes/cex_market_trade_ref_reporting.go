package apptypes

import (
	"fmt"
	"strconv"
	"strings"
)

// Reporting helpers over a batch of trade references.
//
// These existed three times, once per stage of the chain: the SDK rendered them
// when it decoded an event, pelacli when it drained the outbox, and the appchain
// when it adopted a batch. Three copies of one algorithm is three chances for
// the three stages of one investigation to disagree about what they are looking
// at - and comparing those stages against each other is the entire reason the
// lines exist. A market whose references enter the outbox and never reach
// adoption is only visible when both ends count the same way.
//
// They live in apptypes because that is where the reference type and the
// registry both live, and because all three stages already depend on it while
// depending on nothing of each other.

// FormatCEXMarketTradeRefMarkets renders one "exchange/marketType/symbol=refs"
// entry per market, in the order each market was first seen.
//
// Identifiers stay numeric on purpose: this is a per-batch line on a hot path,
// and a registry lookup per reference would put naming in front of throughput.
// Use CEXMarketTradeRefLabels where a human-readable name is worth the lookup,
// such as a metric label read long after the fact.
func FormatCEXMarketTradeRefMarkets(refs []CEXMarketTradeBatchRef) string {
	if len(refs) == 0 {
		return ""
	}

	type marketKey struct {
		exchange   CEXExchangeID
		marketType CEXMarketTypeID
		symbol     CEXSymbolID
	}

	counts := make(map[marketKey]int, len(refs))
	order := make([]marketKey, 0, len(refs))

	for _, ref := range refs {
		key := marketKey{ref.ExchangeID, ref.MarketTypeID, ref.SymbolID}
		if _, seen := counts[key]; !seen {
			order = append(order, key)
		}

		counts[key]++
	}

	parts := make([]string, 0, len(order))
	for _, key := range order {
		parts = append(parts, fmt.Sprintf(
			"%d/%d/%d=%d", key.exchange, key.marketType, key.symbol, counts[key],
		))
	}

	return strings.Join(parts, ",")
}

// CEXMarketTradeRefLabels resolves one reference to the names a metric is
// labelled with.
//
// An identity the registry cannot name keeps its numeric id rather than being
// dropped or blanked: a market missing from the registry is precisely the case
// worth seeing, and a label that disappears takes the series with it.
func CEXMarketTradeRefLabels(
	ref CEXMarketTradeBatchRef,
) (exchange string, marketType string, symbol string) {
	registry := DefaultOrderBookIDRegistry

	exchange, ok := registry.ExchangeLabel(ref.ExchangeID)
	if !ok {
		exchange = strconv.FormatUint(uint64(ref.ExchangeID), 10)
	}

	marketType, ok = registry.MarketTypeLabel(ref.MarketTypeID)
	if !ok {
		marketType = strconv.FormatUint(uint64(ref.MarketTypeID), 10)
	}

	symbol, ok = registry.SymbolLabel(ref.ExchangeID, ref.MarketTypeID, ref.SymbolID)
	if !ok {
		symbol = strconv.FormatUint(uint64(ref.SymbolID), 10)
	}

	return exchange, marketType, symbol
}
