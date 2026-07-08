package apptypes

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
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

	cexExchangeLabelMEXC        = "mexc"
	cexExchangeLabelHyperliquid = "hyperliquid"
	cexMarketTypeLabelSpot      = "spot"
	cexMarketTypeLabelPerp      = "perp"
)

var (
	errEmptyCEXSymbol       = errors.New("empty cex symbol")
	errUnknownCEXExchange   = errors.New("unknown cex exchange")
	errUnknownCEXMarketType = errors.New("unknown cex market_type")
	errUnknownCEXSymbol     = errors.New("unknown cex symbol")
	errAmbiguousCEXSymbolID = errors.New("ambiguous cex symbol id")

	// DefaultOrderBookIDRegistry is the immutable JSON-backed CEX order-book
	// identity registry used by storage and compatibility wrappers.
	DefaultOrderBookIDRegistry = mustNewDefaultOrderBookIDRegistry()
)

//go:embed cex_order_book_identity.json
var defaultOrderBookIdentityJSON []byte

// OrderBookIdentityJSON is the checked-in order-book identity snapshot format.
// The SDK embeds this JSON so adding symbols changes data, not Go routing code.
type OrderBookIdentityJSON struct {
	Version     uint32                  `json:"version"`
	Exchanges   []OrderBookExchangeJSON `json:"exchanges"`
	MarketTypes []OrderBookMarketJSON   `json:"market_types"`
	Symbols     []OrderBookSymbolJSON   `json:"symbols"`
}

// OrderBookExchangeJSON describes one exchange label and numeric identity.
type OrderBookExchangeJSON struct {
	ID    CEXExchangeID `json:"id"`
	Label string        `json:"label"`
}

// OrderBookMarketJSON describes one market-type label and numeric identity.
type OrderBookMarketJSON struct {
	ID    CEXMarketTypeID `json:"id"`
	Label string          `json:"label"`
}

// OrderBookSymbolJSON describes one symbol label scoped by exchange and market.
type OrderBookSymbolJSON struct {
	ExchangeID   CEXExchangeID   `json:"exchange_id"`
	MarketTypeID CEXMarketTypeID `json:"market_type_id"`
	SymbolID     CEXSymbolID     `json:"symbol_id"`
	Label        string          `json:"label"`
	BaseAsset    string          `json:"base_asset,omitempty"`
	QuoteAsset   string          `json:"quote_asset,omitempty"`
	Canonical    bool            `json:"canonical,omitempty"`
}

// OrderBookIdentityJSONLoader supplies a checked-in or approved runtime JSON
// identity snapshot to the registry constructor.
type OrderBookIdentityJSONLoader interface {
	LoadOrderBookIdentity(context.Context) (OrderBookIdentityJSON, error)
}

// OrderBookIDRegistry owns JSON-backed CEX order-book label-to-ID mappings.
// Storage and event refs consume only numeric IDs; label lookups stay at config,
// wrapper, diagnostics, and fixture boundaries.
type OrderBookIDRegistry struct {
	exchangeIDByLabel   map[string]CEXExchangeID
	exchangeLabelByID   map[CEXExchangeID]string
	marketIDByLabel     map[string]CEXMarketTypeID
	marketLabelByID     map[CEXMarketTypeID]string
	symbolByLookup      map[cexSymbolLookupKey]cexSymbolRecord
	symbolLabelByID     map[cexSymbolIDKey]string
	legacyCandidates    map[cexLegacySymbolKey][]CEXSymbolCandidate
	legacyCandidateSeen map[cexLegacyCandidateKey]struct{}
}

// NewOrderBookIDRegistry constructs the default embedded JSON-backed registry.
func NewOrderBookIDRegistry() *OrderBookIDRegistry {
	return DefaultOrderBookIDRegistry
}

// NewOrderBookIDRegistryFromJSON validates a checked-in JSON identity document
// and returns an immutable lookup registry.
func NewOrderBookIDRegistryFromJSON(doc OrderBookIdentityJSON) (*OrderBookIDRegistry, error) {
	if doc.Version == 0 {
		return nil, errors.New("order-book identity version is zero")
	}
	if len(doc.Exchanges) == 0 {
		return nil, errors.New("order-book identity has no exchanges")
	}
	if len(doc.MarketTypes) == 0 {
		return nil, errors.New("order-book identity has no market types")
	}
	if len(doc.Symbols) == 0 {
		return nil, errors.New("order-book identity has no symbols")
	}

	registry := &OrderBookIDRegistry{
		exchangeIDByLabel:   make(map[string]CEXExchangeID, len(doc.Exchanges)),
		exchangeLabelByID:   make(map[CEXExchangeID]string, len(doc.Exchanges)),
		marketIDByLabel:     make(map[string]CEXMarketTypeID, len(doc.MarketTypes)),
		marketLabelByID:     make(map[CEXMarketTypeID]string, len(doc.MarketTypes)),
		symbolByLookup:      make(map[cexSymbolLookupKey]cexSymbolRecord, len(doc.Symbols)),
		symbolLabelByID:     make(map[cexSymbolIDKey]string, len(doc.Symbols)),
		legacyCandidates:    make(map[cexLegacySymbolKey][]CEXSymbolCandidate, len(doc.Symbols)),
		legacyCandidateSeen: make(map[cexLegacyCandidateKey]struct{}, len(doc.Symbols)),
	}

	for _, exchange := range doc.Exchanges {
		label := normalizeCEXLabel(exchange.Label)
		if exchange.ID == 0 {
			return nil, errors.New("order-book identity exchange id is zero")
		}
		if label == "" {
			return nil, errors.New("order-book identity exchange label is empty")
		}
		if _, exists := registry.exchangeLabelByID[exchange.ID]; exists {
			return nil, fmt.Errorf("duplicate order-book exchange id: %d", exchange.ID)
		}
		if _, exists := registry.exchangeIDByLabel[label]; exists {
			return nil, fmt.Errorf("duplicate order-book exchange label: %q", exchange.Label)
		}

		registry.exchangeIDByLabel[label] = exchange.ID
		registry.exchangeLabelByID[exchange.ID] = label
	}

	for _, market := range doc.MarketTypes {
		label := normalizeCEXLabel(market.Label)
		if market.ID == 0 {
			return nil, errors.New("order-book identity market_type id is zero")
		}
		if label == "" {
			return nil, errors.New("order-book identity market_type label is empty")
		}
		if _, exists := registry.marketLabelByID[market.ID]; exists {
			return nil, fmt.Errorf("duplicate order-book market_type id: %d", market.ID)
		}
		if _, exists := registry.marketIDByLabel[label]; exists {
			return nil, fmt.Errorf("duplicate order-book market_type label: %q", market.Label)
		}

		registry.marketIDByLabel[label] = market.ID
		registry.marketLabelByID[market.ID] = label
	}

	for _, symbol := range doc.Symbols {
		if err := registry.addSymbol(symbol); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

// ResolveExchangeID maps a committed exchange label to its numeric ID.
func (r *OrderBookIDRegistry) ResolveExchangeID(exchange string) (CEXExchangeID, error) {
	if r == nil {
		return 0, errors.New("nil order-book id registry")
	}
	id, ok := r.exchangeIDByLabel[normalizeCEXLabel(exchange)]
	if !ok {
		return 0, fmt.Errorf("%w: %q", errUnknownCEXExchange, exchange)
	}

	return id, nil
}

// ExchangeLabel maps a committed exchange ID to its diagnostic label.
func (r *OrderBookIDRegistry) ExchangeLabel(id CEXExchangeID) (string, bool) {
	if r == nil {
		return "", false
	}
	label, ok := r.exchangeLabelByID[id]

	return label, ok
}

// ResolveMarketTypeID maps a committed market-type label to its numeric ID.
func (r *OrderBookIDRegistry) ResolveMarketTypeID(marketType string) (CEXMarketTypeID, error) {
	if r == nil {
		return 0, errors.New("nil order-book id registry")
	}
	label := normalizeCEXLabel(marketType)
	if label == "perps" || label == "perpetual" {
		label = cexMarketTypeLabelPerp
	}

	id, ok := r.marketIDByLabel[label]
	if !ok {
		return 0, fmt.Errorf("%w: %q", errUnknownCEXMarketType, marketType)
	}

	return id, nil
}

// MarketTypeLabel maps a committed market-type ID to its diagnostic label.
func (r *OrderBookIDRegistry) MarketTypeLabel(id CEXMarketTypeID) (string, bool) {
	if r == nil {
		return "", false
	}
	label, ok := r.marketLabelByID[id]

	return label, ok
}

// ResolveSymbolID maps an exact JSON-backed venue symbol label to the numeric
// symbol ID scoped by exchange and market type.
func (r *OrderBookIDRegistry) ResolveSymbolID(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	symbol string,
) (CEXSymbolID, error) {
	if r == nil {
		return 0, errors.New("nil order-book id registry")
	}
	label := strings.TrimSpace(symbol)
	if label == "" {
		return 0, errEmptyCEXSymbol
	}

	record, ok := r.symbolByLookup[cexSymbolLookupKey{
		exchangeID:   exchangeID,
		marketTypeID: marketTypeID,
		label:        label,
	}]
	if !ok {
		return 0, fmt.Errorf(
			"%w: exchange_id=%d market_type_id=%d symbol=%q",
			errUnknownCEXSymbol,
			exchangeID,
			marketTypeID,
			symbol,
		)
	}

	return record.symbolID, nil
}

// ResolveLegacySymbolID maps a deprecated exchange+symbol boundary lookup to
// the symbol ID shared by every registered market with that label. Storage still
// keys exact books by exchange ID, market-type ID, and symbol ID; this helper
// exists only so compatibility readers can run the bounded DB ambiguity check
// over exchange ID and symbol ID before choosing a market type.
func (r *OrderBookIDRegistry) ResolveLegacySymbolID(
	exchangeID CEXExchangeID,
	symbol string,
) (CEXSymbolID, error) {
	if r == nil {
		return 0, errors.New("nil order-book id registry")
	}
	label := strings.TrimSpace(symbol)
	if label == "" {
		return 0, errEmptyCEXSymbol
	}

	candidates := r.legacyCandidates[cexLegacySymbolKey{
		exchangeID: exchangeID,
		label:      label,
	}]
	if len(candidates) == 0 {
		return 0, fmt.Errorf(
			"%w: exchange_id=%d symbol=%q",
			errUnknownCEXSymbol,
			exchangeID,
			symbol,
		)
	}

	resolved := candidates[0].SymbolID
	for _, candidate := range candidates[1:] {
		if candidate.SymbolID != resolved {
			return 0, fmt.Errorf(
				"%w: exchange_id=%d symbol=%q",
				errAmbiguousCEXSymbolID,
				exchangeID,
				symbol,
			)
		}
	}

	return resolved, nil
}

// SymbolLabel maps a scoped JSON-backed symbol ID to its diagnostic label.
func (r *OrderBookIDRegistry) SymbolLabel(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	id CEXSymbolID,
) (string, bool) {
	if r == nil {
		return "", false
	}
	label, ok := r.symbolLabelByID[cexSymbolIDKey{
		exchangeID:   exchangeID,
		marketTypeID: marketTypeID,
		symbolID:     id,
	}]

	return label, ok
}

// CEXSymbolCandidate is one JSON-backed market candidate for a legacy
// exchange+symbol boundary lookup.
type CEXSymbolCandidate struct {
	MarketTypeID CEXMarketTypeID
	SymbolID     CEXSymbolID
}

// SymbolCandidates returns bounded JSON-backed market candidates for a
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

	candidates := r.legacyCandidates[cexLegacySymbolKey{
		exchangeID: exchangeID,
		label:      label,
	}]
	if len(candidates) == 0 {
		return nil
	}

	out := make([]CEXSymbolCandidate, len(candidates))
	copy(out, candidates)

	return out
}

func mustNewDefaultOrderBookIDRegistry() *OrderBookIDRegistry {
	var doc OrderBookIdentityJSON
	if err := json.Unmarshal(defaultOrderBookIdentityJSON, &doc); err != nil {
		panic(fmt.Sprintf("decode embedded cex order-book identity: %v", err))
	}

	registry, err := NewOrderBookIDRegistryFromJSON(doc)
	if err != nil {
		panic(fmt.Sprintf("validate embedded cex order-book identity: %v", err))
	}

	return registry
}

func (r *OrderBookIDRegistry) addSymbol(symbol OrderBookSymbolJSON) error {
	label := strings.TrimSpace(symbol.Label)
	if symbol.ExchangeID == 0 || symbol.MarketTypeID == 0 || symbol.SymbolID == 0 {
		return errors.New("order-book identity symbol id fields must be non-zero")
	}
	if label == "" {
		return errors.New("order-book identity symbol label is empty")
	}
	if _, ok := r.exchangeLabelByID[symbol.ExchangeID]; !ok {
		return fmt.Errorf("order-book identity symbol references unknown exchange_id: %d", symbol.ExchangeID)
	}
	if _, ok := r.marketLabelByID[symbol.MarketTypeID]; !ok {
		return fmt.Errorf("order-book identity symbol references unknown market_type_id: %d", symbol.MarketTypeID)
	}

	lookupKey := cexSymbolLookupKey{
		exchangeID:   symbol.ExchangeID,
		marketTypeID: symbol.MarketTypeID,
		label:        label,
	}
	if _, exists := r.symbolByLookup[lookupKey]; exists {
		return fmt.Errorf(
			"duplicate order-book symbol label: exchange_id=%d market_type_id=%d label=%q",
			symbol.ExchangeID,
			symbol.MarketTypeID,
			label,
		)
	}

	idKey := cexSymbolIDKey{
		exchangeID:   symbol.ExchangeID,
		marketTypeID: symbol.MarketTypeID,
		symbolID:     symbol.SymbolID,
	}
	if previousLabel, exists := r.symbolLabelByID[idKey]; exists {
		return fmt.Errorf(
			"duplicate order-book symbol id: exchange_id=%d market_type_id=%d symbol_id=%d previous=%q new=%q",
			symbol.ExchangeID,
			symbol.MarketTypeID,
			symbol.SymbolID,
			previousLabel,
			label,
		)
	}

	record := cexSymbolRecord{
		symbolID:   symbol.SymbolID,
		baseAsset:  strings.TrimSpace(symbol.BaseAsset),
		quoteAsset: strings.TrimSpace(symbol.QuoteAsset),
	}
	r.symbolByLookup[lookupKey] = record
	r.symbolLabelByID[idKey] = label

	legacyKey := cexLegacySymbolKey{
		exchangeID: symbol.ExchangeID,
		label:      label,
	}
	candidate := CEXSymbolCandidate{
		MarketTypeID: symbol.MarketTypeID,
		SymbolID:     symbol.SymbolID,
	}
	candidateKey := cexLegacyCandidateKey{
		exchangeID:   symbol.ExchangeID,
		label:        label,
		marketTypeID: symbol.MarketTypeID,
		symbolID:     symbol.SymbolID,
	}
	if _, exists := r.legacyCandidateSeen[candidateKey]; exists {
		return fmt.Errorf(
			"duplicate order-book legacy candidate: exchange_id=%d market_type_id=%d symbol_id=%d label=%q",
			symbol.ExchangeID,
			symbol.MarketTypeID,
			symbol.SymbolID,
			label,
		)
	}
	r.legacyCandidateSeen[candidateKey] = struct{}{}
	r.legacyCandidates[legacyKey] = append(r.legacyCandidates[legacyKey], candidate)

	return nil
}

func normalizeCEXLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

type cexSymbolLookupKey struct {
	exchangeID   CEXExchangeID
	marketTypeID CEXMarketTypeID
	label        string
}

type cexSymbolIDKey struct {
	exchangeID   CEXExchangeID
	marketTypeID CEXMarketTypeID
	symbolID     CEXSymbolID
}

type cexLegacySymbolKey struct {
	exchangeID CEXExchangeID
	label      string
}

type cexLegacyCandidateKey struct {
	exchangeID   CEXExchangeID
	label        string
	marketTypeID CEXMarketTypeID
	symbolID     CEXSymbolID
}

type cexSymbolRecord struct {
	symbolID   CEXSymbolID
	baseAsset  string
	quoteAsset string
}
