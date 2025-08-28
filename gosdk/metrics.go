package gosdk

import "github.com/prometheus/client_golang/prometheus"

//nolint:gochecknoglobals // metrics - it's easier to use global vars then put everything into context as a container
var (
	// bacis
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

	// latency and progress
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

	// size and structure of batches and blocks
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

	// FSNotify/IO
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

	// MDBX wrapper
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
)

func init() {
	prometheus.MustRegister(
		ProcessedBlocks,
		ProcessedTransactions,
		BlockProcessingDuration,
		BatchProcessingDuration,
		EventStreamBlockingDuration,

		HeadBlockNumber, EventStreamPosition,
		EventBatchEvents, BlockExternalTxs, BlockInternalTxs, BlockBytes,
		StreamReadDuration,

		MdbxWaitCyclesTotal, MdbxWaitTimeSeconds, MdbxTxLookupDuration,
		MdbxTxBatchesExpectedTotal, MdbxTxBatchesFoundTotal, MdbxEventParseDuration,
	)
}
