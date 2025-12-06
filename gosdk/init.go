package gosdk

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

// ============================================================================
// Defaults
// ============================================================================

// Default data directory for both Docker and local deployments.
const DefaultDataDir = "./data"

// Default ports and chainID for appchain services.
const (
	// DefaultEmitterPort is the gRPC port for emitter API.
	// Used by: fetcher (pelacli/core) to fetch checkpoints, txbatches, external txs.
	DefaultEmitterPort = ":9090"

	// DefaultRPCPort is the HTTP port for JSON-RPC server.
	// Used by: external clients to query blocks, txs, receipts.
	DefaultRPCPort = ":8080"

	// DefaultAppchainID is the default chain ID for local development.
	// Production appchains should use unique chain IDs.
	DefaultAppchainID uint64 = 42
)

// Directory names within the data directory.
const (
	// MultichainDirName stores external chain data (EVM, Solana blocks).
	// Written by: pelacli/core (multichain oracle)
	// Read by: appchain (via MultichainStateAccessor)
	MultichainDirName = "multichain"

	// EventsDirName stores consensus events/snapshots.
	// Written by: pelacli/core (consensus stub)
	// Read by: appchain (via MdbxEventStreamWrapper)
	EventsDirName = "events"

	// AppchainDirName stores appchain state (blocks, checkpoints, receipts).
	// Written by: appchain
	// Read by: appchain, RPC server
	AppchainDirName = "appchain"

	// FetcherDirName stores fetcher data including transaction batches.
	// Written by: pelacli/core (fetcher WriterLoop)
	// Read by: appchain (to process batches)
	FetcherDirName = "fetcher"

	// LocalDirName stores node-local data like txpool.
	// Written by: appchain (txpool)
	// Read by: appchain, emitter API (exposes to fetcher via gRPC)
	LocalDirName = "local"
)

// ============================================================================
// Path Helpers
// ============================================================================

// MultichainPath returns the multichain directory for a given base path.
func MultichainPath(dataDir string) string {
	return filepath.Join(dataDir, MultichainDirName)
}

// EventsPath returns the events directory for a given base path.
func EventsPath(dataDir string) string {
	return filepath.Join(dataDir, EventsDirName)
}

// ChainDBPath returns the database path for a specific chain ID.
func ChainDBPath(dataDir string, chainID uint64) string {
	return filepath.Join(dataDir, MultichainDirName, strconv.FormatUint(chainID, 10))
}

// AppchainDBPath returns the appchain database directory for a specific chain.
func AppchainDBPath(dataDir string, chainID uint64) string {
	return filepath.Join(dataDir, AppchainDirName, strconv.FormatUint(chainID, 10))
}

// TxBatchPath returns the transaction batch directory for an appchain.
func TxBatchPath(dataDir string, chainID uint64) string {
	return filepath.Join(dataDir, FetcherDirName, strconv.FormatUint(chainID, 10))
}

// LocalDBPath returns the local database directory for an appchain (for txpool).
func LocalDBPath(dataDir string, chainID uint64) string {
	return filepath.Join(dataDir, LocalDirName, strconv.FormatUint(chainID, 10))
}

// DiscoverMultichainDBsInDir scans a directory for multichain databases.
// Only includes directories that contain actual database files (SQLite or MDBX).
func DiscoverMultichainDBsInDir(dir string) (MultichainConfig, error) {
	config := make(MultichainConfig)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}

		return nil, fmt.Errorf("failed to read multichain directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		chainID, err := strconv.ParseUint(entry.Name(), 10, 64)
		if err != nil {
			continue
		}

		dbPath := filepath.Join(dir, entry.Name())

		// Check if SQLite database file exists (preferred)
		sqliteFile := filepath.Join(dbPath, "sqlite")
		if _, err := os.Stat(sqliteFile); err == nil {
			config[apptypes.ChainType(chainID)] = dbPath
			continue
		}

		// Fallback: check for MDBX database file
		mdbxFile := filepath.Join(dbPath, "mdbx.dat")
		if _, err := os.Stat(mdbxFile); err == nil {
			config[apptypes.ChainType(chainID)] = dbPath
			continue
		}

		// Skip directories without database files
	}

	return config, nil
}

// ============================================================================
// Initialization
// ============================================================================

// InitConfig holds configuration for initializing an appchain.
type InitConfig struct {
	// ChainID is the unique identifier for this appchain.
	ChainID uint64

	// DataDir is the root data directory (shared with pelacli).
	// Defaults to /data in Docker deployments.
	DataDir string

	// EmitterPort is the gRPC port for the emitter API.
	// Defaults to :9090.
	EmitterPort string

	// CustomTables are additional MDBX tables for the appchain.
	// These are merged with SDK's default tables.
	CustomTables kv.TableCfg

	// Logger is an optional custom logger.
	Logger *zerolog.Logger
}

// InitResult contains all initialized components for an appchain.
type InitResult struct {
	// Config is the fully populated appchain config.
	Config AppchainConfig

	// AppchainDB is the main appchain database.
	AppchainDB kv.RwDB

	// LocalDB is the database for transaction pool.
	LocalDB kv.RwDB

	// TxBatchDB is the read-only database for transaction batches (written by pelacli).
	TxBatchDB kv.RoDB

	// Multichain is the accessor for external chain data (SQLite or MDBX).
	Multichain MultichainStateAccessor

	// Subscriber manages external chain subscriptions.
	Subscriber *Subscriber
}

// Close releases all resources held by the init result.
func (r *InitResult) Close() {
	if r.Multichain != nil {
		r.Multichain.Close()
	}

	if r.TxBatchDB != nil {
		r.TxBatchDB.Close()
	}

	if r.LocalDB != nil {
		r.LocalDB.Close()
	}

	if r.AppchainDB != nil {
		r.AppchainDB.Close()
	}
}

// Init initializes all common appchain components with sensible defaults.
// This eliminates boilerplate and provides a consistent setup across appchains.
//
// Example usage:
//
//	result, err := gosdk.Init(ctx, gosdk.InitConfig{
//	    ChainID:      42,
//	    CustomTables: myapp.Tables(),
//	})
//	if err != nil {
//	    log.Fatal().Err(err).Msg("Failed to init")
//	}
//	defer result.Close()
func Init(ctx context.Context, cfg InitConfig) (*InitResult, error) {
	// Apply defaults
	if cfg.ChainID == 0 {
		cfg.ChainID = DefaultAppchainID
	}

	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}

	if cfg.EmitterPort == "" {
		cfg.EmitterPort = DefaultEmitterPort
	}

	logger := &log.Logger
	if cfg.Logger != nil {
		logger = cfg.Logger
	}

	result := &InitResult{}

	// Auto-discover multichain databases
	multichainConfig, err := DiscoverMultichainDBsInDir(MultichainPath(cfg.DataDir))
	if err != nil {
		logger.Warn().
			Err(err).
			Msg("Failed to discover multichain DBs, continuing with empty config")

		multichainConfig = make(MultichainConfig)
	}

	logger.Info().Int("chains", len(multichainConfig)).Msg("Discovered multichain databases")

	// Create appchain config
	result.Config = AppchainConfig{
		ChainID:           cfg.ChainID,
		EmitterPort:       cfg.EmitterPort,
		AppchainDBPath:    AppchainDBPath(cfg.DataDir, cfg.ChainID),
		EventStreamDir:    EventsPath(cfg.DataDir),
		TxStreamDir:       TxBatchPath(cfg.DataDir, cfg.ChainID),
		MultichainStateDB: multichainConfig,
		Logger:            logger,
	}

	// Initialize multichain state accessor (SQLite-based)
	chainDBs, err := NewMultichainStateAccessSQLDB(ctx, multichainConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create multichain db: %w", err)
	}

	result.Multichain = NewMultichainStateAccessSQL(chainDBs)

	// Initialize appchain database
	tables := DefaultTables()
	if cfg.CustomTables != nil {
		tables = MergeTables(tables, cfg.CustomTables)
	}

	// Initialize appchain database for state
	result.AppchainDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(result.Config.AppchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return tables
		}).Open()
	if err != nil {
		result.Close()

		return nil, fmt.Errorf("failed to open appchain database: %w", err)
	}

	// Initialize subscriber
	result.Subscriber, err = NewSubscriber(ctx, result.AppchainDB)
	if err != nil {
		result.Close()

		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	// Initialize local database for transaction pool
	result.LocalDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(LocalDBPath(cfg.DataDir, cfg.ChainID)).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	if err != nil {
		result.Close()

		return nil, fmt.Errorf("failed to open local database: %w", err)
	}

	// Open transaction batch database (read-only)
	// This database is created by pelacli's fetcher
	result.TxBatchDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(result.Config.TxStreamDir).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return TxBucketsTables()
		}).
		Readonly().Open()
	if err != nil {
		result.Close()

		return nil, fmt.Errorf(
			"failed to open tx batch database at %s: %w",
			result.Config.TxStreamDir,
			err,
		)
	}

	return result, nil
}

// InitDevValidatorSet initializes a single-validator set for local development.
// Call this from your application for local (pelacli) testing only.
// On testnet/mainnet, the validator set comes from the consensus layer.
func InitDevValidatorSet(ctx context.Context, db kv.RwDB) error {
	valset := &ValidatorSet{Set: map[ValidatorID]Stake{0: 100}}

	var epochKey [4]byte
	binary.BigEndian.PutUint32(epochKey[:], 1)

	valsetData, err := cbor.Marshal(valset)
	if err != nil {
		return fmt.Errorf("failed to marshal validator set: %w", err)
	}

	return db.Update(ctx, func(tx kv.RwTx) error {
		return tx.Put(ValsetBucket, epochKey[:], valsetData)
	})
}
