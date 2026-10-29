package cexvenue

import (
	"cmp"
	"slices"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// SkipReason says why a venue market got no record.
type SkipReason string

const (
	// SkipReasonLabelRule: the label would break label rule v1.
	SkipReasonLabelRule SkipReason = "label_rule"
	// SkipReasonDuplicateLabel: two venue markets produce one label; both are refused.
	SkipReasonDuplicateLabel SkipReason = "duplicate_label"
)

// SkippedMarket is a venue market that got no record.
type SkippedMarket struct {
	MarketTypeID apptypes.CEXMarketTypeID
	Network      string
	VenueCoin    string
	Reason       SkipReason
}

// Normalized is one venue response turned into records, sorted by record key,
// plus the markets that got no record, sorted by venue coin.
type Normalized struct {
	Markets []apptypes.CEXMarketFacts
	Skipped []SkippedMarket
}

// finalize refuses every market whose record key is shared with another
// market and sorts both lists.
func finalize(facts []apptypes.CEXMarketFacts, skipped []SkippedMarket) Normalized {
	counts := make(map[marketKey]int, len(facts))
	for i := range facts {
		counts[keyOf(&facts[i])]++
	}

	out := Normalized{Markets: make([]apptypes.CEXMarketFacts, 0, len(facts)), Skipped: skipped}

	for i := range facts {
		f := &facts[i]
		if counts[keyOf(f)] > 1 {
			out.Skipped = append(out.Skipped, SkippedMarket{
				MarketTypeID: f.MarketTypeID,
				Network:      f.Network,
				VenueCoin:    f.VenueCoin,
				Reason:       SkipReasonDuplicateLabel,
			})

			continue
		}

		out.Markets = append(out.Markets, *f)
	}

	sortFacts(out.Markets)
	slices.SortFunc(out.Skipped, func(a, b SkippedMarket) int {
		return cmp.Or(
			cmp.Compare(a.VenueCoin, b.VenueCoin),
			cmp.Compare(a.MarketTypeID, b.MarketTypeID),
			cmp.Compare(a.Network, b.Network),
			cmp.Compare(a.Reason, b.Reason),
		)
	})

	return out
}

type marketKey struct {
	exchangeID   apptypes.CEXExchangeID
	marketTypeID apptypes.CEXMarketTypeID
	network      string
	label        string
}

func keyOf(f *apptypes.CEXMarketFacts) marketKey {
	return marketKey{
		exchangeID:   f.ExchangeID,
		marketTypeID: f.MarketTypeID,
		network:      f.Network,
		label:        f.Label,
	}
}

func sortFacts(facts []apptypes.CEXMarketFacts) {
	slices.SortFunc(facts, func(a, b apptypes.CEXMarketFacts) int {
		return cmp.Or(
			cmp.Compare(a.ExchangeID, b.ExchangeID),
			cmp.Compare(a.MarketTypeID, b.MarketTypeID),
			cmp.Compare(a.Network, b.Network),
			cmp.Compare(a.Label, b.Label),
		)
	})
}
