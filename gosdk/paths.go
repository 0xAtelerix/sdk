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
// Base Directory Paths
// ============================================================================

// ConsensusPath returns the base consensus directory.
// All consensus-related subdirectories are under this path.
func ConsensusPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName)
}

// AppchainPath returns the base appchain directory.
// All appchain-specific subdirectories are under this path.
func AppchainPath(dataDir string) string {
	return filepath.Join(dataDir, AppchainDirName)
}

// MultichainPath returns the base multichain directory.
// All external chain data subdirectories are under this path.
func MultichainPath(dataDir string) string {
	return filepath.Join(dataDir, MultichainDirName)
}

// ============================================================================
// Appchain Paths (used by appchain implementations)
// ============================================================================

// MultichainChainPath returns the database path for a specific external chain (EVM/Solana).
func MultichainChainPath(multichainRoot string, chainID uint64) string {
	return filepath.Join(multichainRoot, strconv.FormatUint(chainID, 10))
}

// EventsPath returns the consensus events directory (snapshots, validator sets).
// Appchains read consensus events from this directory.
func EventsPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), EventsDirName)
}

// TxBatchPath returns the base transaction batch directory.
// Used by: pelacli fetcher (writes to {base}/{chainID}/ subdirectories).
// Note: Appchains should use TxBatchPathForChain() to read from chain-specific subdirectory.
func TxBatchPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), TxBatchDirName)
}

// TxBatchPathForChain returns the transaction batch path for a specific appchain.
// Returns: {txBatchRoot}/{chainID}/
// Used by: appchains to read their transaction batches (written by fetcher).
// Note: Pass the resolved txBatch base path (with custom path handling) to support custom paths.
func TxBatchPathForChain(txBatchRoot string, chainID uint64) string {
	return filepath.Join(txBatchRoot, strconv.FormatUint(chainID, 10))
}

// AppchainDBPath returns the main appchain database path (blocks, state, receipts).
func AppchainDBPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), AppchainDBDirName)
}

// TxPoolPath returns the transaction pool database path.
func TxPoolPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), TxPoolDirName)
}

// ============================================================================
// Pelacli Paths (consensus layer services)
// ============================================================================

// ConsensusFetcherDBPath returns the fetcher database path.
// Used by: pelacli fetcher to track fetched transactions and checkpoints.
func ConsensusFetcherDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), FetcherDirName)
}

// ============================================================================
// Core Node Paths (validator node services)
// ============================================================================

// ConsensusNodeDBPath returns the consensus node database path.
// Used by: core/node for validator consensus state.
func ConsensusNodeDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), NodeDBDirName)
}

// ============================================================================
// Core Multichain Oracle Paths
// ============================================================================

// MultichainIndexDBPath returns the multichain oracle index database path.
// Used by: core/multichain oracle for tracking external chain state.
func MultichainIndexDBPath(dataDir string) string {
	return filepath.Join(MultichainPath(dataDir), IndexDBDirName)
}

// ============================================================================
// Core Signer Paths
// ============================================================================

// SignerDBPath returns the signer database path.
// Used by: core/signer for managing validator keys and signatures.
func SignerDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), SignerDirName)
}
