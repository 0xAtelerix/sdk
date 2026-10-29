package cexvenue

import (
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type mexcSymbol struct {
	Symbol               string `json:"symbol"`
	Status               string `json:"status"`
	BaseAsset            string `json:"baseAsset"`
	QuoteAsset           string `json:"quoteAsset"`
	BaseAssetPrecision   *uint8 `json:"baseAssetPrecision"`
	QuoteAssetPrecision  *uint8 `json:"quoteAssetPrecision"`
	QuoteAmountPrecision string `json:"quoteAmountPrecision"`
}

// NormalizeMEXCSpot turns a GET /api/v3/exchangeInfo response into records.
// MEXC lists no LOT_SIZE or PRICE_FILTER, so QuantityStep and PriceTick stay
// empty; quoteAmountPrecision (the minimum order amount in quote) is the
// minimum notional. Status "1" is trading, "2" paused, "3" offline.
func NormalizeMEXCSpot(raw []byte, observedAtUnixMs int64) (Normalized, error) {
	var info struct {
		Symbols []mexcSymbol `json:"symbols"`
	}

	if err := decode("mexc spot exchangeInfo", raw, &info); err != nil {
		return Normalized{}, err
	}

	c := &collector{exchangeID: ExchangeMEXC, observedAt: observedAtUnixMs}

	for i := range info.Symbols {
		s := &info.Symbols[i]
		c.add(&apptypes.CEXMarketFacts{
			MarketTypeID:        MarketTypeSpot,
			VenueCoin:           s.Symbol,
			Kind:                apptypes.CEXMarketKindMEXCSpot,
			Status:              mexcStatus(s.Status),
			VenueStatus:         s.Status,
			BaseAsset:           s.BaseAsset,
			QuoteAsset:          s.QuoteAsset,
			BaseAssetPrecision:  s.BaseAssetPrecision,
			QuoteAssetPrecision: s.QuoteAssetPrecision,
			MinNotional:         s.QuoteAmountPrecision,
		}, s.Symbol)
	}

	return finalize(c.facts, c.skipped), nil
}
