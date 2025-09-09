package gosdk

import "github.com/ledgerwatch/erigon-lib/kv"

const (
	CheckpointBucket = "checkpoints"
	ExternalTxBucket = "external_transactions"
	BlocksBucket     = "blocks"
	ConfigBucket     = "config" // last block number, hash, chainID, configHash, generisHash/infohash
	LastBlockKey     = "last_block"
	StateBucket      = "state"
	Snapshot         = "snapshot"
	TxSnapshot       = "tx_snapshot"
	ReceiptBucket    = "receipts" // tx-hash -> receipt

	processedBuckets       = "processed_buckets"
	eventStreamPositionKey = "event_stream_pos"

	subscriptionBucket = "subscription_bucket"  // chainID -> []{address|contract}
	ValsetBucket       = "validator_set_bucket" // Epoch -> map[validatorID][stake] // map[uint32][uint64]

	TxBuckets = "txbatch"
)

func TxBucketsTables() kv.TableCfg {
	return kv.TableCfg{
		TxBuckets: {},
	}
}

func DefaultTables() kv.TableCfg {
	return kv.TableCfg{
		CheckpointBucket:   {},
		ExternalTxBucket:   {},
		BlocksBucket:       {},
		ConfigBucket:       {},
		StateBucket:        {},
		Snapshot:           {},
		ReceiptBucket:      {},
		subscriptionBucket: {},
		ValsetBucket:       {},
	}
}

func MergeTables(bucketSets ...kv.TableCfg) kv.TableCfg {
	final := kv.TableCfg{}

	for _, buckets := range bucketSets {
		for i := range buckets {
			final[i] = buckets[i]
		}
	}

	return final
}
