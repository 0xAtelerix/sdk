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
// Returns: {dataDir}/consensus
func ConsensusPath(dataDir string) string {
	return filepath.Join(dataDir, ConsensusDirName)
}

// AppchainPath returns the base appchain directory.
// Returns: {dataDir}/appchain
func AppchainPath(dataDir string) string {
	return filepath.Join(dataDir, AppchainDirName)
}

// MultichainPath returns the base multichain directory.
// Returns: {dataDir}/multichain
func MultichainPath(dataDir string) string {
	return filepath.Join(dataDir, MultichainDirName)
}

// ============================================================================
// Multichain Paths (external chain data)
// ============================================================================

// MultichainChainPath returns the database path for a specific external chain.
// Returns: {multichainRoot}/{chainID}
// Note: Accepts resolved root (with custom path handling) to support multiple chains.
func MultichainChainPath(multichainRoot string, chainID uint64) string {
	return filepath.Join(multichainRoot, strconv.FormatUint(chainID, 10))
}

// MultichainIndexDBPath returns the multichain oracle index database path.
// Returns: {dataDir}/multichain/index-db
// Used by: core/multichain oracle for tracking external chain state.
func MultichainIndexDBPath(dataDir string) string {
	return filepath.Join(MultichainPath(dataDir), IndexDBDirName)
}

// ============================================================================
// Appchain Paths (used by appchain implementations)
// ============================================================================

// EventsPath returns the consensus events directory.
// Returns: {dataDir}/consensus/events
// Used by: appchains to read consensus snapshots and validator sets.
func EventsPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), EventsDirName)
}

// TxBatchPath returns the base transaction batch directory.
// Returns: {dataDir}/consensus/txbatch
// Used by: pelacli fetcher (writes to {base}/{chainID}/ subdirectories).
// Note: Appchains should use TxBatchPathForChain() to read from chain-specific subdirectory.
func TxBatchPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), TxBatchDirName)
}

// TxBatchPathForChain returns the transaction batch path for a specific appchain.
// Returns: {dataDir}/consensus/txbatch/{chainID}
// Used by: appchains to read their transaction batches (written by fetcher).
// Note: For custom paths, use CustomPaths.TxBatchDir directly (full path, no chain ID needed).
func TxBatchPathForChain(dataDir string, chainID uint64) string {
	return filepath.Join(TxBatchPath(dataDir), strconv.FormatUint(chainID, 10))
}

// AppchainDBPath returns the main appchain database path.
// Returns: {dataDir}/appchain/db
// Used by: appchains to store blocks, state, and receipts.
func AppchainDBPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), AppchainDBDirName)
}

// TxPoolPath returns the transaction pool database path.
// Returns: {dataDir}/appchain/txpool
// Used by: appchains to store pending transactions.
func TxPoolPath(dataDir string) string {
	return filepath.Join(AppchainPath(dataDir), TxPoolDirName)
}

// ============================================================================
// Pelacli Paths (consensus layer services)
// ============================================================================

// ConsensusFetcherDBPath returns the fetcher database path.
// Returns: {dataDir}/consensus/fetcher-db
// Used by: pelacli fetcher to track fetched transactions and checkpoints.
func ConsensusFetcherDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), FetcherDirName)
}

// ============================================================================
// Core Node Paths (validator node services)
// ============================================================================

// ConsensusNodeDBPath returns the consensus node database path.
// Returns: {dataDir}/consensus/node-db
// Used by: core/node for validator consensus state.
func ConsensusNodeDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), NodeDBDirName)
}

// ============================================================================
// Core Signer Paths
// ============================================================================

// SignerDBPath returns the signer database path.
// Returns: {dataDir}/consensus/signer-db
// Used by: core/signer for managing validator keys and signatures.
func SignerDBPath(dataDir string) string {
	return filepath.Join(ConsensusPath(dataDir), SignerDirName)
}
