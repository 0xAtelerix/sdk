package cexvenue

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestVenueStatusMapping(t *testing.T) {
	t.Parallel()

	const (
		trading  = apptypes.CEXMarketStatusTrading
		halted   = apptypes.CEXMarketStatusHalted
		delisted = apptypes.CEXMarketStatusDelisted
	)

	cases := []struct {
		mapper func(string) apptypes.CEXMarketStatus
		raw    string
		want   apptypes.CEXMarketStatus
	}{
		{binanceSpotStatus, "TRADING", trading},
		{binanceSpotStatus, "BREAK", delisted},
		{binanceSpotStatus, "HALT", halted},
		{binanceSpotStatus, "PRE_TRADING", halted},
		{binanceSpotStatus, "AUCTION_MATCH", halted},
		{binanceSpotStatus, "END_OF_DAY", halted},
		{binanceSpotStatus, "", halted},
		{binanceUSDMStatus, "TRADING", trading},
		{binanceUSDMStatus, "CLOSE", delisted},
		{binanceUSDMStatus, "SETTLING", delisted},
		{binanceUSDMStatus, "DELIVERED", delisted},
		{binanceUSDMStatus, "PENDING_TRADING", halted},
		{binanceUSDMStatus, "PRE_DELIVERING", halted},
		{binanceUSDMStatus, "DELIVERING", halted},
		{binanceUSDMStatus, "PRE_SETTLE", halted},
		{binanceUSDMStatus, "BREAK", halted},
		{mexcStatus, "1", trading},
		{mexcStatus, "ENABLED", trading},
		{mexcStatus, "2", halted},
		{mexcStatus, "3", delisted},
		{mexcStatus, "OFFLINE", delisted},
		{mexcStatus, "unknown", halted},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, tc.mapper(tc.raw), tc.raw)
	}
}
