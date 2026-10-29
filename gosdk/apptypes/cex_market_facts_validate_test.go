package apptypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCEXMarketFactsValidate(t *testing.T) {
	t.Parallel()

	const (
		hl, bn, mx   = CEXExchangeIDHyperliquid, CEXExchangeIDBinance, CEXExchangeIDMEXC
		spot, perp   = CEXMarketTypeIDSpot, CEXMarketTypeIDPerp
		hlCore, bnUM = CEXMarketKindHyperliquidCorePerp, CEXMarketKindBinanceUSDMPerpetual
	)

	venue := func(
		ex CEXExchangeID, mt CEXMarketTypeID, network string, kind CEXMarketKind,
	) func(*CEXMarketFacts) {
		return func(f *CEXMarketFacts) {
			f.ExchangeID, f.MarketTypeID, f.Network, f.Kind = ex, mt, network, kind
		}
	}

	for _, tc := range []struct {
		name    string
		mutate  func(*CEXMarketFacts)
		wantErr string
	}{
		{"valid-binance-spot", func(*CEXMarketFacts) {}, ""},
		{"valid-hl-spot-full", func(f *CEXMarketFacts) { *f = fullCEXMarketFacts() }, ""},
		{"valid-hl-core-perp", venue(hl, perp, "testnet", hlCore), ""},
		{"valid-hl-builder", venue(hl, perp, "mainnet", CEXMarketKindHyperliquidBuilder), ""},
		{"valid-binance-usdm", venue(bn, perp, "", bnUM), ""},
		{"valid-binance-tradfi", venue(bn, perp, "", CEXMarketKindBinanceTradFiPerp), ""},
		{"valid-binance-other", venue(bn, perp, "", CEXMarketKindBinanceOtherFutures), ""},
		{"valid-mexc-spot", venue(mx, spot, "", CEXMarketKindMEXCSpot), ""},
		{"zero-exchange", func(f *CEXMarketFacts) { f.ExchangeID = 0 }, "unknown exchange_id"},
		{"unknown-exchange", func(f *CEXMarketFacts) { f.ExchangeID = 99 }, "unknown exchange_id"},
		{"zero-market-type", func(f *CEXMarketFacts) { f.MarketTypeID = 0 }, "unknown market_type_id"},
		{"unknown-market-type", func(f *CEXMarketFacts) { f.MarketTypeID = 99 }, "unknown market_type_id"},
		{"empty-label", func(f *CEXMarketFacts) { f.Label = "" }, "label"},
		{"empty-venue-coin", func(f *CEXMarketFacts) { f.VenueCoin = "" }, "venue_coin"},
		{"empty-base", func(f *CEXMarketFacts) { f.BaseAsset = "" }, "base_asset"},
		{"empty-quote", func(f *CEXMarketFacts) { f.QuoteAsset = "" }, "quote_asset"},
		{"unknown-kind", func(f *CEXMarketFacts) { f.Kind = CEXMarketKindUnknown }, "unknown kind"},
		{"out-of-range-kind", func(f *CEXMarketFacts) { f.Kind = 9 }, "unknown kind"},
		{"unknown-status", func(f *CEXMarketFacts) { f.Status = CEXMarketStatusUnknown }, "status"},
		{"out-of-range-status", func(f *CEXMarketFacts) { f.Status = 5 }, "status"},
		{"hl-empty-network", venue(hl, perp, "", hlCore), "network"},
		{"hl-bad-network", venue(hl, perp, "devnet", hlCore), "network"},
		{"binance-with-network", venue(bn, spot, "mainnet", CEXMarketKindBinanceSpot), "network"},
		{"hl-kind-on-binance", venue(bn, perp, "", hlCore), "does not match"},
		{"binance-kind-on-hl", venue(hl, perp, "testnet", bnUM), "does not match"},
		{"binance-spot-kind-on-perp", venue(bn, perp, "", CEXMarketKindBinanceSpot), "does not match"},
		{"binance-perp-kind-on-spot", venue(bn, spot, "", bnUM), "does not match"},
		{"hl-perp-kind-on-spot", venue(hl, spot, "testnet", hlCore), "does not match"},
		{"hl-spot-kind-on-perp", venue(hl, perp, "testnet", CEXMarketKindHyperliquidSpot), "does not match"},
		{"mexc-kind-on-binance", venue(bn, spot, "", CEXMarketKindMEXCSpot), "does not match"},
		{"zero-observed-at", func(f *CEXMarketFacts) { f.ObservedAtUnixMs = 0 }, "observed_at"},
		{"negative-observed-at", func(f *CEXMarketFacts) { f.ObservedAtUnixMs = -1 }, "observed_at"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			facts := minimalCEXMarketFacts()
			tc.mutate(&facts)

			err := facts.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			require.ErrorIs(t, err, ErrCEXMarketFactsInvalid)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
