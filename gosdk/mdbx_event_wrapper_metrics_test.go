package gosdk

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestObserveCEXEventRefAgesRecordsPerSymbol(t *testing.T) {
	t.Parallel()

	now := time.UnixMilli(1_000)

	observeCEXEventRefAges([]apptypes.CEXOrderBookRef{{
		Exchange:  "mexc",
		Symbol:    "NPCUSDT",
		FetchedAt: now.Add(-250 * time.Millisecond).UnixNano(),
	}}, "validator-test", "chain-test", now)

	observer, err := MdbxCEXEventHandoffRefAge.GetMetricWithLabelValues(
		"validator-test",
		"chain-test",
		"mexc",
		"NPCUSDT",
	)
	require.NoError(t, err)

	var metric dto.Metric

	writable, ok := observer.(interface {
		Write(metric *dto.Metric) error
	})
	require.True(t, ok)
	require.NoError(t, writable.Write(&metric))
	require.Equal(t, uint64(1), metric.GetHistogram().GetSampleCount())
}
