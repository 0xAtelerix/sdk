package gosdk

import "time"

// Phase labels for BlockPhaseDuration and BlockAbortedTotal. One appchain block
// is exactly these phases plus the unaccounted remainder, so a panel built on
// them cannot quietly drop an untimed step the way a partial breakdown does.
const (
	blockPhasePrepareProcessor = "prepare_processor"
	blockPhaseBeginTx          = "begin_tx"
	blockPhaseProcessBatch     = "process_batch"
	blockPhaseStoreReceipts    = "store_receipts"
	blockPhaseStateRoot        = "state_root"
	blockPhaseBuildBlock       = "build_block"
	blockPhaseWriteBlock       = "write_block"
	blockPhaseCommit           = "commit"
	blockPhaseAfterCommit      = "after_commit"
	blockPhaseUnaccounted      = "unaccounted"
	blockPhaseTotal            = "total"
)

// blockPhaseLedger measures one processBatch call phase by phase. Each phase
// starts where the previous one ended, so the phases tile the block; whatever
// they do not claim is published as unaccounted instead of vanishing from the
// sum.
type blockPhaseLedger struct {
	validatorID string
	chainID     string
	start       time.Time
	phaseStart  time.Time
	attributed  time.Duration
}

func newBlockPhaseLedger(validatorID string, chainID string) blockPhaseLedger {
	now := time.Now()

	return blockPhaseLedger{
		validatorID: validatorID,
		chainID:     chainID,
		start:       now,
		phaseStart:  now,
	}
}

// observe closes one phase and opens the next one at the same instant.
func (ledger *blockPhaseLedger) observe(phase string) {
	now := time.Now()
	elapsed := now.Sub(ledger.phaseStart)

	ledger.attributed += elapsed
	ledger.phaseStart = now

	BlockPhaseDuration.WithLabelValues(ledger.validatorID, ledger.chainID, phase).
		Observe(elapsed.Seconds())
}

// abort records that block production stopped inside this phase. An aborted
// block never reaches observeTotal, so without this counter a halt shows up
// only as phases that stop arriving — which is indistinguishable from a node
// that was shut down on purpose.
func (ledger *blockPhaseLedger) abort(phase string) {
	BlockAbortedTotal.WithLabelValues(ledger.validatorID, ledger.chainID, phase).Inc()
}

// unaccounted is the part of the block no phase claimed. It is clamped because
// the phases are measured one by one and their rounding can exceed the single
// total measurement on an empty block.
func (ledger *blockPhaseLedger) unaccounted(total time.Duration) time.Duration {
	return max(total-ledger.attributed, 0)
}

// observeTotal closes the block: it publishes the remainder before total so
// both describe the same block.
func (ledger *blockPhaseLedger) observeTotal() {
	total := time.Since(ledger.start)

	BlockPhaseDuration.WithLabelValues(ledger.validatorID, ledger.chainID, blockPhaseUnaccounted).
		Observe(ledger.unaccounted(total).Seconds())
	BlockPhaseDuration.WithLabelValues(ledger.validatorID, ledger.chainID, blockPhaseTotal).
		Observe(total.Seconds())
}
