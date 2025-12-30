package gosdk

import (
	"context"
	"fmt"
	"os"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

type InitConfig struct {
	ChainID        *uint64              `yaml:"chain_id,omitempty"`
	DataDir        string               `yaml:"data_dir,omitempty"`
	EmitterPort    string               `yaml:"emitter_port,omitempty"`
	RPCPort        string               `yaml:"rpc_port,omitempty"`
	LogLevel       int                  `yaml:"log_level,omitempty"`
	RequiredChains []uint64             `yaml:"required_chains,omitempty"`
	PrometheusPort string               `yaml:"prometheus_port,omitempty"`
	ValidatorID    string               `yaml:"validator_id,omitempty"`
	CustomTables   kv.TableCfg          `yaml:"-"`
	CustomPaths    *AppchainCustomPaths `yaml:"custom_paths,omitempty"`
}

type AppchainCustomPaths struct {
	MultichainDir   string `yaml:"multichain_dir,omitempty"`
	AppchainDBDir   string `yaml:"appchain_db_dir,omitempty"`
	TxPoolDir       string `yaml:"txpool_dir,omitempty"`
	EventsStreamDir string `yaml:"events_stream_dir,omitempty"`
	TxBatchDir      string `yaml:"txbatch_dir,omitempty"`
}

type AppInit[AppTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	Storage *Storage[AppTx, R]
	Config  *AppchainConfig
}

func (a *AppInit[AppTx, R]) Close() {
	if a.Storage != nil {
		a.Storage.Close()
	}
}

func LoadConfig(path string) (*InitConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg InitConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func SetupLogger(ctx context.Context, logLevel int) context.Context {
	if logLevel == 0 {
		logLevel = int(zerolog.InfoLevel)
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).
		Level(zerolog.Level(logLevel))

	return log.Logger.WithContext(ctx)
}

func InitApp[AppTx apptypes.AppTransaction[R], R apptypes.Receipt](
	ctx context.Context,
	cfg InitConfig,
) (*AppInit[AppTx, R], error) {
	chainID := DefaultAppchainID
	if cfg.ChainID != nil {
		chainID = *cfg.ChainID
	}

	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}

	if cfg.EmitterPort == "" {
		cfg.EmitterPort = DefaultEmitterPort
	}

	if cfg.RPCPort == "" {
		cfg.RPCPort = DefaultRPCPort
	}

	logger := log.Ctx(ctx)

	eventStreamDir := EventsPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.EventsStreamDir != "" {
		eventStreamDir = cfg.CustomPaths.EventsStreamDir
	}

	txStreamDir := TxBatchPathForChain(cfg.DataDir, chainID)
	if cfg.CustomPaths != nil && cfg.CustomPaths.TxBatchDir != "" {
		txStreamDir = cfg.CustomPaths.TxBatchDir
	}

	config := &AppchainConfig{
		ChainID:        chainID,
		DataDir:        cfg.DataDir,
		EmitterPort:    cfg.EmitterPort,
		RPCPort:        cfg.RPCPort,
		PrometheusPort: cfg.PrometheusPort,
		ValidatorID:    cfg.ValidatorID,
		Logger:         logger,
	}

	storage := &Storage[AppTx, R]{
		eventStreamDir: eventStreamDir,
		txStreamDir:    txStreamDir,
	}

	var success bool

	defer func() {
		if !success {
			storage.Close()
		}
	}()

	multichainConfig := make(MultichainConfig)

	multichainRoot := MultichainPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.MultichainDir != "" {
		multichainRoot = cfg.CustomPaths.MultichainDir
	}

	for _, chainID := range cfg.RequiredChains {
		chainType := apptypes.ChainType(chainID)
		if !IsEvmChain(chainType) && !IsSolanaChain(chainType) {
			return nil, fmt.Errorf("chain ID %d: %w", chainID, ErrUnknownChain)
		}

		chainPath := MultichainChainPath(multichainRoot, chainID)
		multichainConfig[chainType] = chainPath

		logger.Info().
			Uint64("chainID", chainID).
			Str("path", chainPath).
			Msg("Waiting for external chain data...")

		if err := WaitFile(ctx, chainPath, logger); err != nil {
			return nil, fmt.Errorf("chain %d not available: %w", chainID, err)
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
			return nil, fmt.Errorf("failed to create multichain db: %w", err)
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
		return nil, fmt.Errorf("failed to create appchain db directory: %w", err)
	}

	var err error

	storage.appchainDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(appchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return tables
		}).Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open appchain database: %w", err)
	}

	storage.subscriber, err = NewSubscriber(ctx, storage.appchainDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	txPoolDir := TxPoolPath(cfg.DataDir)
	if cfg.CustomPaths != nil && cfg.CustomPaths.TxPoolDir != "" {
		txPoolDir = cfg.CustomPaths.TxPoolDir
	}

	err = os.MkdirAll(txPoolDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create txpool directory: %w", err)
	}

	storage.txPoolDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(txPoolDir).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open txpool database: %w", err)
	}

	storage.txPool = txpool.NewTxPool[AppTx](storage.txPoolDB)

	storage.txBatchDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(storage.txStreamDir).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return TxBucketsTables()
		}).
		Readonly().Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open txbatch database: %w", err)
	}

	logger.Info().
		Uint64("chainID", chainID).
		Str("dataDir", cfg.DataDir).
		Msg("Appchain initialization complete")

	success = true

	return &AppInit[AppTx, R]{
		Storage: storage,
		Config:  config,
	}, nil
}
