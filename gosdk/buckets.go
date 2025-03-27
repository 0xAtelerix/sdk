package gosdk

import "github.com/ledgerwatch/erigon-lib/kv"

const (
	checkpointBucket       = "checkpoints"
	externalTxBucket       = "external_transactions"
	blocksBucket           = "blocks"
	configBucket           = "config" // last block number, hash, chainID, configHash, generisHash/infohash
	lastBlockKey           = "last_block"
	stateBucket            = "state"
	snapshot               = "snapshot"
	processed_buckets      = "processed_buckets"
	eventStreamPositionKey = "event_stream_pos"
)

var defaultTables = kv.TableCfg{
	checkpointBucket: {},
	externalTxBucket: {},
	blocksBucket:     {},
	configBucket:     {},
	stateBucket:      {},
	snapshot:         {},
}
