package cexvenue

import (
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// binanceFilter holds the exchangeInfo filter fields the facts use.
type binanceFilter struct {
	FilterType  string `json:"filterType"`
	StepSize    string `json:"stepSize"`
	TickSize    string `json:"tickSize"`
	MinNotional string `json:"minNotional"` // spot NOTIONAL and MIN_NOTIONAL
	Notional    string `json:"notional"`    // USD-M MIN_NOTIONAL
}

type binanceSymbol struct {
	Symbol              string          `json:"symbol"`
	Status              string          `json:"status"`
	BaseAsset           string          `json:"baseAsset"`
	QuoteAsset          string          `json:"quoteAsset"`
	BaseAssetPrecision  *uint8          `json:"baseAssetPrecision"`
	QuoteAssetPrecision *uint8          `json:"quoteAssetPrecision"`
	Filters             []binanceFilter `json:"filters"`
}

type binanceExchangeInfo struct {
	Symbols []binanceSymbol `json:"symbols"`
}

// NormalizeBinanceSpot turns a GET /api/v3/exchangeInfo response into records.
// Every listed symbol is reported with its raw status; the label is the venue
// symbol under label rule v1.
func NormalizeBinanceSpot(raw []byte, observedAtUnixMs int64) (Normalized, error) {
	var info binanceExchangeInfo
	if err := decode("binance spot exchangeInfo", raw, &info); err != nil {
		return Normalized{}, err
	}

	c := &collector{exchangeID: ExchangeBinance, observedAt: observedAtUnixMs}

	for i := range info.Symbols {
		s := &info.Symbols[i]
		f := binanceFacts(
			s,
			MarketTypeSpot,
			apptypes.CEXMarketKindBinanceSpot,
			binanceSpotStatus(s.Status),
		)
		f.QuoteAssetPrecision = s.QuoteAssetPrecision
		c.add(&f, s.Symbol)
	}

	return finalize(c.facts, c.skipped), nil
}

func binanceFacts(
	s *binanceSymbol,
	marketTypeID apptypes.CEXMarketTypeID,
	kind apptypes.CEXMarketKind,
	status apptypes.CEXMarketStatus,
) apptypes.CEXMarketFacts {
	f := apptypes.CEXMarketFacts{
		MarketTypeID:       marketTypeID,
		VenueCoin:          s.Symbol,
		Kind:               kind,
		Status:             status,
		VenueStatus:        s.Status,
		BaseAsset:          s.BaseAsset,
		QuoteAsset:         s.QuoteAsset,
		BaseAssetPrecision: s.BaseAssetPrecision,
	}

	for _, filter := range s.Filters {
		switch filter.FilterType {
		case "LOT_SIZE":
			f.QuantityStep = filter.StepSize
		case "PRICE_FILTER":
			f.PriceTick = filter.TickSize
		case "NOTIONAL", "MIN_NOTIONAL":
			f.MinNotional = filter.MinNotional
			if filter.Notional != "" {
				f.MinNotional = filter.Notional
			}
		default:
		}
	}

	return f
}
