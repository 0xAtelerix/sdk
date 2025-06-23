package gosdk

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
	"github.com/0xAtelerix/sdk/gosdk/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"net"
	"os"
	"path/filepath"
	"time"
)

func NewAppchain[STI StateTransitionInterface[AppTx],
	AppTx types.AppTransaction,
	AppBlock types.AppchainBlock](sti STI,
	blockBuilder types.AppchainBlockConstructor[AppTx, AppBlock],
	txpool types.TxPoolInterface[AppTx],
	config AppchainConfig, appchainDB kv.RwDB, options ...func(a *Appchain[STI, AppTx, AppBlock])) (Appchain[STI, AppTx, AppBlock], error) {

	log.Info().Str("db_path", config.AppchainDBPath).Msg("Initializing appchain database")

	emiterAPI := NewServer(appchainDB, config.ChainID, txpool)
	emiterAPI.logger = &log.Logger
	if config.Logger != nil {
		emiterAPI.logger = config.Logger
	}
	log.Info().Msg("Appchain initialized successfully")
	multichainDB, err := NewMultichainStateAccess(config.MultichainStateDB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize MultichainStateAccess")
		return Appchain[STI, AppTx, AppBlock]{}, err
	}
	appchain := Appchain[STI, AppTx, AppBlock]{
		appchainStateExecution: sti,
		rootCalculator:         NewStubRootCalculator(),
		blockBuilder:           blockBuilder,
		emiterAPI:              emiterAPI,
		AppchainDB:             appchainDB,
		config:                 config,
		multichainDB:           multichainDB,
	}
	for _, option := range options {
		option(&appchain)
	}

	return appchain, nil
}

func WithRootCalculator[STI StateTransitionInterface[AppTx],
	AppTx types.AppTransaction,
	AppBlock types.AppchainBlock](rc types.RootCalculator) func(a *Appchain[STI, AppTx, AppBlock]) {
	return func(a *Appchain[STI, AppTx, AppBlock]) {
		a.rootCalculator = rc
	}
}

type AppchainConfig struct {
	ChainID           uint64
	EmitterPort       string
	AppchainDBPath    string
	EventStreamDir    string
	TxStreamDir       string
	MultichainStateDB map[uint32]string
	Logger            *zerolog.Logger
}

// todo: it should be stored at the first run and checked on next
func MakeAppchainConfig(chainID uint64) AppchainConfig {
	return AppchainConfig{
		ChainID:        chainID,
		EmitterPort:    ":50051",
		AppchainDBPath: "chaindb",
		EventStreamDir: "epochs",
		TxStreamDir:    fmt.Sprintf("%d", chainID),
	}
}

type Appchain[STI StateTransitionInterface[appTx], appTx types.AppTransaction, AppBlock types.AppchainBlock] struct {
	appchainStateExecution STI
	rootCalculator         types.RootCalculator
	blockBuilder           types.AppchainBlockConstructor[appTx, AppBlock]

	emiterAPI    emitterproto.EmitterServer
	AppchainDB   kv.RwDB
	TxBatchDB    kv.RoDB
	config       AppchainConfig
	multichainDB *MultichainStateAccess
}

func (a *Appchain[STI, appTx, AppBlock]) Run(ctx context.Context) error {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Appchain run started")

	go a.RunEmitterAPI(ctx)

	startEventPos, startTxPos, err := GetLastStreamPositions(a.AppchainDB)
	if err != nil {
		return err
	}

	//todo надо открывать в run. Отсутствие файла - не должно быть причиной падения
	logger.Info().Str("dir", a.config.EventStreamDir).Int64("start event", startEventPos).Int64("start tx", startTxPos).Msg("Initializing event readers")
	for {
		_, err := os.Stat(a.config.EventStreamDir)
		if err != nil {
			logger.Warn().Err(err).Msg("waiting event stream file")
			time.Sleep(5 * time.Second)
			continue
		}
		_, err = os.Stat(a.config.TxStreamDir)
		if err != nil && a.TxBatchDB == nil {
			logger.Warn().Err(err).Msg("waiting tx stream file")
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	eventStream, err := NewMdbxEventStreamWrapper[appTx](filepath.Join(a.config.EventStreamDir, "epoch_0.data"),
		uint32(a.config.ChainID),
		startEventPos,
		a.TxBatchDB,
		logger,
	)
	//eventStream, err := NewEventStreamWrapper[appTx](filepath.Join(a.config.EventStreamDir, "epoch_0.data"),
	//	filepath.Join(a.config.TxStreamDir, "epoch_0_"+fmt.Sprintf("%d", a.config.ChainID)+"_tx.data"),
	//	uint32(a.config.ChainID),
	//	startEventPos, startTxPos,
	//)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create event stream")
		return fmt.Errorf("Failed to create event stream: %w", err)
	}

	var (
		previousBlockNumber uint64
		previousBlockHash   [32]byte
	)

	err = a.AppchainDB.View(ctx, func(tx kv.Tx) error {
		previousBlockNumber, previousBlockHash, err = GetLastBlock(tx)

		return err
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get last block")
		return fmt.Errorf("Failed to get last block: %w", err)
	}
	logger.Info().Uint64("previous block number", previousBlockNumber).Hex("hash", previousBlockHash[:]).Msg("load bn")

runFor:
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Appchain context cancelled, stopping")
			break runFor
		default:
		}
		logger.Debug().Msg("getting batches")
		batches, err := eventStream.GetNewBatchesBlocking(ctx, 10)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get new batches blocking")
			return fmt.Errorf("Failed to get new batch: %w", err)
		}

		if len(batches) == 0 {
			logger.Debug().Msg("No new batches")
			continue
		}
		logger.Debug().Int("batches num", len(batches)).Msg("received new batches")

		for i, batch := range batches {
			logger.Debug().Int("batch", i).Int("tx", len(batches[i].Transactions)).Int("blocks", len(batches[i].ExternalBlocks)).Msg("received new batch")

			err = func() error {
				rwtx, err := a.AppchainDB.BeginRw(context.TODO())
				if err != nil {
					logger.Error().Err(err).Msg("Failed to get new batch")
					return fmt.Errorf("Failed to begin write tx: %w", err)
				}
				defer rwtx.Rollback()

				//2) Process Batch. Execute transaction there.
				logger.Debug().Int("tx", len(batch.Transactions)).Msg("Process batch")
				extTxs, err := a.appchainStateExecution.ProcessBatch(batch, rwtx)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to process batch")
					return fmt.Errorf("Failed to process batch: %w", err)
				}

				// Разделение на блоки(возможное) тоже тут.
				stateRoot, err := a.rootCalculator.StateRootCalculator(rwtx)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to calculate state root")
					return fmt.Errorf("Failed to calculate state root: %w", err)
				}

				// blockNumber uint64, stateRoot [32]byte, previousBlockHash [32]byte, txs Batch[appTx]
				// we believe that blocks are not very important, so we added them for capability with block explorers
				blockNumber := previousBlockNumber + 1
				block := a.blockBuilder(blockNumber, stateRoot, previousBlockHash, batch)

				// сохраняем блок
				logger.Debug().Uint64("block_number", blockNumber).
					Msg("Write block")
				if err = WriteBlock(rwtx, block.Number(), block.Bytes()); err != nil {
					logger.Error().Err(err).Msg("Failed to write block")
					return fmt.Errorf("Failed to write block: %w", err)
				}

				blockHash := block.Hash()

				externalTXRoot, err := WriteExternalTransactions(context.TODO(), rwtx, blockNumber, extTxs)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write external transactions")
					return fmt.Errorf("Failed to write external transactions: %w", err)
				}

				checkpoint := types.Checkpoint{
					ChainID:                  a.config.ChainID, //todo надо бы его иметь в базе и в genesis
					BlockNumber:              blockNumber,
					BlockHash:                blockHash,
					StateRoot:                stateRoot,
					ExternalTransactionsRoot: externalTXRoot,
				}

				logger.Debug().Uint64("block", checkpoint.BlockNumber).Msg("Write checkpoint")
				err = WriteCheckpoint(context.TODO(), rwtx, checkpoint)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write checkpoint")
					return fmt.Errorf("Failed to write checkpoint: %w", err)
				}

				logger.Debug().Uint64("block_number", blockNumber).
					Str("hash", hex.EncodeToString(blockHash[:])).
					Msg("Write last block")
				err = WriteLastBlock(rwtx, blockNumber, blockHash)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write last block")
					return fmt.Errorf("Failed to write last block: %w", err)
				}

				logger.Debug().Int64("Next snapshot pos", batch.EndOffset).Msg("Write checkpoint")
				err = WriteSnapshotPosition(rwtx, currentEpoch, batch.EndOffset)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write snapshot pos")
					return fmt.Errorf("Failed to write snapshot pos: %w", err)
				}

				err = rwtx.Commit()
				if err != nil {
					logger.Error().Err(err).Msg("Failed to commit")
					return fmt.Errorf("Failed to commit: %w", err)
				}

				logger.Info().Uint64("block_number", blockNumber).Hex("atropos", batch.Atropos[:]).Msg("Block processed and committed")

				previousBlockNumber = blockNumber
				previousBlockHash = block.Hash()

				return nil
			}()

			if err != nil {
				logger.Error().Err(err).Msg("Failed to handle batch")
				return err
			}
		}
	}
	return nil
}

func (a *Appchain[STI, appTx, AppBlock]) RunEmitterAPI(ctx context.Context) {
	logger := log.Ctx(ctx)
	lis, err := net.Listen("tcp", a.config.EmitterPort)
	if err != nil {
		logger.Fatal().Err(err).Str("port", a.config.EmitterPort).Msg("Failed to create listener")
	}
	logger.Info().Str("port", a.config.EmitterPort).Msg("Starting gRPC server")

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()
	emitterproto.RegisterEmitterServer(grpcServer, a.emiterAPI)
	emitterproto.RegisterHealthServer(grpcServer, &HealthServer{})

	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal().Err(err).Msg("gRPC server crashed")
	}
}

func WriteBlock(rwtx kv.RwTx, blockNumber uint64, blockBytes []byte) error {
	number := make([]byte, 8)
	binary.BigEndian.PutUint64(number, blockNumber)
	return rwtx.Put(BlocksBucket, number, blockBytes)
}

func WriteLastBlock(rwtx kv.RwTx, number uint64, hash [32]byte) error {
	value := make([]byte, 8+32)
	binary.BigEndian.PutUint64(value[:8], number)
	copy(value[8:], hash[:])

	return rwtx.Put(ConfigBucket, []byte(LastBlockKey), value)
}

func GetLastBlock(tx kv.Tx) (uint64, [32]byte, error) {
	value, err := tx.GetOne(ConfigBucket, []byte(LastBlockKey))
	if err != nil {
		return 0, [32]byte{}, err
	}

	if len(value) != 8+32 {
		return 0, [32]byte{}, nil
	}

	number := binary.BigEndian.Uint64(value[:8])
	return number, ([32]byte)(value[8:]), err
}

// Функция записи внешней транзакции в MDBX
// todo ответственность за взаимодействия с валидатором на стороне AppchainEmitterServer
// Should be called strictly once per block
func WriteExternalTransactions(ctx context.Context, dbTx kv.RwTx, blockNumber uint64, txs []types.ExternalTransaction) ([32]byte, error) {
	root := Merklize(txs)

	value, err := proto.Marshal(TransactionToProto(txs, blockNumber, root))
	if err != nil {
		log.Error().Err(err).Msg("Transaction serialization failed")
		return [32]byte{}, fmt.Errorf("transaction serialization failed: %w", err)
	}

	// Записываем в базу
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNumber)

	if err = dbTx.Put(ExternalTxBucket, key, value); err != nil {
		return [32]byte{}, fmt.Errorf("can't write external transactions to the DB: error %w", err)
	}

	// todo: store the root in DB
	return root, nil
}

func TransactionToProto(txs []types.ExternalTransaction, blockNumber uint64, root [32]byte) *emitterproto.GetExternalTransactionsResponse_BlockTransactions {
	protoTxs := make([]*emitterproto.ExternalTransaction, len(txs))

	for i, tx := range txs {
		protoTxs[i].ChainId = tx.ChainID
		protoTxs[i].Tx = tx.Tx
	}

	return &emitterproto.GetExternalTransactionsResponse_BlockTransactions{
		BlockNumber:          blockNumber,
		TransactionsRootHash: root[:],
		ExternalTransactions: protoTxs,
	}
}

func CheckpointToProto(cp types.Checkpoint) *emitterproto.CheckpointResponse_Checkpoint {
	return &emitterproto.CheckpointResponse_Checkpoint{
		LatestBlockNumber:  cp.BlockNumber,
		StateRoot:          cp.StateRoot[:],                // Преобразование массива в срез
		BlockHash:          cp.BlockHash[:],                // Преобразование массива в срез
		ExternalTxRootHash: cp.ExternalTransactionsRoot[:], // Преобразование массива в срез
	}
}

// todo: merklize transactions
func Merklize(txs []types.ExternalTransaction) [32]byte {
	return [32]byte{}
}

// fixme надо писать чекпоинт в staged sync
// должен быть совместим с GetCheckpoints
func WriteCheckpoint(ctx context.Context, dbTx kv.RwTx, checkpoint types.Checkpoint) error {
	logger := log.Ctx(ctx)

	// Генерируем ключ из LatestBlockNumber (8 байт)
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, checkpoint.BlockNumber)

	// Сериализуем чекпоинт
	value, err := json.Marshal(checkpoint)
	if err != nil {
		logger.Error().Err(err).Msg("Checkpoint serialization failed")
		return fmt.Errorf("checkpoint serialization failed: %w", err)
	}

	// Записываем в базу
	return dbTx.Put(CheckpointBucket, key, value)
}

func WriteSnapshotPosition(rwtx kv.RwTx, epoch uint32, pos int64) error {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, uint64(pos))

	return rwtx.Put(Snapshot, key, val)
}

func ReadSnapshotPosition(tx kv.Tx, epoch uint32) (int64, error) {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val, err := tx.GetOne(Snapshot, key)
	if err != nil {
		return 0, err
	}
	if len(val) != 8 {
		return 0, nil // default to beginning
	}

	return int64(binary.BigEndian.Uint64(val)), nil
}

func ReadTxSnapshotPosition(tx kv.Tx, epoch uint32) (int64, error) {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val, err := tx.GetOne(TxSnapshot, key)
	if err != nil {
		return 8, nil // fallback: пропускаем заголовок
	}
	if len(val) != 8 {
		return 8, nil
	}

	return int64(binary.BigEndian.Uint64(val)), nil
}

func WriteTxSnapshotPosition(rwtx kv.RwTx, epoch uint32, pos int64) error {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, uint64(pos))

	return rwtx.Put(TxSnapshot, key, val)
}

var currentEpoch = uint32(1)

func GetLastStreamPositions(appchainDB kv.RwDB) (int64, int64, error) {
	startEventPos := int64(8)
	startTxPos := int64(8)

	err := appchainDB.View(context.TODO(), func(tx kv.Tx) error {
		var err error
		startEventPos, err = ReadSnapshotPosition(tx, currentEpoch)
		if err != nil {
			return err
		}

		startTxPos, err = ReadTxSnapshotPosition(tx, currentEpoch)
		return err
	})
	if err != nil {
		return 0, 0, fmt.Errorf("Faild to get stream positions:%w", err)
	}
	return startEventPos, startTxPos, nil
}
