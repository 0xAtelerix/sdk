package gosdk

import (
	"context"
	"fmt"
	"os"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

type InitConfig struct {
	ChainID     uint64 // Defaults to 42
	DataDir     string // Defaults to /data
	EmitterPort string // Defaults to :9090

	RequiredChains []uint64 // External chain IDs to wait for

	PrometheusPort string               // Optional, leave empty to disable
	ValidatorID    string               // Optional, for multi-validator metrics/logs
	CustomTables   kv.TableCfg          // Optional, merged with default tables
	CustomPaths    *AppchainCustomPaths // Optional, overrides DataDir-derived paths
}

// AppchainCustomPaths overrides default directory paths.
// Optional - if not set, paths are derived from DataDir.
type AppchainCustomPaths struct {
	MultichainDir   string // Default: {DataDir}/multichain
	AppchainDBDir   string // Default: {DataDir}/appchain/db
	TxPoolDir       string // Default: {DataDir}/appchain/txpool
	EventsStreamDir string // Default: {DataDir}/consensus/events
	TxBatchDir      string // Default: {DataDir}/consensus/txbatch
}

// InitApp initializes the appchain storage and configuration.
func InitApp[AppTx apptypes.AppTransaction[R], R apptypes.Receipt](
	ctx context.Context,
	cfg InitConfig,
) (*Storage[AppTx, R], *AppchainConfig, error) {
	if cfg.ChainID == 0 {
		cfg.ChainID = DefaultAppchainID
	}

	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}

	if cfg.EmitterPort == "" {
		cfg.EmitterPort = DefaultEmitterPort
	}

	logger := log.Ctx(ctx)

	eventStreamDir := EventsPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.EventsStreamDir != "" {
		eventStreamDir = cfg.CustomPaths.EventsStreamDir
	}

	txStreamDir := TxBatchPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.TxBatchDir != "" {
		txStreamDir = cfg.CustomPaths.TxBatchDir
	}

	config := &AppchainConfig{
		ChainID:        cfg.ChainID,
		DataDir:        cfg.DataDir,
		EmitterPort:    cfg.EmitterPort,
		PrometheusPort: cfg.PrometheusPort,
		ValidatorID:    cfg.ValidatorID,
		Logger:         logger,
	}

	storage := &Storage[AppTx, R]{
		eventStreamDir: eventStreamDir,
		txStreamDir:    txStreamDir,
	}

	multichainConfig := make(MultichainConfig)

	multichainRoot := MultichainPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.MultichainDir != "" {
		multichainRoot = cfg.CustomPaths.MultichainDir
	}

	for _, chainID := range cfg.RequiredChains {
		chainType := apptypes.ChainType(chainID)
		if !IsEvmChain(chainType) && !IsSolanaChain(chainType) {
			return nil, nil, fmt.Errorf("unsupported chain ID %d: not a recognized chain", chainID)
		}

		chainPath := MultichainChainPath(multichainRoot, chainID)
		multichainConfig[chainType] = chainPath

		logger.Info().
			Uint64("chainID", chainID).
			Str("path", chainPath).
			Msg("Waiting for external chain data...")

		if err := WaitFile(ctx, chainPath, logger); err != nil {
			storage.Close()

			return nil, nil, fmt.Errorf("chain %d not available: %w", chainID, err)
		}

		logger.Info().
			Uint64("chainID", chainID).
			Msg("External chain data available")
	}

	logger.Info().
		Int("chains", len(multichainConfig)).
		Uints64("chainIDs", cfg.RequiredChains).
		Msg("Multichain databases configured")

	if len(multichainConfig) > 0 {
		chainDBs, err := NewMultichainStateAccessSQLDB(ctx, multichainConfig)
		if err != nil {
			storage.Close()

			return nil, nil, fmt.Errorf("failed to create multichain db: %w", err)
		}

		storage.multichain = NewMultichainStateAccessSQL(chainDBs)
	}

	tables := DefaultTables()
	if cfg.CustomTables != nil {
		tables = MergeTables(tables, cfg.CustomTables)
	}

	appchainDBPath := AppchainDBPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.AppchainDBDir != "" {
		appchainDBPath = cfg.CustomPaths.AppchainDBDir
	}

	if err := os.MkdirAll(appchainDBPath, 0o755); err != nil {
		storage.Close()

		return nil, nil, fmt.Errorf("failed to create appchain db directory: %w", err)
	}

	var err error

	storage.appchainDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(appchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return tables
		}).Open()
	if err != nil {
		storage.Close()

		return nil, nil, fmt.Errorf("failed to open appchain database: %w", err)
	}

	storage.subscriber, err = NewSubscriber(ctx, storage.appchainDB)
	if err != nil {
		storage.Close()

		return nil, nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	txPoolDir := TxPoolPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.TxPoolDir != "" {
		txPoolDir = cfg.CustomPaths.TxPoolDir
	}

	err = os.MkdirAll(txPoolDir, 0o755)
	if err != nil {
		storage.Close()

		return nil, nil, fmt.Errorf("failed to create txpool directory: %w", err)
	}

	storage.txPoolDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(txPoolDir).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	if err != nil {
		storage.Close()

		return nil, nil, fmt.Errorf("failed to open txpool database: %w", err)
	}

	storage.txPool = txpool.NewTxPool[AppTx](storage.txPoolDB)

	storage.txBatchDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(storage.txStreamDir).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return TxBucketsTables()
		}).
		Readonly().Open()
	if err != nil {
		storage.Close()

		return nil, nil, fmt.Errorf("failed to open txbatch database: %w", err)
	}

	logger.Info().
		Uint64("chainID", cfg.ChainID).
		Str("dataDir", cfg.DataDir).
		Msg("Appchain initialization complete")

	return storage, config, nil
}
