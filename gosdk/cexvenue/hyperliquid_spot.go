package cexvenue

import (
	"fmt"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// hlSpotAssetBase: a spot order asset id is 10000 + the spotMeta pair index.
const hlSpotAssetBase = 10000

type hlToken struct {
	Name        string `json:"name"`
	SzDecimals  uint8  `json:"szDecimals"`
	WeiDecimals uint8  `json:"weiDecimals"`
	Index       uint32 `json:"index"`
}

type hlSpotPair struct {
	Name   string   `json:"name"`
	Tokens []uint32 `json:"tokens"`
	Index  uint32   `json:"index"`
}

type hlSpotMeta struct {
	Tokens   []hlToken    `json:"tokens"`
	Universe []hlSpotPair `json:"universe"`
}

// addHyperliquidSpot labels each spot pair base token name + quote token name.
// Hyperliquid spot has no delisted flag, so every listed pair is trading.
func (c *collector) addHyperliquidSpot(pairs []hlSpotPair, tokens map[uint32]hlToken) error {
	for i := range pairs {
		pair := &pairs[i]
		if len(pair.Tokens) != 2 { //nolint:mnd // [base, quote]
			return fmt.Errorf("%w: hyperliquid spot %q tokens", ErrInvalidVenueResponse, pair.Name)
		}

		base, okBase := tokens[pair.Tokens[0]]
		quote, okQuote := tokens[pair.Tokens[1]]

		if !okBase || !okQuote {
			return fmt.Errorf(
				"%w: hyperliquid spot %q token unknown",
				ErrInvalidVenueResponse,
				pair.Name,
			)
		}

		c.add(&apptypes.CEXMarketFacts{
			MarketTypeID:     MarketTypeSpot,
			VenueCoin:        pair.Name,
			Kind:             apptypes.CEXMarketKindHyperliquidSpot,
			Status:           statusTrading,
			BaseAsset:        base.Name,
			QuoteAsset:       quote.Name,
			VenueAssetID:     hlSpotAssetBase + pair.Index,
			SizeDecimals:     &base.SzDecimals,
			BaseTokenIndex:   &pair.Tokens[0],
			QuoteTokenIndex:  &pair.Tokens[1],
			BaseWeiDecimals:  &base.WeiDecimals,
			QuoteWeiDecimals: &quote.WeiDecimals,
		}, base.Name+quote.Name)
	}

	return nil
}
