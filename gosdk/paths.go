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

// ============================================================================
// Appchain Paths (used by appchain implementations)
// ============================================================================

// MultichainPath returns the root directory for all external chain data.
func MultichainPath(dataDir string) string {
	return filepath.Join(dataDir, MultichainDirName)
}

// MultichainChainPath returns the database path for a specific external chain (EVM/Solana).
func MultichainChainPath(multichainRoot string, chainID uint64) string {
	return filepath.Join(multichainRoot, strconv.FormatUint(chainID, 10))
}

// EventsPath returns the consensus events directory (snapshots, validator sets).
func EventsPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName, EventsDirName)
}

// TxBatchPath returns the transaction batch directory (written by pelacli fetcher).
func TxBatchPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName, TxBatchDirName)
}

// AppchainDBPath returns the main appchain database path (blocks, state, receipts).
func AppchainDBPath(dataDir string) string {
	return filepath.Join(dataDir, AppchainDirName, AppchainDBDirName)
}

// TxPoolPath returns the transaction pool database path.
func TxPoolPath(dataDir string) string {
	return filepath.Join(dataDir, AppchainDirName, TxPoolDirName)
}

// ============================================================================
// Pelacli Paths (consensus layer services)
// ============================================================================

// ConsensusFetcherDBPath returns the fetcher database path.
// Used by: pelacli fetcher to track fetched transactions and checkpoints.
func ConsensusFetcherDBPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName, FetcherDirName)
}

// ============================================================================
// Core Node Paths (validator node services)
// ============================================================================

// ConsensusNodeDBPath returns the consensus node database path.
// Used by: core/node for validator consensus state.
func ConsensusNodeDBPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName, NodeDBDirName)
}

// ============================================================================
// Core Multichain Oracle Paths
// ============================================================================

// MultichainIndexDBPath returns the multichain oracle index database path.
// Used by: core/multichain oracle for tracking external chain state.
func MultichainIndexDBPath(dataDir string) string {
	return filepath.Join(dataDir, MultichainDirName, IndexDBDirName)
}

// ============================================================================
// Core Signer Paths
// ============================================================================

// SignerDBPath returns the signer database path.
// Used by: core/signer for managing validator keys and signatures.
func SignerDBPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName, SignerDirName)
}

// ============================================================================
// Multi-Appchain Paths (for cluster deployments)
// ============================================================================

// TxBatchPathForChain returns the transaction batch path for a specific appchain.
// Used by: appchains in multi-appchain clusters (e.g., core/simpleappchain).
// For single appchain deployments, use TxBatchPath instead.
func TxBatchPathForChain(dataDir string, chainID uint64) string {
	return filepath.Join(dataDir, ConsensusDirName, TxBatchDirName, strconv.FormatUint(chainID, 10))
}
