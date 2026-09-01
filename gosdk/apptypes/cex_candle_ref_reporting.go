package apptypes

import (
	"fmt"
	"strconv"
	"strings"
)

// Reporting helpers over a batch of candle references, mirroring the trade
// helpers above them in this package and living here for the same reason: the
// SDK, pelacli, and the appchain must count the same way for the three stages
// of one investigation to agree.

// FormatCEXCandleRefMarkets renders one
// "exchange/marketType/symbol@timeframe=refs" entry per source, in first-seen
// order. Identifiers stay numeric on purpose — see FormatCEXMarketTradeRefMarkets.
func FormatCEXCandleRefMarkets(refs []CEXCandleBatchRef) string {
	if len(refs) == 0 {
		return ""
	}

	type sourceKey struct {
		exchange   CEXExchangeID
		marketType CEXMarketTypeID
		symbol     CEXSymbolID
		timeframe  uint64
	}

	counts := make(map[sourceKey]int, len(refs))
	order := make([]sourceKey, 0, len(refs))

	for _, ref := range refs {
		key := sourceKey{ref.ExchangeID, ref.MarketTypeID, ref.SymbolID, ref.TimeframeMS}
		if _, seen := counts[key]; !seen {
			order = append(order, key)
		}

		counts[key]++
	}

	parts := make([]string, 0, len(order))
	for _, key := range order {
		parts = append(parts, fmt.Sprintf(
			"%d/%d/%d@%d=%d",
			key.exchange, key.marketType, key.symbol, key.timeframe, counts[key],
		))
	}

	return strings.Join(parts, ",")
}

// CEXCandleRefLabels resolves one candle reference to metric label names.
// Unregistered identities keep their numeric ids — a market missing from the
// registry is precisely the case worth seeing.
func CEXCandleRefLabels(
	ref CEXCandleBatchRef,
) (exchange string, marketType string, symbol string, timeframe string) {
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

	return exchange, marketType, symbol, strconv.FormatUint(ref.TimeframeMS, 10)
}
