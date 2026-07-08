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

	errNilOrderBookIDRegistry          = errors.New("nil order-book id registry")
	errOrderBookIdentityVersionZero    = errors.New("order-book identity version is zero")
	errOrderBookIdentityNoExchanges    = errors.New("order-book identity has no exchanges")
	errOrderBookIdentityNoMarketTypes  = errors.New("order-book identity has no market types")
	errOrderBookIdentityNoSymbols      = errors.New("order-book identity has no symbols")
	errOrderBookExchangeIDZero         = errors.New("order-book identity exchange id is zero")
	errOrderBookExchangeLabelEmpty     = errors.New("order-book identity exchange label is empty")
	errDuplicateOrderBookExchangeID    = errors.New("duplicate order-book exchange id")
	errDuplicateOrderBookExchangeLabel = errors.New("duplicate order-book exchange label")
	errOrderBookMarketTypeIDZero       = errors.New("order-book identity market_type id is zero")
	errOrderBookMarketTypeLabelEmpty   = errors.New(
		"order-book identity market_type label is empty",
	)
	errDuplicateOrderBookMarketTypeID   = errors.New("duplicate order-book market_type id")
	errDuplicateOrderBookMarketTypeName = errors.New("duplicate order-book market_type label")
	errOrderBookSymbolIDFieldsZero      = errors.New(
		"order-book identity symbol id fields must be non-zero",
	)
	errOrderBookSymbolLabelEmpty      = errors.New("order-book identity symbol label is empty")
	errOrderBookSymbolUnknownExchange = errors.New(
		"order-book identity symbol references unknown exchange_id",
	)
	errOrderBookSymbolUnknownMarketType = errors.New(
		"order-book identity symbol references unknown market_type_id",
	)
	errDuplicateOrderBookSymbolLabel  = errors.New("duplicate order-book symbol label")
	errDuplicateOrderBookSymbolID     = errors.New("duplicate order-book symbol id")
	errConflictingOrderBookSymbolMeta = errors.New("conflicting order-book symbol metadata")
	errDuplicateOrderBookLegacySymbol = errors.New("duplicate order-book legacy candidate")
)

const (
	// DefaultOrderBookIDRegistry is a compatibility handle for the embedded
	// JSON-backed CEX order-book identity registry.
	DefaultOrderBookIDRegistry DefaultOrderBookIDRegistryHandle = 0
)

//go:embed cex_order_book_identity.json
var defaultOrderBookIdentityJSON []byte

// DefaultOrderBookIDRegistryHandle exposes the embedded registry through the
// historical package-level handle without making the registry cache itself a
// package global.
type DefaultOrderBookIDRegistryHandle uint8

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
	LoadOrderBookIdentity(ctx context.Context) (OrderBookIdentityJSON, error)
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
	metadataByLabel     map[cexLegacySymbolKey]cexSymbolMetadata
}

// NewOrderBookIDRegistry constructs the default embedded JSON-backed registry.
func NewOrderBookIDRegistry() *OrderBookIDRegistry {
	return mustNewDefaultOrderBookIDRegistry()
}

// ResolveExchangeID maps a committed exchange label to its numeric ID.
func (h DefaultOrderBookIDRegistryHandle) ResolveExchangeID(
	exchange string,
) (CEXExchangeID, error) {
	return h.registry().ResolveExchangeID(exchange)
}

// ExchangeLabel maps a committed exchange ID to its diagnostic label.
func (h DefaultOrderBookIDRegistryHandle) ExchangeLabel(id CEXExchangeID) (string, bool) {
	return h.registry().ExchangeLabel(id)
}

// ResolveMarketTypeID maps a committed market-type label to its numeric ID.
func (h DefaultOrderBookIDRegistryHandle) ResolveMarketTypeID(
	marketType string,
) (CEXMarketTypeID, error) {
	return h.registry().ResolveMarketTypeID(marketType)
}

// MarketTypeLabel maps a committed market-type ID to its diagnostic label.
func (h DefaultOrderBookIDRegistryHandle) MarketTypeLabel(id CEXMarketTypeID) (string, bool) {
	return h.registry().MarketTypeLabel(id)
}

// ResolveSymbolID maps an exact venue symbol label to the numeric symbol ID.
func (h DefaultOrderBookIDRegistryHandle) ResolveSymbolID(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	symbol string,
) (CEXSymbolID, error) {
	return h.registry().ResolveSymbolID(exchangeID, marketTypeID, symbol)
}

// ResolveLegacySymbolID maps a deprecated exchange+symbol boundary lookup to
// the symbol ID shared by every registered market with that label.
func (h DefaultOrderBookIDRegistryHandle) ResolveLegacySymbolID(
	exchangeID CEXExchangeID,
	symbol string,
) (CEXSymbolID, error) {
	return h.registry().ResolveLegacySymbolID(exchangeID, symbol)
}

// SymbolLabel maps a scoped symbol ID to its diagnostic label.
func (h DefaultOrderBookIDRegistryHandle) SymbolLabel(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	id CEXSymbolID,
) (string, bool) {
	return h.registry().SymbolLabel(exchangeID, marketTypeID, id)
}

// SymbolCandidates returns bounded market candidates for a deprecated
// exchange+symbol boundary lookup.
func (h DefaultOrderBookIDRegistryHandle) SymbolCandidates(
	exchangeID CEXExchangeID,
	symbol string,
) []CEXSymbolCandidate {
	return h.registry().SymbolCandidates(exchangeID, symbol)
}

func (DefaultOrderBookIDRegistryHandle) registry() *OrderBookIDRegistry {
	return NewOrderBookIDRegistry()
}

// NewOrderBookIDRegistryFromJSON validates a checked-in JSON identity document
// and returns an immutable lookup registry.
func NewOrderBookIDRegistryFromJSON(doc OrderBookIdentityJSON) (*OrderBookIDRegistry, error) {
	if doc.Version == 0 {
		return nil, errOrderBookIdentityVersionZero
	}

	if len(doc.Exchanges) == 0 {
		return nil, errOrderBookIdentityNoExchanges
	}

	if len(doc.MarketTypes) == 0 {
		return nil, errOrderBookIdentityNoMarketTypes
	}

	if len(doc.Symbols) == 0 {
		return nil, errOrderBookIdentityNoSymbols
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
		metadataByLabel:     make(map[cexLegacySymbolKey]cexSymbolMetadata, len(doc.Symbols)),
	}

	for _, exchange := range doc.Exchanges {
		label := normalizeCEXLabel(exchange.Label)
		if exchange.ID == 0 {
			return nil, errOrderBookExchangeIDZero
		}

		if label == "" {
			return nil, errOrderBookExchangeLabelEmpty
		}

		if _, exists := registry.exchangeLabelByID[exchange.ID]; exists {
			return nil, fmt.Errorf("%w: %d", errDuplicateOrderBookExchangeID, exchange.ID)
		}

		if _, exists := registry.exchangeIDByLabel[label]; exists {
			return nil, fmt.Errorf("%w: %q", errDuplicateOrderBookExchangeLabel, exchange.Label)
		}

		registry.exchangeIDByLabel[label] = exchange.ID
		registry.exchangeLabelByID[exchange.ID] = label
	}

	for _, market := range doc.MarketTypes {
		label := normalizeCEXLabel(market.Label)
		if market.ID == 0 {
			return nil, errOrderBookMarketTypeIDZero
		}

		if label == "" {
			return nil, errOrderBookMarketTypeLabelEmpty
		}

		if _, exists := registry.marketLabelByID[market.ID]; exists {
			return nil, fmt.Errorf("%w: %d", errDuplicateOrderBookMarketTypeID, market.ID)
		}

		if _, exists := registry.marketIDByLabel[label]; exists {
			return nil, fmt.Errorf("%w: %q", errDuplicateOrderBookMarketTypeName, market.Label)
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
		return 0, errNilOrderBookIDRegistry
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
		return 0, errNilOrderBookIDRegistry
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
		return 0, errNilOrderBookIDRegistry
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
		return 0, errNilOrderBookIDRegistry
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
		return errOrderBookSymbolIDFieldsZero
	}

	if label == "" {
		return errOrderBookSymbolLabelEmpty
	}

	if _, ok := r.exchangeLabelByID[symbol.ExchangeID]; !ok {
		return fmt.Errorf("%w: %d", errOrderBookSymbolUnknownExchange, symbol.ExchangeID)
	}

	if _, ok := r.marketLabelByID[symbol.MarketTypeID]; !ok {
		return fmt.Errorf("%w: %d", errOrderBookSymbolUnknownMarketType, symbol.MarketTypeID)
	}

	lookupKey := cexSymbolLookupKey{
		exchangeID:   symbol.ExchangeID,
		marketTypeID: symbol.MarketTypeID,
		label:        label,
	}
	if _, exists := r.symbolByLookup[lookupKey]; exists {
		return fmt.Errorf(
			"%w: exchange_id=%d market_type_id=%d label=%q",
			errDuplicateOrderBookSymbolLabel,
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
			"%w: exchange_id=%d market_type_id=%d symbol_id=%d previous=%q new=%q",
			errDuplicateOrderBookSymbolID,
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

	metadata := cexSymbolMetadata{
		baseAsset:  strings.TrimSpace(symbol.BaseAsset),
		quoteAsset: strings.TrimSpace(symbol.QuoteAsset),
	}
	if previous, exists := r.metadataByLabel[legacyKey]; exists && previous != metadata {
		return fmt.Errorf(
			"%w: exchange_id=%d label=%q",
			errConflictingOrderBookSymbolMeta,
			symbol.ExchangeID,
			label,
		)
	}

	r.metadataByLabel[legacyKey] = metadata

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
			"%w: exchange_id=%d market_type_id=%d symbol_id=%d label=%q",
			errDuplicateOrderBookLegacySymbol,
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

type cexSymbolMetadata struct {
	baseAsset  string
	quoteAsset string
}
