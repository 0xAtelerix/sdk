package gosdk

const (
	checkpointBucket = "checkpoints"
	externalTxBucket = "external_transactions"
	blocksBucket     = "blocks"
	configBucket     = "config" // last block number, hash, chainID, configHash, generisHash/infohash
	lastBlockKey     = "last_block"
	stateBucket      = "state"
)
