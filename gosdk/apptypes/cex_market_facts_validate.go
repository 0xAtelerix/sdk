package apptypes

import (
	"cmp"
	"fmt"
	"slices"

	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

// ErrCEXMarketFactsInvalid is the root of every market facts validation error.
const ErrCEXMarketFactsInvalid sdkerrors.SDKError = "invalid cex market facts"

// CEXMarketFactsKey identifies the market a facts record describes. A newer
// record with the same key replaces the stored one.
type CEXMarketFactsKey struct {
	ExchangeID   CEXExchangeID
	MarketTypeID CEXMarketTypeID
	Network      string
	Label        string
}

// Key returns the record key (ExchangeID, MarketTypeID, Network, Label).
func (f CEXMarketFacts) Key() CEXMarketFactsKey {
	return CEXMarketFactsKey{
		ExchangeID:   f.ExchangeID,
		MarketTypeID: f.MarketTypeID,
		Network:      f.Network,
		Label:        f.Label,
	}
}

// SortCEXMarketFacts sorts records by key; records with equal keys keep their
// input order.
func SortCEXMarketFacts(records []CEXMarketFacts) {
	slices.SortStableFunc(records, func(a, b CEXMarketFacts) int {
		return cmp.Or(
			cmp.Compare(a.ExchangeID, b.ExchangeID),
			cmp.Compare(a.MarketTypeID, b.MarketTypeID),
			cmp.Compare(a.Network, b.Network),
			cmp.Compare(a.Label, b.Label),
		)
	})
}

// Validate checks a record without registry state beyond the embedded
// exchange and market-type tables. SymbolID is not checked here; receivers
// recompute it from the exchange and label.
func (f CEXMarketFacts) Validate() error {
	if _, ok := DefaultOrderBookIDRegistry.ExchangeLabel(f.ExchangeID); !ok {
		return fmt.Errorf("%w: unknown exchange_id=%d", ErrCEXMarketFactsInvalid, f.ExchangeID)
	}

	if _, ok := DefaultOrderBookIDRegistry.MarketTypeLabel(f.MarketTypeID); !ok {
		return fmt.Errorf("%w: unknown market_type_id=%d", ErrCEXMarketFactsInvalid, f.MarketTypeID)
	}

	for _, field := range []struct{ name, value string }{
		{"label", f.Label},
		{"venue_coin", f.VenueCoin},
		{"base_asset", f.BaseAsset},
		{"quote_asset", f.QuoteAsset},
	} {
		if field.value == "" {
			return fmt.Errorf("%w: %s is empty", ErrCEXMarketFactsInvalid, field.name)
		}
	}

	if f.Status < CEXMarketStatusTrading || f.Status > CEXMarketStatusDelisted {
		return fmt.Errorf("%w: unknown status=%d", ErrCEXMarketFactsInvalid, f.Status)
	}

	if err := f.validateNetwork(); err != nil {
		return err
	}

	exchangeID, marketTypeID, ok := cexMarketKindVenue(f.Kind)
	if !ok {
		return fmt.Errorf("%w: unknown kind=%d", ErrCEXMarketFactsInvalid, f.Kind)
	}

	if exchangeID != f.ExchangeID || marketTypeID != f.MarketTypeID {
		return fmt.Errorf(
			"%w: kind=%d does not match exchange_id=%d market_type_id=%d",
			ErrCEXMarketFactsInvalid, f.Kind, f.ExchangeID, f.MarketTypeID,
		)
	}

	if f.ObservedAtUnixMs <= 0 {
		return fmt.Errorf(
			"%w: observed_at_unix_ms=%d",
			ErrCEXMarketFactsInvalid,
			f.ObservedAtUnixMs,
		)
	}

	return nil
}

func (f CEXMarketFacts) validateNetwork() error {
	if f.ExchangeID != CEXExchangeIDHyperliquid {
		if f.Network != "" {
			return fmt.Errorf(
				"%w: network is set for a non-hyperliquid exchange",
				ErrCEXMarketFactsInvalid,
			)
		}

		return nil
	}

	switch HyperliquidNetwork(f.Network) {
	case HyperliquidNetworkMainnet, HyperliquidNetworkTestnet:
		return nil
	default:
		return fmt.Errorf("%w: hyperliquid network=%q", ErrCEXMarketFactsInvalid, f.Network)
	}
}

// cexMarketKindVenue returns the exchange and market type a kind belongs to.
func cexMarketKindVenue(kind CEXMarketKind) (CEXExchangeID, CEXMarketTypeID, bool) {
	switch kind {
	case CEXMarketKindHyperliquidCorePerp, CEXMarketKindHyperliquidBuilder:
		return CEXExchangeIDHyperliquid, CEXMarketTypeIDPerp, true
	case CEXMarketKindHyperliquidSpot:
		return CEXExchangeIDHyperliquid, CEXMarketTypeIDSpot, true
	case CEXMarketKindBinanceSpot:
		return CEXExchangeIDBinance, CEXMarketTypeIDSpot, true
	case CEXMarketKindBinanceUSDMPerpetual, CEXMarketKindBinanceTradFiPerp,
		CEXMarketKindBinanceOtherFutures:
		return CEXExchangeIDBinance, CEXMarketTypeIDPerp, true
	case CEXMarketKindMEXCSpot:
		return CEXExchangeIDMEXC, CEXMarketTypeIDSpot, true
	case CEXMarketKindUnknown:
		fallthrough
	default:
		return 0, 0, false
	}
}
