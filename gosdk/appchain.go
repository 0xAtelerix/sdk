package gosdk

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
	"github.com/0xAtelerix/sdk/gosdk/types"
)

func NewAppchain[STI StateTransitionInterface[appTx],
	appTx types.AppTransaction,
	AppBlock types.AppchainBlock](sti STI,
	rootCalculator types.RootCalculator,
	blockBuilder types.AppchainBlockConstructor[appTx, AppBlock],
	txpool types.TxPoolInterface[appTx],
	config AppchainConfig) (Appchain[STI, appTx, AppBlock], error) {

	// инициализируем базу на нашей стороне
	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(config.AppchainDBPath).
		WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				checkpointBucket: {},
				externalTxBucket: {},
				blocksBucket:     {},
			}
		}).
		Open()
	if err != nil {
		return Appchain[STI, appTx, AppBlock]{}, fmt.Errorf("ошибка инициализации MDBX: %w", err)
	}

	emiterAPI := NewServer(db, config.ChainID)
	return Appchain[STI, appTx, AppBlock]{
		appchainStateExecution: sti,
		rootCalculator:         rootCalculator,
		blockBuilder:           blockBuilder,
		emiterAPI:              emiterAPI,
		AppchainDB:             db,
		config:                 config,
	}, nil
}

type AppchainConfig struct {
	ChainID        uint64
	EmitterPort    string
	AppchainDBPath string
	TmpDBPath      string
	EventStreamDir string
}

// todo: it should be stored at the first run and checked on next
func MakeAppchainConfig(chainID uint64) AppchainConfig {
	return AppchainConfig{
		ChainID:        chainID,
		EmitterPort:    ":50051",
		AppchainDBPath: "./test",
		TmpDBPath:      "./test_tmp",
		EventStreamDir: "",
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
		return fmt.Errorf("Failed to get last block: %w", err)
	}

runFor:
	for {
		select {
		case <-ctx.Done():
			break runFor
		default:
		}

		batches, err := a.eventStream.GetNewBatchesBlocking(10)
		if err != nil {
			return fmt.Errorf("Failed to get new batch: %w", err)
		}

		for _, batch := range batches {
			err = func() error {
				rwtx, err := a.AppchainDB.BeginRw(context.TODO())
				if err != nil {
					return fmt.Errorf("Failed to begin write tx: %w", err)
				}
				defer rwtx.Rollback()

				//2) Process Batch. Execute transaction there.
				extTxs, err := a.appchainStateExecution.ProcessBatch(batch, rwtx)
				if err != nil {
					return fmt.Errorf("Failed to process batch: %w", err)
				}

				// Разделение на блоки(возможное) тоже тут.
				stateRoot, err := a.rootCalculator.StateRootCalculator(rwtx)
				if err != nil {
					return fmt.Errorf("Failed to calculate state root: %w", err)
				}

				// blockNumber uint64, stateRoot [32]byte, previousBlockHash [32]byte, txs Batch[appTx]
				// we believe that blocks are not very important, so we added them for capability with block explorers
				blockNumber := previousBlockNumber + 1
				block := a.blockBuilder(blockNumber, stateRoot, previousBlockHash, batch)

				// сохраняем блок
				if err = WriteBlock(rwtx, block.Number(), block.Bytes()); err != nil {
					return fmt.Errorf("Failed to write block: %w", err)
				}

				blockHash := block.Hash()

				externalTXRoot, err := WriteExternalTransactions(context.TODO(), rwtx, blockNumber, extTxs)
				if err != nil {
					return fmt.Errorf("Failed to write external transactions: %w", err)
				}

				checkpoint := types.Checkpoint{
					ChainID:                  a.config.ChainID, //todo надо бы его иметь в базе и в genesis
					BlockNumber:              blockNumber,
					BlockHash:                blockHash,
					StateRoot:                stateRoot,
					ExternalTransactionsRoot: externalTXRoot,
				}

				err = WriteCheckpoint(context.TODO(), rwtx, checkpoint)
				if err != nil {
					return fmt.Errorf("Failed to write checkpoint: %w", err)
				}

				err = WriteLastBlock(rwtx, blockNumber, blockHash)
				if err != nil {
					return fmt.Errorf("Failed to write last block: %w", err)
				}

				err = rwtx.Commit()
				if err != nil {
					return fmt.Errorf("Failed to commit: %w", err)
				}

				previousBlockNumber = block.Number()
				previousBlockHash = block.Hash()

				return nil
			}()

			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Appchain[STI, appTx, AppBlock]) RunEmitterAPI() {
	lis, err := net.Listen("tcp", a.config.EmitterPort)
	if err != nil {
		log.Fatalf("Не удалось создать listener на %s: %v", a.config.EmitterPort, err)
	}

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()
	emitterproto.RegisterEmitterServer(grpcServer, a.emiterAPI)
	emitterproto.RegisterHealthServer(grpcServer, &HealthServer{})

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Ошибка работы сервера: %v", err)
	}
}

func WriteBlock(rwtx kv.RwTx, blockNumber uint64, blockBytes []byte) error {
	number := make([]byte, 8)
	binary.BigEndian.PutUint64(number, blockNumber)
	return rwtx.Put(blocksBucket, number, blockBytes)
}

func WriteLastBlock(rwtx kv.RwTx, number uint64, hash [32]byte) error {
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
		return 0, [32]byte{}, fmt.Errorf("got inconsistant last block value: len %d, value %v", len(value), value)
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
		return [32]byte{}, fmt.Errorf("ошибка сериализации транзакции: %w", err)
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
		return fmt.Errorf("ошибка сериализации чекпоинта: %w", err)
	}

	// Записываем в базу
	return dbTx.Put(checkpointBucket, key, value)
}
