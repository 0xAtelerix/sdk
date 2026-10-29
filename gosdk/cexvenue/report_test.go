package cexvenue

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var errPollFailed = errors.New("poll failed")

func bnSpot(n int, status apptypes.CEXMarketStatus) apptypes.CEXMarketFacts {
	label := fmt.Sprintf("C%02dUSDT", n)

	return apptypes.CEXMarketFacts{
		ExchangeID:   ExchangeBinance,
		MarketTypeID: MarketTypeSpot,
		Label:        label,
		Status:       status,
	}
}

func tradingSpots(n int) []apptypes.CEXMarketFacts {
	out := make([]apptypes.CEXMarketFacts, n)
	for i := range out {
		out[i] = bnSpot(i, apptypes.CEXMarketStatusTrading)
	}

	return out
}

func TestApplyVenueReportRejectsFaults(t *testing.T) {
	t.Parallel()

	previous := tradingSpots(10)

	halfHalted := tradingSpots(10)
	for i := range 5 {
		halfHalted[i].Status = apptypes.CEXMarketStatusHalted
	}

	for name, tc := range map[string]struct {
		report []apptypes.CEXMarketFacts
		err    error
	}{
		"failed":             {tradingSpots(10), errPollFailed},
		"empty":              {nil, nil},
		"half missing":       {tradingSpots(10)[:5], nil},
		"half non-trading":   {halfHalted, nil},
		"thirty percent out": {tradingSpots(10)[:7], nil},
	} {
		next, accepted := ApplyVenueReport(previous, tc.report, tc.err)
		require.False(t, accepted, name)
		require.Equal(t, previous, next, name)
	}
}

func TestApplyVenueReportAccepts(t *testing.T) {
	t.Parallel()

	previous := tradingSpots(10)
	previous = append(
		previous,
		bnSpot(10, apptypes.CEXMarketStatusHalted),
		bnSpot(11, apptypes.CEXMarketStatusDelisted),
	)

	report := tradingSpots(10)[1:9]                     // C00 and C09 missing: exactly 20%, C10 and C11 missing too
	report[0].Status = apptypes.CEXMarketStatusDelisted // C01 BREAK
	report = append(report, bnSpot(12, apptypes.CEXMarketStatusTrading))

	_, accepted := ApplyVenueReport(previous, report, nil)
	require.False(t, accepted, "C01 delisted makes three of ten trading markets leave trading")

	report[0].Status = apptypes.CEXMarketStatusTrading
	report = append(report, bnSpot(9, apptypes.CEXMarketStatusDelisted))
	next, accepted := ApplyVenueReport(previous, report, nil)
	require.True(t, accepted)

	want := map[string]apptypes.CEXMarketStatus{
		"C00USDT": apptypes.CEXMarketStatusSuspectedDelisted,
		"C09USDT": apptypes.CEXMarketStatusDelisted,
		"C10USDT": apptypes.CEXMarketStatusSuspectedDelisted,
		"C11USDT": apptypes.CEXMarketStatusDelisted,
		"C12USDT": apptypes.CEXMarketStatusTrading,
	}

	require.Len(t, next, 13)

	for i, f := range next {
		require.Equal(t, fmt.Sprintf("C%02dUSDT", i), f.Label, "sorted by key")

		status, ok := want[f.Label]
		if !ok {
			status = apptypes.CEXMarketStatusTrading
		}

		require.Equal(t, status, f.Status, f.Label)
	}

	back, accepted := ApplyVenueReport(next, tradingSpots(13), nil)
	require.True(t, accepted, "markets return to trading")

	for _, f := range back {
		require.Equal(t, apptypes.CEXMarketStatusTrading, f.Status, f.Label)
	}
}

func TestApplyVenueReportFirstPollAndSingleBreak(t *testing.T) {
	t.Parallel()

	_, accepted := ApplyVenueReport(nil, nil, nil)
	require.False(t, accepted, "an empty first poll is a fault")

	first, accepted := ApplyVenueReport(nil, tradingSpots(1), nil)
	require.True(t, accepted)
	require.Len(t, first, 1)

	report := tradingSpots(10)
	report[3].Status = apptypes.CEXMarketStatusDelisted
	report[3].VenueStatus = "BREAK"

	next, accepted := ApplyVenueReport(tradingSpots(10), report, nil)
	require.True(t, accepted)
	require.Equal(t, apptypes.CEXMarketStatusDelisted, next[3].Status)
	require.Equal(t, "BREAK", next[3].VenueStatus)
}
