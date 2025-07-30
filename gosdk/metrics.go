package gosdk

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ProcessedBlocks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "processed_blocks_total",
			Help:      "Total number of processed blocks",
		},
		[]string{"validator_id"},
	)

	ProcessedTransactions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "processed_transactions_total",
			Help:      "Total number of processed transactions",
		},
		[]string{"validator_id"},
	)

	BlockProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "block_processing_duration_seconds",
			Help:      "Histogram of block processing durations",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id"},
	)
	BatchProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "batch_processing_duration_seconds",
			Help:      "Time to process a single batch (excluding blocking)",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id"},
	)
	EventStreamBlockingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "appchain",
			Subsystem: "run",
			Name:      "get_batches_duration",
			Help:      "Time spent blocking on event stream batch retrieval",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"validator_id"},
	)
)

func init() {
	prometheus.MustRegister(
		ProcessedBlocks,
		ProcessedTransactions,
		BlockProcessingDuration,
		BatchProcessingDuration,
		EventStreamBlockingDuration,
	)
}
