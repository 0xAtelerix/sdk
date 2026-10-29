package cexvenue

import (
	"fmt"
)

// HyperliquidInput holds raw POST /info responses for one network.
type HyperliquidInput struct {
	Network  string // "mainnet" or "testnet"
	SpotMeta []byte // {"type":"spotMeta"}
	// ObservedAtUnixMs is the poll time, passed in by the caller.
	ObservedAtUnixMs int64
}

// NormalizeHyperliquid turns one network's Hyperliquid metadata into records.
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

	if err := c.addHyperliquidSpot(spot.Universe, tokens); err != nil {
		return Normalized{}, err
	}

	return finalize(c.facts, c.skipped), nil
}
