// Package cexvenue turns raw venue market listings into market facts records.
// Every function is pure: no network reads and no clock reads.
package cexvenue

import (
	"strings"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Frozen v9 exchange and market-type ids.
const (
	ExchangeMEXC        apptypes.CEXExchangeID   = 1
	ExchangeHyperliquid apptypes.CEXExchangeID   = 2
	ExchangeBinance     apptypes.CEXExchangeID   = 3
	MarketTypeSpot      apptypes.CEXMarketTypeID = 1
	MarketTypePerp      apptypes.CEXMarketTypeID = 2
)

// LabelRuleVersion is the version of the rule NewMarketLabel applies.
const LabelRuleVersion = 1

// NewMarketLabel applies label rule v1 to a candidate built from venue names.
// A label in the frozen v9 table for that exchange and market type keeps its
// spelling. Any other label must be ASCII letters and digits only, because
// downstream symbol and token types reject everything else. A refused
// candidate returns ok=false and the market gets no record.
func NewMarketLabel(
	exchangeID apptypes.CEXExchangeID,
	marketTypeID apptypes.CEXMarketTypeID,
	candidate string,
) (string, bool) {
	if candidate == "" {
		return "", false
	}

	if isASCIIAlnum(candidate) {
		return candidate, true
	}

	if strings.TrimSpace(candidate) == candidate {
		_, err := apptypes.DefaultOrderBookIDRegistry.ResolveSymbolID(
			exchangeID,
			marketTypeID,
			candidate,
		)
		if err == nil {
			return candidate, true
		}
	}

	return "", false
}

func isASCIIAlnum(s string) bool {
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}

	return true
}
