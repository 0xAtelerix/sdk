package cexvenue

import (
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Status mapping: the venue's delisted or closed states are delisted, its
// trading state is trading, and every other state is halted. Callers keep the
// raw venue string in VenueStatus.

const (
	statusTrading = apptypes.CEXMarketStatusTrading
	statusHalted  = apptypes.CEXMarketStatusHalted
	statusGone    = apptypes.CEXMarketStatusDelisted

	binanceTrading = "TRADING"
	binanceBreak   = "BREAK"

	// hyperliquidDelisted is the VenueStatus of a Hyperliquid market whose
	// metadata sets isDelisted; Hyperliquid has no status string of its own.
	hyperliquidDelisted = "isDelisted"
)

func binanceSpotStatus(raw string) apptypes.CEXMarketStatus {
	switch raw {
	case binanceTrading:
		return statusTrading
	case binanceBreak:
		return statusGone
	default:
		return statusHalted
	}
}

func binanceUSDMStatus(raw string) apptypes.CEXMarketStatus {
	switch raw {
	case binanceTrading:
		return statusTrading
	case "CLOSE", "SETTLING", "DELIVERED":
		return statusGone
	default:
		return statusHalted
	}
}

// mexcStatus maps MEXC spot status: "1" online, "2" paused, "3" offline.
func mexcStatus(raw string) apptypes.CEXMarketStatus {
	switch raw {
	case "1", "ENABLED", binanceTrading:
		return statusTrading
	case "3", "OFFLINE":
		return statusGone
	default:
		return statusHalted
	}
}
