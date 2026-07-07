package apptypes

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
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

// OrderBookIDRegistry owns code-side CEX order-book label-to-ID mappings.
// Storage and event refs consume only numeric IDs; label lookups stay at
// config, wrapper, diagnostics, and fixture boundaries.
type OrderBookIDRegistry struct {
	mu sync.RWMutex

	symbolByID map[CEXSymbolID]string
	idBySymbol map[string]CEXSymbolID
}

// NewOrderBookIDRegistry constructs a registry with committed exchange and
// market-type mappings and an empty exact-symbol cache.
func NewOrderBookIDRegistry() *OrderBookIDRegistry {
	return &OrderBookIDRegistry{
		symbolByID: make(map[CEXSymbolID]string),
		idBySymbol: make(map[string]CEXSymbolID),
	}
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

// ResolveSymbolID maps an exact venue symbol label to a deterministic numeric
// symbol ID and records the reverse mapping in this process.
func (r *OrderBookIDRegistry) ResolveSymbolID(symbol string) (CEXSymbolID, error) {
	if symbol == "" {
		return 0, fmt.Errorf("empty cex symbol")
	}

	id := deterministicCEXSymbolID(symbol)
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing := r.symbolByID[id]; existing != "" && existing != symbol {
		return 0, fmt.Errorf("cex symbol id collision id=%d %q %q", id, existing, symbol)
	}

	r.symbolByID[id] = symbol
	r.idBySymbol[symbol] = id

	return id, nil
}

// SymbolLabel maps a process-known symbol ID to its diagnostic label.
func (r *OrderBookIDRegistry) SymbolLabel(id CEXSymbolID) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	label := r.symbolByID[id]

	return label, label != ""
}

func deterministicCEXSymbolID(symbol string) CEXSymbolID {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(symbol))
	id := hasher.Sum32()
	if id == 0 {
		return 1
	}

	return CEXSymbolID(id)
}
