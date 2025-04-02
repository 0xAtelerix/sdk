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
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"net"
	"path/filepath"
)

func NewAppchain[STI StateTransitionInterface[appTx],
	appTx types.AppTransaction,
	AppBlock types.AppchainBlock](sti STI,
	rootCalculator types.RootCalculator,
	blockBuilder types.AppchainBlockConstructor[appTx, AppBlock],
	txpool types.TxPoolInterface[appTx],
	config AppchainConfig) (Appchain[STI, appTx, AppBlock], error) {

	log.Info().Str("db_path", config.AppchainDBPath).Msg("Initializing appchain database")

	// инициализируем базу на нашей стороне
	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(config.AppchainDBPath).
		WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
			return defaultTables
		}).
		Open()
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize MDBX")
		return Appchain[STI, appTx, AppBlock]{}, fmt.Errorf("failed to initialize MDBX: %w", err)
	}

	startEventPos := int64(8)
	startTxPos := int64(8)

	err = appchainDB.View(context.TODO(), func(tx kv.Tx) error {
		startEventPos, err = ReadSnapshotPosition(tx, currentEpoch)
		if err != nil {
			return err
		}

		startTxPos, err = ReadTxSnapshotPosition(tx, currentEpoch)
		return err
	})
	if err != nil {
		return Appchain[STI, appTx, AppBlock]{}, err
	}

	//todo надо открывать в run. Отсутствие файла - не должно быть причиной падения
	log.Info().Str("dir", config.EventStreamDir).Int64("start event", startEventPos).Int64("start tx", startTxPos).Msg("Initializing event readers")
	eventStream, err := NewEventStreamWrapper[appTx](filepath.Join(config.EventStreamDir, "epoch_0.data"),
		filepath.Join(config.TxStreamDir, fmt.Sprintf("%d", config.ChainID), "epoch_0_"+fmt.Sprintf("%d", config.ChainID)+"_tx.data"),
		uint32(config.ChainID),
		startEventPos, startTxPos,
	)

	emiterAPI := NewServer(appchainDB, config.ChainID, txpool)
	log.Info().Msg("Appchain initialized successfully")

	return Appchain[STI, appTx, AppBlock]{
		appchainStateExecution: sti,
		rootCalculator:         rootCalculator,
		blockBuilder:           blockBuilder,
		emiterAPI:              emiterAPI,
		AppchainDB:             appchainDB,
		eventStream:            eventStream,
		config:                 config,
	}, nil
}

type AppchainConfig struct {
	ChainID        uint64
	EmitterPort    string
	AppchainDBPath string
	TmpDBPath      string
	EventStreamDir string
	TxStreamDir    string
}

// todo: it should be stored at the first run and checked on next
func MakeAppchainConfig(chainID uint64) AppchainConfig {
	return AppchainConfig{
		ChainID:        chainID,
		EmitterPort:    ":50051",
		AppchainDBPath: "appchaindb",
		TmpDBPath:      "tmpdb",
		EventStreamDir: "epochs",
		TxStreamDir:    fmt.Sprintf("%d", chainID),
	}
}

type Appchain[STI StateTransitionInterface[appTx], appTx types.AppTransaction, AppBlock types.AppchainBlock] struct {
	appchainStateExecution STI
	rootCalculator         types.RootCalculator
	blockBuilder           types.AppchainBlockConstructor[appTx, AppBlock]

	eventStream BatchReader[appTx] //тут наши детерминированные снепшоты
	emiterAPI   emitterproto.EmitterServer
	AppchainDB  kv.RwDB
	config      AppchainConfig
}

func (a *Appchain[STI, appTx, AppBlock]) Run(ctx context.Context) error {
	log.Info().Msg("Appchain run started")

	go a.RunEmitterAPI()

	var (
		previousBlockNumber uint64
		previousBlockHash   [32]byte
		err                 error
	)

	err = a.AppchainDB.View(ctx, func(tx kv.Tx) error {
		previousBlockNumber, previousBlockHash, err = GetLastBlock(tx)

		return err
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get last block")
		return fmt.Errorf("Failed to get last block: %w", err)
	}

runFor:
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Appchain context cancelled, stopping")
			break runFor
		default:
		}

		batches, err := a.eventStream.GetNewBatchesBlocking(10)
		if err != nil {
			return fmt.Errorf("Failed to get new batch: %w", err)
		}

		if len(batches) == 0 {
			log.Debug().Msg("No new batches")
			continue
		} else {
			log.Debug().Int("batches num", len(batches)).Msg("received new batch")
			for i := range batches {
				log.Debug().Int("batch", i).Int("tx", len(batches[i].Transactions)).Int("blocks", len(batches[i].ExternalBlocks)).Msg("received new batch")
			}
		}

		for _, batch := range batches {
			err = func() error {
				rwtx, err := a.AppchainDB.BeginRw(context.TODO())
				if err != nil {
					log.Error().Err(err).Msg("Failed to get new batch")
					return fmt.Errorf("Failed to begin write tx: %w", err)
				}
				defer rwtx.Rollback()

				//2) Process Batch. Execute transaction there.
				log.Debug().Int("tx", len(batch.Transactions)).Msg("Process batch")
				extTxs, err := a.appchainStateExecution.ProcessBatch(batch, rwtx)
				if err != nil {
					log.Error().Err(err).Msg("Failed to process batch")
					return fmt.Errorf("Failed to process batch: %w", err)
				}

				// Разделение на блоки(возможное) тоже тут.
				stateRoot, err := a.rootCalculator.StateRootCalculator(rwtx)
				if err != nil {
					log.Error().Err(err).Msg("Failed to calculate state root")
					return fmt.Errorf("Failed to calculate state root: %w", err)
				}

				// blockNumber uint64, stateRoot [32]byte, previousBlockHash [32]byte, txs Batch[appTx]
				// we believe that blocks are not very important, so we added them for capability with block explorers
				blockNumber := previousBlockNumber + 1
				block := a.blockBuilder(blockNumber, stateRoot, previousBlockHash, batch)

				// сохраняем блок
				if err = WriteBlock(rwtx, block.Number(), block.Bytes()); err != nil {
					log.Error().Err(err).Msg("Failed to write block")
					return fmt.Errorf("Failed to write block: %w", err)
				}

				blockHash := block.Hash()

				externalTXRoot, err := WriteExternalTransactions(context.TODO(), rwtx, blockNumber, extTxs)
				if err != nil {
					log.Error().Err(err).Msg("Failed to write external transactions")
					return fmt.Errorf("Failed to write external transactions: %w", err)
				}

				checkpoint := types.Checkpoint{
					ChainID:                  a.config.ChainID, //todo надо бы его иметь в базе и в genesis
					BlockNumber:              blockNumber,
					BlockHash:                blockHash,
					StateRoot:                stateRoot,
					ExternalTransactionsRoot: externalTXRoot,
				}

				log.Debug().Uint64("block", checkpoint.BlockNumber).Msg("Write checkpoint")
				err = WriteCheckpoint(context.TODO(), rwtx, checkpoint)
				if err != nil {
					log.Error().Err(err).Msg("Failed to write checkpoint")
					return fmt.Errorf("Failed to write checkpoint: %w", err)
				}

				err = WriteLastBlock(rwtx, blockNumber, blockHash)
				if err != nil {
					log.Error().Err(err).Msg("Failed to write last block")
					return fmt.Errorf("Failed to write last block: %w", err)
				}

				log.Debug().Int64("Next snapshot pos", batch.EndOffset).Msg("Write checkpoint")
				err = WriteSnapshotPosition(rwtx, currentEpoch, batch.EndOffset)
				if err != nil {
					log.Error().Err(err).Msg("Failed to write snapshot pos")
					return fmt.Errorf("Failed to write snapshot pos: %w", err)
				}

				err = rwtx.Commit()
				if err != nil {
					log.Error().Err(err).Msg("Failed to commit")
					return fmt.Errorf("Failed to commit: %w", err)
				}

				log.Info().Uint64("block_number", blockNumber).Msg("Block processed and committed")

				previousBlockNumber = block.Number()
				previousBlockHash = block.Hash()

				return nil
			}()

			if err != nil {
				log.Error().Err(err).Msg("Failed to handle batch")
				return err
			}
		}
	}
	return nil
}

func (a *Appchain[STI, appTx, AppBlock]) RunEmitterAPI() {
	lis, err := net.Listen("tcp", a.config.EmitterPort)
	if err != nil {
		log.Fatal().Err(err).Str("port", a.config.EmitterPort).Msg("Failed to create listener")
	}
	log.Info().Str("port", a.config.EmitterPort).Msg("Starting gRPC server")

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()
	emitterproto.RegisterEmitterServer(grpcServer, a.emiterAPI)
	emitterproto.RegisterHealthServer(grpcServer, &HealthServer{})

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("gRPC server crashed")
	}
}

func WriteBlock(rwtx kv.RwTx, blockNumber uint64, blockBytes []byte) error {
	log.Debug().Uint64("block_number", blockNumber).
		Msg("Write block")

	number := make([]byte, 8)
	binary.BigEndian.PutUint64(number, blockNumber)
	return rwtx.Put(blocksBucket, number, blockBytes)
}

func WriteLastBlock(rwtx kv.RwTx, number uint64, hash [32]byte) error {
	log.Debug().Uint64("block_number", number).
		Str("hash", hex.EncodeToString(hash[:])).
		Msg("Write last block")

	value := make([]byte, 8+32)
	binary.BigEndian.PutUint64(value[:8], number)
	copy(value[8:], hash[:])

	return rwtx.Put(configBucket, []byte(lastBlockKey), value)
}

func GetLastBlock(tx kv.Tx) (uint64, [32]byte, error) {
	value, err := tx.GetOne(configBucket, []byte(lastBlockKey))
	if err != nil {
		return 0, [32]byte{}, err
	}

	if len(value) != 8+32 {
		return 0, [32]byte{}, nil
	}

	number := binary.BigEndian.Uint64(value[:8])
	log.Debug().Uint64("Block", number).
		Str("hash", hex.EncodeToString((value[8:]))).
		Msg("GetLastBlock")

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

	if err = dbTx.Put(externalTxBucket, key, value); err != nil {
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
	// Генерируем ключ из LatestBlockNumber (8 байт)
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, checkpoint.BlockNumber)

	// Сериализуем чекпоинт
	value, err := json.Marshal(checkpoint)
	if err != nil {
		log.Error().Err(err).Msg("Checkpoint serialization failed")
		return fmt.Errorf("checkpoint serialization failed: %w", err)
	}

	// Записываем в базу
	return dbTx.Put(checkpointBucket, key, value)
}

func WriteSnapshotPosition(rwtx kv.RwTx, epoch uint32, pos int64) error {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, uint64(pos))

	return rwtx.Put(snapshot, key, val)
}

func ReadSnapshotPosition(tx kv.Tx, epoch uint32) (int64, error) {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val, err := tx.GetOne(snapshot, key)
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

	val, err := tx.GetOne(txSnapshot, key)
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

	return rwtx.Put(txSnapshot, key, val)
}

var currentEpoch = uint32(1)
