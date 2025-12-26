package scheme

import (
	"github.com/ledgerwatch/erigon-lib/kv"
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

	SubscriptionBucket             = "subscription_bucket"          // chainID -> []{address|contract|topic}
	SubscriptionEventLibraryBucket = "subscription_kinds_bucket"    // chainID|sha(contract|eventName) -> event
	ValsetBucket                   = "validator_set_bucket"         // Epoch -> map[validatorID][stake] // map[uint32][uint64]
	ExternalBlockVotingBucket      = "external_block_voting_bucket" // Chain|Block|Hash -> {votedStake, []{Epoch, Signer}}
	CheckpointVotingBucket         = "checkpoint_voting_bucket"     // Chain|Block|Hash -> {votedStake, []{Epoch, Signer}}

	TxBuckets     = "txbatch"
	ReceiptBucket = "receipts" // tx-hash -> receipt

	// Multichain
	ChainIDBucket = "chainid"
	EvmBlocks     = "blocks"
	EvmReceipts   = "ethereum_receipts"
	SolanaBlocks  = "solana_blocks"

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
		ReceiptBucket:                  {},
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
