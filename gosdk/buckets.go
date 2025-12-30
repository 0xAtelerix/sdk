package gosdk

import (
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/receipt"
)

const (
	CheckpointBucket = "checkpoints"
	ExternalTxBucket = "external_transactions"
	BlocksBucket     = "blocks"
	ConfigBucket     = "config" // last block number, hash, chainID, configHash, generisHash/infohash
	LastBlockKey     = "last_block"
	StateBucket      = "state"
	Snapshot         = "snapshot"
	TxSnapshot       = "tx_snapshot"

	// chainID -> []{address|contract}
	SubscriptionBucket = "subscription_bucket"

	// chainID|sha(contract|eventName) -> event	ExternalBlockVotingBucket   = "external_block_voting_bucket"
	// Chain|Block|Hash -> {votedStake, []{Epoch, Signer}}
	SubscriptionEventLibraryBucket = "subscription_kinds_bucket"

	ValsetBucket              = "validator_set_bucket"         // Epoch -> map[validatorID][stake] // map[uint32][uint64]
	ExternalBlockVotingBucket = "external_block_voting_bucket" // Chain|Block|Hash -> {votedStake, []{Epoch, Signer}}
	CheckpointVotingBucket    = "checkpoint_voting_bucket"     // Chain|Block|Hash -> {votedStake, []{Epoch, Signer}}

	TxBuckets = "txbatch"

	BlockTransactionsBucket = "block_transactions" // blockNumber -> []Transaction (CBOR encoded) [PRIMARY STORAGE]
	TxLookupBucket          = "tx_lookup"          // txHash -> (blockNumber[8 bytes], txIndex[4 bytes]) [INDEX]
)

func TxBucketsTables() kv.TableCfg {
	return kv.TableCfg{
		TxBuckets: {},
	}
}

func DefaultTables() kv.TableCfg {
	return kv.TableCfg{
		CheckpointBucket:               {},
		ExternalTxBucket:               {},
		BlocksBucket:                   {},
		ConfigBucket:                   {},
		StateBucket:                    {},
		Snapshot:                       {},
		receipt.ReceiptBucket:          {},
		EvmReceipts:                    {},
		SubscriptionBucket:             {},
		SubscriptionEventLibraryBucket: {},
		ValsetBucket:                   {},
		ExternalBlockVotingBucket:      {},
		CheckpointVotingBucket:         {},
		BlockTransactionsBucket:        {},
		TxLookupBucket:                 {},
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
