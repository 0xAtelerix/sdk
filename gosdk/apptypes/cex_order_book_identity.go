package apptypes

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const (
	// CEXExchangeIDMEXC is the committed numeric exchange identity for MEXC.
	CEXExchangeIDMEXC CEXExchangeID = 1
	// CEXExchangeIDHyperliquid is the committed numeric exchange identity for Hyperliquid.
	CEXExchangeIDHyperliquid CEXExchangeID = 2
	// CEXExchangeIDBinance is the committed numeric exchange identity for Binance.
	CEXExchangeIDBinance CEXExchangeID = 3

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
	errInconsistentLegacySymbolID     = errors.New(
		"order-book identity legacy symbol id diverges across market types",
	)
	errNilOrderBookIdentityLoader = errors.New("nil order-book identity loader")
)

const (
	// DefaultOrderBookIDRegistry is a compatibility handle for the embedded
	// JSON-backed CEX order-book identity registry.
	DefaultOrderBookIDRegistry DefaultOrderBookIDRegistryHandle = 0
)

//go:embed cex_order_book_identity.json
var defaultOrderBookIdentityJSON []byte

// DefaultOrderBookIDRegistryHandle exposes the embedded registry through the
// historical package-level handle, resolving against a single memoized registry
// instance rather than rebuilding one per call.
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
	exchangeIDByLabel map[string]CEXExchangeID
	exchangeLabelByID map[CEXExchangeID]string
	marketIDByLabel   map[string]CEXMarketTypeID
	marketLabelByID   map[CEXMarketTypeID]string
	symbolByLookup    map[cexSymbolLookupKey]CEXSymbolID
	symbolLabelByID   map[cexSymbolIDKey]string
	legacyCandidates  map[cexLegacySymbolKey][]CEXSymbolCandidate
	metadataByLabel   map[cexLegacySymbolKey]cexSymbolMetadata
}

// NewOrderBookIDRegistry constructs the default embedded JSON-backed registry.
func NewOrderBookIDRegistry() (*OrderBookIDRegistry, error) {
	return NewOrderBookIDRegistryWithContext(context.Background())
}

// NewOrderBookIDRegistryWithContext constructs the default embedded
// JSON-backed registry while preserving caller cancellation.
func NewOrderBookIDRegistryWithContext(ctx context.Context) (*OrderBookIDRegistry, error) {
	return NewOrderBookIDRegistryFromLoader(ctx, embeddedOrderBookIdentityJSONLoader{})
}

// ResolveExchangeID maps a committed exchange label to its numeric ID.
func (h DefaultOrderBookIDRegistryHandle) ResolveExchangeID(
	exchange string,
) (CEXExchangeID, error) {
	registry, err := h.registry()
	if err != nil {
		return 0, err
	}

	return registry.ResolveExchangeID(exchange)
}

// ExchangeLabel maps a committed exchange ID to its diagnostic label.
func (h DefaultOrderBookIDRegistryHandle) ExchangeLabel(id CEXExchangeID) (string, bool) {
	registry, err := h.registry()
	if err != nil {
		return "", false
	}

	return registry.ExchangeLabel(id)
}

// ResolveMarketTypeID maps a committed market-type label to its numeric ID.
func (h DefaultOrderBookIDRegistryHandle) ResolveMarketTypeID(
	marketType string,
) (CEXMarketTypeID, error) {
	registry, err := h.registry()
	if err != nil {
		return 0, err
	}

	return registry.ResolveMarketTypeID(marketType)
}

// MarketTypeLabel maps a committed market-type ID to its diagnostic label.
func (h DefaultOrderBookIDRegistryHandle) MarketTypeLabel(id CEXMarketTypeID) (string, bool) {
	registry, err := h.registry()
	if err != nil {
		return "", false
	}

	return registry.MarketTypeLabel(id)
}

// ResolveSymbolID maps an exact venue symbol label to the numeric symbol ID.
func (h DefaultOrderBookIDRegistryHandle) ResolveSymbolID(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	symbol string,
) (CEXSymbolID, error) {
	registry, err := h.registry()
	if err != nil {
		return 0, err
	}

	return registry.ResolveSymbolID(exchangeID, marketTypeID, symbol)
}

// ResolveLegacySymbolID maps a deprecated exchange+symbol boundary lookup to
// the symbol ID shared by every registered market with that label.
func (h DefaultOrderBookIDRegistryHandle) ResolveLegacySymbolID(
	exchangeID CEXExchangeID,
	symbol string,
) (CEXSymbolID, error) {
	registry, err := h.registry()
	if err != nil {
		return 0, err
	}

	return registry.ResolveLegacySymbolID(exchangeID, symbol)
}

// SymbolLabel maps a scoped symbol ID to its diagnostic label.
func (h DefaultOrderBookIDRegistryHandle) SymbolLabel(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	id CEXSymbolID,
) (string, bool) {
	registry, err := h.registry()
	if err != nil {
		return "", false
	}

	return registry.SymbolLabel(exchangeID, marketTypeID, id)
}

// SymbolAssets returns the committed base and quote assets for an exact
// order-book identity, reporting false when the identity is unregistered.
func (h DefaultOrderBookIDRegistryHandle) SymbolAssets(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	id CEXSymbolID,
) (baseAsset, quoteAsset string, ok bool) {
	registry, err := h.registry()
	if err != nil {
		return "", "", false
	}

	return registry.SymbolAssets(exchangeID, marketTypeID, id)
}

// SymbolCandidates returns bounded market candidates for a deprecated
// exchange+symbol boundary lookup.
func (h DefaultOrderBookIDRegistryHandle) SymbolCandidates(
	exchangeID CEXExchangeID,
	symbol string,
) []CEXSymbolCandidate {
	registry, err := h.registry()
	if err != nil {
		return nil
	}

	return registry.SymbolCandidates(exchangeID, symbol)
}

// defaultOrderBookRegistry memoizes the embedded registry. It is immutable after
// construction (its maps are only read), so a single shared instance keeps the
// handle cheap for hot callers instead of re-parsing the embedded JSON per call.
//
//nolint:gochecknoglobals // immutable, lazily-built read-only registry cache
var defaultOrderBookRegistry = sync.OnceValues(NewOrderBookIDRegistry)

func (DefaultOrderBookIDRegistryHandle) registry() (*OrderBookIDRegistry, error) {
	return defaultOrderBookRegistry()
}

type embeddedOrderBookIdentityJSONLoader struct{}

func (embeddedOrderBookIdentityJSONLoader) LoadOrderBookIdentity(
	ctx context.Context,
) (OrderBookIdentityJSON, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return OrderBookIdentityJSON{}, err
		}
	}

	var doc OrderBookIdentityJSON
	if err := json.Unmarshal(defaultOrderBookIdentityJSON, &doc); err != nil {
		return OrderBookIdentityJSON{}, fmt.Errorf(
			"decode embedded cex order-book identity: %w",
			err,
		)
	}

	return doc, nil
}

// NewOrderBookIDRegistryFromLoader loads and validates a JSON-backed CEX
// order-book identity document without installing hidden globals or panics.
func NewOrderBookIDRegistryFromLoader(
	ctx context.Context,
	loader OrderBookIdentityJSONLoader,
) (*OrderBookIDRegistry, error) {
	if loader == nil {
		return nil, errNilOrderBookIdentityLoader
	}

	doc, err := loader.LoadOrderBookIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("load cex order-book identity: %w", err)
	}

	registry, err := NewOrderBookIDRegistryFromJSON(doc)
	if err != nil {
		return nil, fmt.Errorf("validate cex order-book identity: %w", err)
	}

	return registry, nil
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
		exchangeIDByLabel: make(map[string]CEXExchangeID, len(doc.Exchanges)),
		exchangeLabelByID: make(map[CEXExchangeID]string, len(doc.Exchanges)),
		marketIDByLabel:   make(map[string]CEXMarketTypeID, len(doc.MarketTypes)),
		marketLabelByID:   make(map[CEXMarketTypeID]string, len(doc.MarketTypes)),
		symbolByLookup:    make(map[cexSymbolLookupKey]CEXSymbolID, len(doc.Symbols)),
		symbolLabelByID:   make(map[cexSymbolIDKey]string, len(doc.Symbols)),
		legacyCandidates:  make(map[cexLegacySymbolKey][]CEXSymbolCandidate, len(doc.Symbols)),
		metadataByLabel:   make(map[cexLegacySymbolKey]cexSymbolMetadata, len(doc.Symbols)),
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

	symbolID, ok := r.symbolByLookup[cexSymbolLookupKey{
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

	return symbolID, nil
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

// SymbolAssets returns the committed base and quote assets for an exact
// order-book identity. Owner: the identity registry. Consumers resolve asset
// units from the identity the storage key was already validated against instead
// of inferring them from symbol text, which is required for venues that list the
// same symbol on several products. It reports false for an unregistered identity
// or a row missing either asset, so callers fail closed rather than fall back.
func (r *OrderBookIDRegistry) SymbolAssets(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	id CEXSymbolID,
) (baseAsset, quoteAsset string, ok bool) {
	if r == nil {
		return "", "", false
	}

	label, ok := r.symbolLabelByID[cexSymbolIDKey{
		exchangeID:   exchangeID,
		marketTypeID: marketTypeID,
		symbolID:     id,
	}]
	if !ok {
		return "", "", false
	}

	metadata, ok := r.metadataByLabel[cexLegacySymbolKey{
		exchangeID: exchangeID,
		label:      label,
	}]
	if !ok || metadata.baseAsset == "" || metadata.quoteAsset == "" {
		return "", "", false
	}

	return metadata.baseAsset, metadata.quoteAsset, true
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

	r.symbolByLookup[lookupKey] = symbol.SymbolID
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

	for _, existing := range r.legacyCandidates[legacyKey] {
		if existing.SymbolID != symbol.SymbolID {
			return fmt.Errorf(
				"%w: exchange_id=%d label=%q existing_symbol_id=%d new_symbol_id=%d",
				errInconsistentLegacySymbolID,
				symbol.ExchangeID,
				label,
				existing.SymbolID,
				symbol.SymbolID,
			)
		}
	}

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

type cexSymbolMetadata struct {
	baseAsset  string
	quoteAsset string
}
