package cexvenue

import (
	"slices"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const percent = 100

// ApplyVenueReport merges one venue poll (one exchange, network and endpoint)
// into the last accepted facts. A failed poll (pollErr != nil), an empty
// report, or one that takes more than CEXVenueFaultMaxRemovalPercent of the
// trading markets out of trading (missing or non-trading) is a venue fault,
// rejected whole: previous is returned unchanged. Otherwise reported markets
// take the reported facts, and a market the report omits keeps its last facts
// as suspected_delisted (or delisted if the venue already said so). The
// result is sorted by record key.
func ApplyVenueReport(
	previous, report []apptypes.CEXMarketFacts,
	pollErr error,
) ([]apptypes.CEXMarketFacts, bool) {
	if pollErr != nil || len(report) == 0 {
		return previous, false
	}

	reported := make(map[marketKey]apptypes.CEXMarketStatus, len(report))
	for i := range report {
		reported[keyOf(&report[i])] = report[i].Status
	}

	var wasTrading, leftTrading int

	for i := range previous {
		if previous[i].Status != apptypes.CEXMarketStatusTrading {
			continue
		}

		wasTrading++

		if status, found := reported[keyOf(&previous[i])]; !found ||
			status != apptypes.CEXMarketStatusTrading {
			leftTrading++
		}
	}

	if leftTrading*percent > apptypes.CEXVenueFaultMaxRemovalPercent*wasTrading {
		return previous, false
	}

	next := slices.Clone(report)

	for i := range previous {
		if _, found := reported[keyOf(&previous[i])]; found {
			continue
		}

		missing := previous[i]
		if missing.Status != apptypes.CEXMarketStatusDelisted {
			missing.Status = apptypes.CEXMarketStatusSuspectedDelisted
		}

		next = append(next, missing)
	}

	sortFacts(next)

	return next, true
}
