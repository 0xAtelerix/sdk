package apptypes

import (
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

	// DefaultOrderBookIDRegistry is the stateless source-controlled CEX
	// order-book identity registry used by compatibility wrappers.
	DefaultOrderBookIDRegistry OrderBookIDRegistry = 0
)

var (
	errEmptyCEXSymbol       = errors.New("empty cex symbol")
	errUnknownCEXExchange   = errors.New("unknown cex exchange")
	errUnknownCEXMarketType = errors.New("unknown cex market_type")
	errUnknownCEXSymbol     = errors.New("unknown cex symbol")
	errAmbiguousCEXSymbolID = errors.New("ambiguous cex symbol id")
)

// OrderBookIDRegistry owns source-controlled CEX order-book label-to-ID mappings.
// Storage and event refs consume only numeric IDs; label lookups stay at config,
// wrapper, diagnostics, and fixture boundaries.
type OrderBookIDRegistry uint8

// NewOrderBookIDRegistry constructs the committed exchange, market-type, and
// scoped symbol ID registry.
func NewOrderBookIDRegistry() *OrderBookIDRegistry {
	r := DefaultOrderBookIDRegistry

	return &r
}

// ResolveExchangeID maps a committed exchange label to its numeric ID.
func (OrderBookIDRegistry) ResolveExchangeID(exchange string) (CEXExchangeID, error) {
	switch strings.ToLower(strings.TrimSpace(exchange)) {
	case cexExchangeLabelMEXC:
		return CEXExchangeIDMEXC, nil
	case cexExchangeLabelHyperliquid:
		return CEXExchangeIDHyperliquid, nil
	default:
		return 0, fmt.Errorf("%w: %q", errUnknownCEXExchange, exchange)
	}
}

// ExchangeLabel maps a committed exchange ID to its diagnostic label.
func (OrderBookIDRegistry) ExchangeLabel(id CEXExchangeID) (string, bool) {
	switch id {
	case CEXExchangeIDMEXC:
		return cexExchangeLabelMEXC, true
	case CEXExchangeIDHyperliquid:
		return cexExchangeLabelHyperliquid, true
	default:
		return "", false
	}
}

// ResolveMarketTypeID maps a committed market-type label to its numeric ID.
func (OrderBookIDRegistry) ResolveMarketTypeID(marketType string) (CEXMarketTypeID, error) {
	switch strings.ToLower(strings.TrimSpace(marketType)) {
	case cexMarketTypeLabelSpot:
		return CEXMarketTypeIDSpot, nil
	case cexMarketTypeLabelPerp, "perps", "perpetual":
		return CEXMarketTypeIDPerp, nil
	default:
		return 0, fmt.Errorf("%w: %q", errUnknownCEXMarketType, marketType)
	}
}

// MarketTypeLabel maps a committed market-type ID to its diagnostic label.
func (OrderBookIDRegistry) MarketTypeLabel(id CEXMarketTypeID) (string, bool) {
	switch id {
	case CEXMarketTypeIDSpot:
		return cexMarketTypeLabelSpot, true
	case CEXMarketTypeIDPerp:
		return cexMarketTypeLabelPerp, true
	default:
		return "", false
	}
}

// ResolveSymbolID maps an exact source-controlled venue symbol label to the
// numeric symbol ID scoped by exchange and market type.
func (OrderBookIDRegistry) ResolveSymbolID(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	symbol string,
) (CEXSymbolID, error) {
	label := strings.TrimSpace(symbol)
	if label == "" {
		return 0, errEmptyCEXSymbol
	}

	for _, identity := range defaultCEXSymbols() {
		if identity.exchangeID == exchangeID &&
			identity.marketTypeID == marketTypeID &&
			identity.label == label {
			return identity.symbolID, nil
		}
	}

	return 0, fmt.Errorf(
		"%w: exchange_id=%d market_type_id=%d symbol=%q",
		errUnknownCEXSymbol,
		exchangeID,
		marketTypeID,
		symbol,
	)
}

// ResolveLegacySymbolID maps a deprecated exchange+symbol boundary lookup to the
// committed symbol ID shared by every registered market with that label. Storage
// still keys exact books by exchange ID, market-type ID, and symbol ID; this
// helper exists only so compatibility readers can run the bounded DB ambiguity
// check over exchange ID and symbol ID before choosing a market type.
func (OrderBookIDRegistry) ResolveLegacySymbolID(
	exchangeID CEXExchangeID,
	symbol string,
) (CEXSymbolID, error) {
	label := strings.TrimSpace(symbol)
	if label == "" {
		return 0, errEmptyCEXSymbol
	}

	var (
		resolved CEXSymbolID
		found    bool
	)
	for _, identity := range defaultCEXSymbols() {
		if identity.exchangeID != exchangeID || identity.label != label {
			continue
		}

		if !found {
			resolved = identity.symbolID
			found = true

			continue
		}

		if identity.symbolID != resolved {
			return 0, fmt.Errorf(
				"%w: exchange_id=%d symbol=%q",
				errAmbiguousCEXSymbolID,
				exchangeID,
				symbol,
			)
		}
	}

	if !found {
		return 0, fmt.Errorf(
			"%w: exchange_id=%d symbol=%q",
			errUnknownCEXSymbol,
			exchangeID,
			symbol,
		)
	}

	return resolved, nil
}

// SymbolLabel maps a scoped source-controlled symbol ID to its diagnostic label.
func (OrderBookIDRegistry) SymbolLabel(
	exchangeID CEXExchangeID,
	marketTypeID CEXMarketTypeID,
	id CEXSymbolID,
) (string, bool) {
	for _, identity := range defaultCEXSymbols() {
		if identity.exchangeID == exchangeID &&
			identity.marketTypeID == marketTypeID &&
			identity.symbolID == id {
			return identity.label, true
		}
	}

	return "", false
}

// CEXSymbolCandidate is one source-controlled market candidate for a legacy
// exchange+symbol boundary lookup.
type CEXSymbolCandidate struct {
	MarketTypeID CEXMarketTypeID
	SymbolID     CEXSymbolID
}

// SymbolCandidates returns bounded source-controlled market candidates for a
// deprecated exchange+symbol boundary lookup.
func (OrderBookIDRegistry) SymbolCandidates(
	exchangeID CEXExchangeID,
	symbol string,
) []CEXSymbolCandidate {
	label := strings.TrimSpace(symbol)
	if label == "" {
		return nil
	}

	out := make([]CEXSymbolCandidate, 0, 2)

	for _, identity := range defaultCEXSymbols() {
		if identity.exchangeID == exchangeID && identity.label == label {
			out = append(out, CEXSymbolCandidate{
				MarketTypeID: identity.marketTypeID,
				SymbolID:     identity.symbolID,
			})
		}
	}

	return out
}

type cexSymbolIdentity struct {
	exchangeID   CEXExchangeID
	marketTypeID CEXMarketTypeID
	symbolID     CEXSymbolID
	label        string
}

func defaultCEXSymbols() []cexSymbolIdentity {
	return []cexSymbolIdentity{
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 1, "BTCUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 2, "ETHUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 3, "SPXUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 4, "NPCUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 5, "PEPEUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 6, "WBTCUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 7, "FAKEUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 8, "SOLUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 9, "ETHUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 10, "BTCUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 11, "SPXUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 12, "PEPEUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 13, "FLOKIUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 14, "FLOKIUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 15, "LINKUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 16, "LINKUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 17, "UNIUSDT"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 18, "UNIUSDC"},
		{CEXExchangeIDMEXC, CEXMarketTypeIDSpot, 19, "DOGEUSDT"},
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
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, 13, "MOGUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 101, "BTC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 102, "ETH"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 103, "HYPE"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 104, "PEPE"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 105, "SPX"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 1, "BTCUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 2, "ETHUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 4, "PEPEUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 10, "FAKEUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 7, "DOGEUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 106, "LINKUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 107, "AAVEUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 108, "UNIUSDC"},
		{CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, 11, "SPXUSDC"},
	}
}
