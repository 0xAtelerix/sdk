package gosdk

import (
	"path/filepath"
	"strconv"
)

// Root directories.
const (
	MultichainDirName = "multichain" // External chain data (oracle writes, appchain reads)
	ConsensusDirName  = "consensus"  // Consensus layer data (core writes, appchain reads)
	AppchainDirName   = "appchain"   // Appchain-specific data
)

// Subdirectories under multichain/
const (
	IndexDBDirName = "index-db" // Multichain oracle index database
)

// Subdirectories under consensus/
const (
	EventsDirName  = "events"     // Consensus events (snapshots, validator sets)
	TxBatchDirName = "txbatch"    // Transaction batches from fetcher
	FetcherDirName = "fetcher-db" // Pelacli fetcher database
	NodeDBDirName  = "node-db"    // Consensus node database (validator state)
	SignerDirName  = "signer-db"  // Validator signer database
)

// Subdirectories under appchain/
const (
	AppchainDBDirName = "db"     // Main appchain database (blocks, state, receipts)
	TxPoolDirName     = "txpool" // Transaction pool database
)

func ConsensusPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName)
}

func AppchainPath(dataDir string) string {
	return filepath.Join(dataDir, AppchainDirName)
}

func MultichainPath(dataDir string) string {
	return filepath.Join(dataDir, MultichainDirName)
}

func MultichainChainPath(multichainRoot string, chainID uint64) string {
	return filepath.Join(multichainRoot, strconv.FormatUint(chainID, 10))
}

func MultichainIndexDBPath(dataDir string) string {
	return filepath.Join(MultichainPath(dataDir), IndexDBDirName)
}

func EventsPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), EventsDirName)
}

func TxBatchPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), TxBatchDirName)
}

func TxBatchPathForChain(dataDir string, chainID uint64) string {
	return filepath.Join(TxBatchPath(dataDir), strconv.FormatUint(chainID, 10))
}

func AppchainDBPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), AppchainDBDirName)
}

func TxPoolPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), TxPoolDirName)
}

func ConsensusFetcherDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), FetcherDirName)
}

func ConsensusNodeDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), NodeDBDirName)
}

func SignerDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), SignerDirName)
}
