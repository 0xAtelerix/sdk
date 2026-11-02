package cexvenue

import (
	"fmt"
)

// HyperliquidInput holds raw POST /info responses for one network.
type HyperliquidInput struct {
	Network  string // "mainnet" or "testnet"
	PerpDexs []byte // {"type":"perpDexs"}
	// PerpMetas maps each dex in PerpDexs ("" is the core dex) to its
	// {"type":"meta","dex":name} response.
	PerpMetas      map[string][]byte
	SpotMeta       []byte // {"type":"spotMeta"}
	PerpCategories []byte // {"type":"perpCategories"}; nil leaves Category empty
	// ObservedAtUnixMs is the poll time, passed in by the caller.
	ObservedAtUnixMs int64
}

// NormalizeHyperliquid turns one network's Hyperliquid metadata into records.
// It fails when any response is unreadable or a listed dex has no meta.
func NormalizeHyperliquid(in HyperliquidInput) (Normalized, error) {
	if in.Network == "" {
		return Normalized{}, fmt.Errorf("%w: hyperliquid network is empty", ErrInvalidVenueResponse)
	}

	var spot hlSpotMeta
	if err := decode("spotMeta", in.SpotMeta, &spot); err != nil {
		return Normalized{}, err
	}

	tokens := make(map[uint32]hlToken, len(spot.Tokens))
	for _, tok := range spot.Tokens {
		tokens[tok.Index] = tok
	}

	c := &collector{
		exchangeID: ExchangeHyperliquid,
		network:    in.Network,
		observedAt: in.ObservedAtUnixMs,
	}

	if err := c.addHyperliquidPerps(&in, tokens); err != nil {
		return Normalized{}, err
	}

	if err := c.addHyperliquidSpot(spot.Universe, tokens); err != nil {
		return Normalized{}, err
	}

	return finalize(c.facts, c.skipped), nil
}
