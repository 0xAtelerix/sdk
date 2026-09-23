package gosdk

import (
	"testing"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

// blockPhaseOwnedPhases are the phases that tile one block. Adding a step to
// processBatch without a phase leaves its time in unaccounted, which is the
// state this instrumentation exists to make visible.
func blockPhaseOwnedPhases() []string {
	return []string{
		blockPhasePrepareProcessor,
		blockPhaseBeginTx,
		blockPhaseProcessBatch,
		blockPhaseStoreReceipts,
		blockPhaseStateRoot,
		blockPhaseBuildBlock,
		blockPhaseWriteBlock,
		blockPhaseCommit,
		blockPhaseAfterCommit,
	}
}

func blockPhaseHistogram(t *testing.T, validatorID string, phase string) *dto.Histogram {
	t.Helper()

	observer := BlockPhaseDuration.WithLabelValues(validatorID, "42", phase)

	metric, ok := observer.(prometheus.Metric)
	require.True(t, ok, "histogram child must expose its samples")

	written := dto.Metric{}
	require.NoError(t, metric.Write(&written))

	return written.GetHistogram()
}

func blockPhaseAbortValue(t *testing.T, validatorID string, phase string) float64 {
	t.Helper()

	written := dto.Metric{}
	require.NoError(t, BlockAbortedTotal.WithLabelValues(validatorID, "42", phase).Write(&written))

	return written.GetCounter().GetValue()
}

// TestBlockPhasesCloseAgainstTotal runs one real block through processBatch and
// proves the published phases account for all of it.
func TestBlockPhasesCloseAgainstTotal(t *testing.T) {
	t.Parallel()

	processor := &step159YBatchCommitObserverProcessor{}
	require.NoError(t, runStep159YBatch(t, processor))

	var phaseSum float64

	for _, phase := range blockPhaseOwnedPhases() {
		histogram := blockPhaseHistogram(t, t.Name(), phase)
		require.Equalf(t, uint64(1), histogram.GetSampleCount(),
			"phase %s must be observed exactly once per block", phase)

		phaseSum += histogram.GetSampleSum()
	}

	remainder := blockPhaseHistogram(t, t.Name(), blockPhaseUnaccounted)
	require.Equal(t, uint64(1), remainder.GetSampleCount())

	total := blockPhaseHistogram(t, t.Name(), blockPhaseTotal)
	require.Equal(t, uint64(1), total.GetSampleCount())

	require.InDelta(t, total.GetSampleSum(), phaseSum+remainder.GetSampleSum(), 1e-9,
		"owned phases plus the remainder must reconstruct the block")
	require.GreaterOrEqual(t, remainder.GetSampleSum(), float64(0))
}

// TestBlockPhaseAbortNamesTheFailedPhase covers the halt signal: an aborted
// block publishes no duration at all, so the counter is the only evidence that
// block production stopped rather than idled.
func TestBlockPhaseAbortNamesTheFailedPhase(t *testing.T) {
	t.Parallel()

	processor := &step159YBatchCommitObserverProcessor{
		processErr: step159YTestError("batch aborted"),
	}
	require.Error(t, runStep159YBatch(t, processor))

	require.InDelta(t, float64(1), blockPhaseAbortValue(t, t.Name(), blockPhaseProcessBatch), 0)
	require.Zero(t, blockPhaseHistogram(t, t.Name(), blockPhaseTotal).GetSampleCount(),
		"an aborted block must not report a completed total")
	require.Zero(t, blockPhaseHistogram(t, t.Name(), blockPhaseProcessBatch).GetSampleCount(),
		"the failed phase reports no duration, only the abort")
	require.Equal(t, uint64(1),
		blockPhaseHistogram(t, t.Name(), blockPhasePrepareProcessor).GetSampleCount(),
		"phases completed before the abort stay observed")
}

// TestBlockPhaseAbortCoversCommitFailure pins the phase name at the site that
// matters most for a halt: the commit itself.
func TestBlockPhaseAbortCoversCommitFailure(t *testing.T) {
	t.Parallel()

	processor := &step159YBatchCommitObserverProcessor{}
	err := runStep159YBatchWithDBWrapper(t, processor, func(db kv.RwDB) kv.RwDB {
		return step159YFailingCommitDB{RwDB: db}
	})
	require.Error(t, err)

	require.InDelta(t, float64(1), blockPhaseAbortValue(t, t.Name(), blockPhaseCommit), 0)
	require.InDelta(t, float64(0), blockPhaseAbortValue(t, t.Name(), blockPhaseProcessBatch), 0)
}

func TestBlockPhaseLedgerRemainderClamps(t *testing.T) {
	t.Parallel()

	ledger := newBlockPhaseLedger(t.Name(), "42")
	ledger.attributed = 250 * time.Millisecond

	require.Equal(t, 150*time.Millisecond, ledger.unaccounted(400*time.Millisecond),
		"the remainder is exactly the time no phase claimed")
	require.Zero(t, ledger.unaccounted(100*time.Millisecond),
		"per-phase rounding must never publish a negative remainder")
}
