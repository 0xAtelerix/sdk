package cexvenue

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// ErrInvalidVenueResponse marks a venue response that cannot be normalized.
var ErrInvalidVenueResponse = errors.New("invalid venue response")

// collector applies the label rule and the common fields to each market.
type collector struct {
	exchangeID apptypes.CEXExchangeID
	network    string
	observedAt int64
	facts      []apptypes.CEXMarketFacts
	skipped    []SkippedMarket
}

func (c *collector) add(f *apptypes.CEXMarketFacts, candidate string) {
	f.ExchangeID, f.Network, f.ObservedAtUnixMs = c.exchangeID, c.network, c.observedAt

	label, ok := NewMarketLabel(f.ExchangeID, f.MarketTypeID, candidate)
	if !ok {
		c.skipped = append(c.skipped, SkippedMarket{
			MarketTypeID: f.MarketTypeID,
			Network:      f.Network,
			VenueCoin:    f.VenueCoin,
			Reason:       SkipReasonLabelRule,
		})

		return
	}

	f.Label = label
	c.facts = append(c.facts, *f)
}

func decode(what string, raw []byte, out any) error {
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInvalidVenueResponse, what, err)
	}

	return nil
}
