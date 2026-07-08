package apptypes

import (
	"fmt"
	"strings"
)

const (
	// CEXExchangeIDMEXC is the committed numeric exchange identity for MEXC.
	CEXExchangeIDMEXC CEXExchangeID = 1
	// CEXExchangeIDHyperliquid is the committed numeric exchange identity for Hyperliquid.
	CEXExchangeIDHyperliquid CEXExchangeID = 2

	// CEXMarketTypeIDSpot is the committed numeric market-type identity for spot.
	CEXMarketTypeIDSpot CEXMarketTypeID = 1
	// CEXMarketTypeIDPerp is the committed numeric market-type identity for perpetual futures.
	CEXMarketTypeIDPerp CEXMarketTypeID = 2
)

// OrderBookIDRegistry owns source-controlled CEX order-book label-to-ID mappings.
// Storage and event refs consume only numeric IDs; label lookups stay at config,
// wrapper, diagnostics, and fixture boundaries.
type OrderBookIDRegistry struct {
	symbolByScopedID map[cexSymbolIDKey]string
	idByScopedSymbol map[cexScopedSymbolKey]CEXSymbolID
}

// NewOrderBookIDRegistry constructs the committed exchange, market-type, and
// scoped symbol ID registry.
func NewOrderBookIDRegistry() *OrderBookIDRegistry {
	r := &OrderBookIDRegistry{
		symbolByScopedID: make(map[cexSymbolIDKey]string, len(defaultCEXSymbols)),
		idByScopedSymbol: make(map[cexScopedSymbolKey]CEXSymbolID, len(defaultCEXSymbols)),
	}

	for _, symbol := range defaultCEXSymbols {
		r.symbolByScopedID[cexSymbolIDKey{
			exchangeID:   symbol.exchangeID,
			marketTypeID: symbol.marketTypeID,
			symbolID:     symbol.symbolID,
		}] = symbol.label
		r.idByScopedSymbol[cexScopedSymbolKey{
			exchangeID:   symbol.exchangeID,
			marketTypeID: symbol.marketTypeID,
			label:        symbol.label,
		}] = symbol.symbolID
	}

	return r
}

// DefaultOrderBookIDRegistry is the shared process-local CEX order-book
// identity registry used by compatibility wrappers.
var DefaultOrderBookIDRegistry = NewOrderBookIDRegistry()

// ResolveExchangeID maps a committed exchange label to its numeric ID.
func (r *OrderBookIDRegistry) ResolveExchangeID(exchange string) (CEXExchangeID, error) {
	switch strings.ToLower(strings.TrimSpace(exchange)) {
	case "mexc":
		return CEXExchangeIDMEXC, nil
	case "hyperliquid":
		return CEXExchangeIDHyperliquid, nil
	default:
		return 0, fmt.Errorf("unknown cex exchange %q", exchange)
	}
}

// ExchangeLabel maps a committed exchange ID to its diagnostic label.
func (r *OrderBookIDRegistry) ExchangeLabel(id CEXExchangeID) (string, bool) {
	switch id {
	case CEXExchangeIDMEXC:
		return "mexc", true
	case CEXExchangeIDHyperliquid:
		return "hyperliquid", true
	default:
		return "", false
	}
}

// ResolveMarketTypeID maps a committed market-type label to its numeric ID.
func (r *OrderBookIDRegistry) ResolveMarketTypeID(marketType string) (CEXMarketTypeID, error) {
	switch strings.ToLower(strings.TrimSpace(marketType)) {
	case "spot":
		return CEXMarketTypeIDSpot, nil
	case "perp", "perps", "perpetual":
		return CEXMarketTypeIDPerp, nil
	default:
		return 0, fmt.Errorf("unknown cex market_type %q", marketType)
	}
}

// MarketTypeLabel maps a committed market-type ID to its diagnostic label.
func (r *OrderBookIDRegistry) MarketTypeLabel(id CEXMarketTypeID) (string, bool) {
	switch id {
	case CEXMarketTypeIDSpot:
		return "spot", true
	case CEXMarketTypeIDPerp:
		return "perp", true
	default:
		return "", false
	}
}

// ResolveSymbolID maps an exact source-controlled venue symbol label to the
// numeric symbol ID scoped by exchange and market type.
func (r *OrderBookIDRegistry) ResolveSymbolID(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	symbol string,
) (CEXSymbolID, error) {
	if r == nil {
		return 0, fmt.Errorf("nil cex order-book id registry")
	}

	label := strings.TrimSpace(symbol)
	if label == "" {
		return 0, fmt.Errorf("empty cex symbol")
	}

	id, ok := r.idByScopedSymbol[cexScopedSymbolKey{
		exchangeID:   exchangeID,
		marketTypeID: marketTypeID,
		label:        label,
	}]
	if !ok {
		return 0, fmt.Errorf(
			"unknown cex symbol exchange_id=%d market_type_id=%d symbol=%q",
			exchangeID,
			marketTypeID,
			symbol,
		)
	}

	return id, nil
}

// SymbolLabel maps a scoped source-controlled symbol ID to its diagnostic label.
func (r *OrderBookIDRegistry) SymbolLabel(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	id CEXSymbolID,
) (string, bool) {
	if r == nil {
		return "", false
	}

	label := r.symbolByScopedID[cexSymbolIDKey{
		exchangeID:   exchangeID,
		marketTypeID: marketTypeID,
		symbolID:     id,
	}]

	return label, label != ""
}

// CEXSymbolCandidate is one source-controlled market candidate for a legacy
// exchange+symbol boundary lookup.
type CEXSymbolCandidate struct {
	MarketTypeID CEXMarketTypeID
	SymbolID     CEXSymbolID
}

// SymbolCandidates returns bounded source-controlled market candidates for a
// deprecated exchange+symbol boundary lookup.
func (r *OrderBookIDRegistry) SymbolCandidates(
	exchangeID CEXExchangeID,
	symbol string,
) []CEXSymbolCandidate {
	if r == nil {
		return nil
	}

	label := strings.TrimSpace(symbol)
	if label == "" {
		return nil
	}

	out := make([]CEXSymbolCandidate, 0, 2)
	for _, marketTypeID := range []CEXMarketTypeID{
		CEXMarketTypeIDSpot,
		CEXMarketTypeIDPerp,
	} {
		id, ok := r.idByScopedSymbol[cexScopedSymbolKey{
			exchangeID:   exchangeID,
			marketTypeID: marketTypeID,
			label:        label,
		}]
		if ok {
			out = append(out, CEXSymbolCandidate{
				MarketTypeID: marketTypeID,
				SymbolID:     id,
			})
		}
	}

	return out
}

type cexScopedSymbolKey struct {
	exchangeID   CEXExchangeID
	marketTypeID CEXMarketTypeID
	label        string
}

type cexSymbolIDKey struct {
	exchangeID   CEXExchangeID
	marketTypeID CEXMarketTypeID
	symbolID     CEXSymbolID
}

type cexSymbolIdentity struct {
	exchangeID   CEXExchangeID
	marketTypeID CEXMarketTypeID
	symbolID     CEXSymbolID
	label        string
}

var defaultCEXSymbols = []cexSymbolIdentity{
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 1, "BTCUSDT"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 2, "ETHUSDT"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 3, "SPXUSDT"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 4, "NPCUSDT"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 5, "PEPEUSDT"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 6, "WBTCUSDT"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 7, "FAKEUSDC"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 8, "SOLUSDT"},
	{CEXExchangeIDMEXC, CEXMarketTypeIDPerp, 1, "SPXUSDT"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 1, "BTCUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 2, "ETHUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 3, "HYPEUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 4, "PEPEUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 5, "FUNUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 6, "AZTECUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 7, "DOGEUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 8, "UETHUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 9, "OTHERUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 10, "FAKEUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 11, "SPXUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 12, "ZZINSTRUMENTUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 1, "BTC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 2, "ETH"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 3, "HYPE"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 4, "PEPE"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 5, "SPX"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 6, "BTCUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 7, "ETHUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 8, "PEPEUSDC"},
	{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 9, "FAKEUSDC"},
}
