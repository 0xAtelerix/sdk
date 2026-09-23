package gosdk

import "github.com/prometheus/client_golang/prometheus"

//nolint:gochecknoglobals // metrics - it's easier to use global vars then put everything into context as a container
var (
	// ProcessedBlocks counts committed appchain blocks.
	ProcessedBlocks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "processed_blocks_total",
			Help:      "Total number of processed blocks",
		},
		[]string{"validator_id", "chain_id"},
	)
	ProcessedTransactions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "processed_transactions_total",
			Help:      "Total number of processed transactions",
		},
		[]string{"validator_id", "chain_id"},
	)
	BlockProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "block_processing_duration_seconds",
			Help:      "Histogram of block processing durations",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)
	// BlockPhaseDuration splits one appchain block into phases. The phases
	// prepare_processor, begin_tx, process_batch, store_receipts, state_root,
	// build_block, write_block, commit and after_commit tile the block, and with
	// unaccounted they add up to total. Its buckets resolve the 100-500ms region
	// the default set collapses into two buckets.
	BlockPhaseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "block_phase_duration_seconds",
			Help:      "Duration of each block-production phase; phases plus unaccounted sum to total",
			Buckets: []float64{
				.0001, .0002, .0004, .0008, .0016, .0032, .0064, .0128, .0256, .0512, .064,
				.1, .128, .16, .18, .2, .22, .24, .256, .26, .28, .3, .32, .34, .36, .38, .4, .42, .44, .46, .48, .5,
				.512, 1.024, 2.048, 4.096, 8.192, 16.384, 32.768, 52.4288,
			},
		},
		[]string{"validator_id", "chain_id", "phase"},
	)
	// BlockAbortedTotal counts blocks whose production stopped, by the phase it
	// stopped in. An aborted block publishes no duration at all, so this is the
	// only positive signal that the node halted rather than idled.
	BlockAbortedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "block_aborted_total",
			Help:      "Blocks whose production aborted, labelled by the phase that failed",
		},
		[]string{"validator_id", "chain_id", "phase"},
	)
	BatchProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "batch_processing_duration_seconds",
			Help:      "Time to process a single batch (excluding blocking)",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)
	BatchTransactions = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "batch_transactions",
			Help:      "Transactions per appchain batch",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	BatchExternalBlocks = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "batch_external_blocks",
			Help:      "External blocks per appchain batch",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	BatchCheckpoints = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "batch_checkpoints",
			Help:      "Checkpoints per appchain batch",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	BatchCEXOrderBookRefs = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "batch_cex_order_book_refs",
			Help:      "CEX order book refs per appchain batch",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	BatchCommitObserverCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "batch_commit_observer_calls_total",
			Help:      "Post-commit batch observer calls partitioned by success or error",
		},
		[]string{"validator_id", "chain_id", "result"},
	)
	EventStreamBlockingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "get_batches_duration",
			Help:      "Time spent blocking on event stream batch retrieval",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)

	// HeadBlockNumber tracks the latest committed block number.
	HeadBlockNumber = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "head_block_number",
			Help:      "Current committed block number",
		},
		[]string{"validator_id", "chain_id"},
	)
	EventStreamPosition = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "appchain",
			Subsystem: "stream",
			Name:      "event_stream_position_bytes",
			Help:      "Last persisted event stream position",
		},
		[]string{"validator_id", "chain_id", "epoch"},
	)

	// EventBatchEvents records the number of events in a stream batch.
	EventBatchEvents = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "stream",
			Name:      "event_batch_events",
			Help:      "Number of events in a single event batch",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	BlockExternalTxs = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "block_external_txs",
			Help:      "External transactions per block",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	BlockInternalTxs = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "block_internal_txs",
			Help:      "Internal transactions per block",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	BlockBytes = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "block_bytes",
			Help:      "Serialized block size in bytes",
			Buckets:   prometheus.ExponentialBuckets(256, 2, 16),
		},
		[]string{"validator_id", "chain_id"},
	)

	// StreamReadDuration records event stream read duration.
	StreamReadDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "io",
			Name:      "stream_read_duration_seconds",
			Help:      "Time to read a single batch from file",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)

	// MdbxWaitCyclesTotal counts MDBX wrapper wait cycles.
	MdbxWaitCyclesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "wait_cycles_total",
			Help:      "Sleep cycles until all tx-batches are found",
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxWaitTimeSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "wait_time_seconds",
			Help:      "Total wait time per event batch",
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxTxLookupDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "tx_lookup_duration_seconds",
			Help:      "MDBX lookup (BeginRo/GetOne/Unflatten)",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxTxBatchesExpectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "tx_batches_expected_total",
			Help:      "Expected tx-batches per event batch",
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxTxBatchesFoundTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "tx_batches_found_total",
			Help:      "Found tx-batches per event batch",
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxEventParseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "event_parse_duration_seconds",
			Help:      "Time to unmarshal events in event batch",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxCEXEventHandoffRefs = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "cex_event_handoff_refs",
			Help:      "CEX order book refs per admitted CEX event",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxCEXEventHandoffDecodeDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "cex_event_handoff_decode_duration_seconds",
			Help:      "Time to decode an admitted CEX event",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxCEXEventHandoffNewestAge = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "cex_event_handoff_newest_age_seconds",
			Help:      "Age of the newest CEX ref in an admitted CEX event",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxCEXEventHandoffOldestAge = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "cex_event_handoff_oldest_age_seconds",
			Help:      "Age of the oldest CEX ref in an admitted CEX event",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id", "chain_id"},
	)
	MdbxCEXEventHandoffRefAge = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "mdbx",
			Name:      "cex_event_handoff_ref_age_seconds",
			Help:      "Age of each CEX ref in an admitted CEX event split by exchange and symbol",
			Buckets:   prometheus.ExponentialBuckets(0.0005, 2, 18),
		},
		[]string{"validator_id", "chain_id", "exchange", "symbol"},
	)
)

func init() {
	prometheus.MustRegister(
		ProcessedBlocks,
		ProcessedTransactions,
		BlockProcessingDuration,
		BlockPhaseDuration,
		BlockAbortedTotal,
		BatchProcessingDuration,
		BatchTransactions,
		BatchExternalBlocks,
		BatchCheckpoints,
		BatchCEXOrderBookRefs,
		BatchCommitObserverCalls,
		EventStreamBlockingDuration,

		HeadBlockNumber, EventStreamPosition,
		EventBatchEvents, BlockExternalTxs, BlockInternalTxs, BlockBytes,
		StreamReadDuration,

		MdbxWaitCyclesTotal, MdbxWaitTimeSeconds, MdbxTxLookupDuration,
		MdbxTxBatchesExpectedTotal, MdbxTxBatchesFoundTotal, MdbxEventParseDuration,
		MdbxCEXEventHandoffRefs,
		MdbxCEXEventHandoffDecodeDuration,
		MdbxCEXEventHandoffNewestAge,
		MdbxCEXEventHandoffOldestAge,
		MdbxCEXEventHandoffRefAge,
	)
}
