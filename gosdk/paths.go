package gosdk

import (
	"path/filepath"
	"strconv"
)

const (
	MultichainDirName = "multichain" // external chain data directory

	ConsensusDirName = "consensus" // Consensus data directory
	EventsDirName    = "events"    // consensus/events for consensus events
	TxBatchDirName   = "txbatch"   // consensus/txbatch for transaction batching

	AppchainDirName   = "appchain" // appchain-specific data directory
	AppchainDBDirName = "db"       // appchain/db for appchain database
	TxPoolDirName     = "txpool"   // appchain/txpool for transaction pool
)

// Multichain paths.
func MultichainPath(dataDir string) string {
	return filepath.Join(dataDir, MultichainDirName)
}

func MultichainChainPath(multichainRoot string, chainID uint64) string {
	return filepath.Join(multichainRoot, strconv.FormatUint(chainID, 10))
}

// Consensus paths.
func ConsensusPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName)
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

// Appchain paths.
func AppchainPath(dataDir string) string {
	return filepath.Join(dataDir, AppchainDirName)
}

func AppchainDBPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), AppchainDBDirName)
}

func TxPoolPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), TxPoolDirName)
}
