package gosdk

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

type ExampleBlock struct{}

func (*ExampleBlock) Number() uint64 {
	return 0
}

func (*ExampleBlock) Hash() [32]byte {
	return [32]byte{}
}

func (*ExampleBlock) StateRoot() [32]byte {
	return [32]byte{}
}

func (*ExampleBlock) Bytes() []byte {
	return []byte{}
}

func WithRootCalculator[STI StateTransitionInterface[AppTx, R],
	AppTx apptypes.AppTransaction[R],
	R apptypes.Receipt,
	AppBlock apptypes.AppchainBlock](rc apptypes.RootCalculator) func(a *Appchain[STI, AppTx, R, AppBlock]) {
	return func(a *Appchain[STI, AppTx, R, AppBlock]) {
		a.rootCalculator = rc
	}
}

type AppchainConfig struct {
	ChainID           uint64
	EmitterPort       string
	PrometheusPort    string
	AppchainDBPath    string
	EventStreamDir    string
	TxStreamDir       string
	MultichainStateDB map[apptypes.ChainType]string
	Logger            *zerolog.Logger
	ValidatorID       string
}

// todo: it should be stored at the first run and checked on next
func MakeAppchainConfig(
	chainID uint64,
	multichainStateDB map[apptypes.ChainType]string,
) AppchainConfig {
	return AppchainConfig{
		ChainID:           chainID,
		EmitterPort:       ":50051",
		PrometheusPort:    "",
		AppchainDBPath:    "chaindb",
		EventStreamDir:    "epochs",
		TxStreamDir:       strconv.FormatUint(chainID, 10),
		MultichainStateDB: multichainStateDB,
	}
}

func NewAppchain[STI StateTransitionInterface[AppTx, R],
	AppTx apptypes.AppTransaction[R],
	R apptypes.Receipt,
	AppBlock apptypes.AppchainBlock](
	sti STI,
	blockBuilder apptypes.AppchainBlockConstructor[AppTx, R, AppBlock],
	txpool apptypes.TxPoolInterface[AppTx, R],
	config AppchainConfig,
	appchainDB kv.RwDB,
	subscriber *Subscriber,
	multichain *MultichainStateAccess,
	txBatchDB kv.RoDB,
	options ...func(a *Appchain[STI, AppTx, R, AppBlock]),
) Appchain[STI, AppTx, R, AppBlock] {
	log.Info().Str("db_path", config.AppchainDBPath).Msg("Initializing appchain database")

	emiterAPI := NewServer(appchainDB, config.ChainID, txpool)

	emiterAPI.logger = &log.Logger
	if config.Logger != nil {
		emiterAPI.logger = config.Logger
	}

	log.Info().Msg("Appchain initialized successfully")

	appchain := Appchain[STI, AppTx, R, AppBlock]{
		appchainStateExecution: sti,
		rootCalculator:         NewStubRootCalculator(),
		blockBuilder:           blockBuilder,
		emiterAPI:              emiterAPI,
		AppchainDB:             appchainDB,
		TxBatchDB:              txBatchDB,
		config:                 config,
		multichainDB:           multichain,
		subscriber:             subscriber,
	}

	for _, option := range options {
		option(&appchain)
	}

	return appchain
}

type Appchain[STI StateTransitionInterface[appTx, R], appTx apptypes.AppTransaction[R], R apptypes.Receipt, AppBlock apptypes.AppchainBlock] struct {
	appchainStateExecution STI
	rootCalculator         apptypes.RootCalculator
	blockBuilder           apptypes.AppchainBlockConstructor[appTx, R, AppBlock]

	emiterAPI    emitterproto.EmitterServer
	AppchainDB   kv.RwDB
	TxBatchDB    kv.RoDB
	config       AppchainConfig
	multichainDB *MultichainStateAccess
	subscriber   *Subscriber
}

func (a *Appchain[STI, appTx, R, AppBlock]) Run(
	ctx context.Context,
	streamConstructor EventStreamWrapperConstructor[appTx, R],
) error {
	logger := log.Ctx(ctx)
	logger.Warn().Msg("Appchain run started")

	ctx = utility.CtxWithValidatorID(ctx, a.config.ValidatorID)
	ctx = utility.CtxWithChainID(ctx, a.config.ChainID)
	vid := utility.ValidatorIDFromCtx(ctx)
	cid := utility.ChainIDFromCtx(ctx)

	emitterErrChan := make(chan error)

	go a.RunEmitterAPI(ctx, emitterErrChan)

	err := <-emitterErrChan
	if err != nil {
		panic(err)
	}

	if a.config.PrometheusPort != "" {
		go startPrometheusServer(ctx, a.config.PrometheusPort)
	}

	startEventPos, startTxPos, err := GetLastStreamPositions(ctx, a.AppchainDB)
	if err != nil {
		return err
	}

	logger.Info().
		Str("dir", a.config.EventStreamDir).
		Int64("start event", startEventPos).
		Int64("start tx", startTxPos).
		Msg("Initializing event readers")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err = os.Stat(a.config.EventStreamDir)
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

	var (
		eventStream       Streamer[appTx, R]
		votingBlocks      *Voting[apptypes.ExternalBlock]
		votingCheckpoints *Voting[apptypes.Checkpoint]
	)

	err = a.AppchainDB.View(ctx, func(tx kv.Tx) error {
		votingBlocks, err = NewVotingFromStorage(tx, apptypes.MakeExternalBlock, nil)
		if err != nil {
			return err
		}

		votingCheckpoints, err = NewVotingFromStorage(tx, apptypes.MakeCheckpoint, nil)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if streamConstructor == nil {
		logger.Info().Msg("NewMdbxEventStreamWrapper")
		eventStream, err = NewMdbxEventStreamWrapper[appTx, R](
			filepath.Join(a.config.EventStreamDir, "epoch_1.data"),
			uint32(a.config.ChainID),
			startEventPos,
			a.TxBatchDB,
			logger,
			a.AppchainDB,
			a.subscriber,
			votingBlocks,
			votingCheckpoints,
		)
	} else {
		eventStream, err = streamConstructor(filepath.Join(a.config.EventStreamDir, "epoch_1.data"),
			uint32(a.config.ChainID),
			startEventPos,
			a.TxBatchDB,
			logger,
			a.AppchainDB,
			a.subscriber,
			votingBlocks,
			votingCheckpoints,
		)
	}

	if err != nil {
		logger.Error().Err(err).Msg("Failed to create event stream")

		return fmt.Errorf("failed to create event stream: %w", err)
	}

	defer func() {
		streamErr := eventStream.Close()
		if streamErr != nil {
			logger.Error().Err(streamErr).Msg("Error closing event stream")
		}
	}()

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

		return fmt.Errorf("failed to get last block: %w", err)
	}

	logger.Info().
		Uint64("previous block number", previousBlockNumber).
		Hex("hash", previousBlockHash[:]).
		Msg("load bn")

runFor:
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Appchain context cancelled, stopping")

			break runFor
		default:
		}

		logger.Debug().Msg("getting batches")

		timer := time.Now()
		batches, err := eventStream.GetNewBatchesBlocking(ctx, 10)

		EventStreamBlockingDuration.WithLabelValues(vid, cid).Observe(time.Since(timer).Seconds())

		if err != nil {
			logger.Error().Err(err).Msg("Failed to get new batches blocking")

			return fmt.Errorf("failed to get new batch: %w", err)
		}

		if len(batches) == 0 {
			logger.Debug().Msg("No new batches")

			continue
		}

		logger.Debug().Int("batches num", len(batches)).Msg("received new batches")

		for i, batch := range batches {
			logger.Debug().Int("batch", i).Int("tx", len(batches[i].Transactions)).Int("blocks", len(batches[i].ExternalBlocks)).Msg("received new batch")

			start := time.Now() // метка начала

			err = func(ctx context.Context) error {
				var rwtx kv.RwTx

				rwtx, err = a.AppchainDB.BeginRw(ctx)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to get new batch")

					return fmt.Errorf("failed to begin write tx: %w", err)
				}
				defer rwtx.Rollback()

				// 2) Process Batch. Execute transaction there.
				logger.Debug().Int("tx", len(batch.Transactions)).Msg("Process batch")

				var (
					extTxs            []apptypes.ExternalTransaction
					processedReceipts []R
				)

				processedReceipts, extTxs, err = a.appchainStateExecution.ProcessBatch(ctx, batch, rwtx)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to process batch")

					return fmt.Errorf("failed to process batch: %w", err)
				}

				for _, processedReceipt := range processedReceipts {
					if storeErr := receipt.StoreReceipt(rwtx, processedReceipt); storeErr != nil {
						logger.Error().Err(storeErr).Msg("Failed to store receipt")

						return fmt.Errorf("failed to store receipt: %w", storeErr)
					}
				}

				// Разделение на блоки(возможное) тоже тут.
				var stateRoot [32]byte

				stateRoot, err = a.rootCalculator.StateRootCalculator(rwtx)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to calculate state root")

					return fmt.Errorf("failed to calculate state root: %w", err)
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

					return fmt.Errorf("failed to write block: %w", err)
				}

				blockHash := block.Hash()

				var externalTXRoot [32]byte

				externalTXRoot, err = WriteExternalTransactions(rwtx, blockNumber, extTxs)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write external transactions")

					return fmt.Errorf("failed to write external transactions: %w", err)
				}

				checkpoint := apptypes.Checkpoint{
					ChainID:                  a.config.ChainID, // todo надо бы его иметь в базе и в genesis
					BlockNumber:              blockNumber,
					BlockHash:                blockHash,
					StateRoot:                stateRoot,
					ExternalTransactionsRoot: externalTXRoot,
				}

				logger.Debug().Uint64("block", checkpoint.BlockNumber).Msg("Write checkpoint")

				err = WriteCheckpoint(ctx, rwtx, checkpoint)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write checkpoint")

					return fmt.Errorf("failed to write checkpoint: %w", err)
				}

				logger.Debug().Uint64("block_number", blockNumber).
					Str("hash", hex.EncodeToString(blockHash[:])).
					Msg("Write last block")

				err = WriteLastBlock(rwtx, blockNumber, blockHash)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write last block")

					return fmt.Errorf("failed to write last block: %w", err)
				}

				logger.Debug().Int64("Next snapshot pos", batch.EndOffset).Msg("Write checkpoint")

				err = WriteSnapshotPosition(rwtx, currentEpoch, batch.EndOffset)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to write snapshot pos")

					return fmt.Errorf("failed to write snapshot pos: %w", err)
				}

				// write voting
				err = votingBlocks.StoreProgress(rwtx)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to store progress block")

					return fmt.Errorf("failed to store progress block: %w", err)
				}

				err = votingCheckpoints.StoreProgress(rwtx)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to store progress checkpoint")

					return fmt.Errorf("failed to store progress checkpoint: %w", err)
				}

				err = rwtx.Commit()
				if err != nil {
					logger.Error().Err(err).Msg("Failed to commit")

					return fmt.Errorf("failed to commit: %w", err)
				}

				logger.
					Info().
					Uint64("block_number", blockNumber).
					Hex("atropos", batch.Atropos[:]).
					Msg("Block processed and committed")

				BlockProcessingDuration.WithLabelValues(vid, cid).Observe(time.Since(start).Seconds())
				ProcessedBlocks.WithLabelValues(vid, cid).Inc()
				ProcessedTransactions.WithLabelValues(vid, cid).Add(float64(len(batch.Transactions)))

				HeadBlockNumber.WithLabelValues(vid, cid).Set(float64(blockNumber))
				EventStreamPosition.
					WithLabelValues(vid, cid, strconv.FormatUint(uint64(currentEpoch), 10)).
					Set(float64(batch.EndOffset))
				BlockExternalTxs.WithLabelValues(vid, cid).Observe(float64(len(extTxs)))
				BlockInternalTxs.WithLabelValues(vid, cid).Observe(float64(len(batch.Transactions)))
				BlockBytes.WithLabelValues(vid, cid).Observe(float64(len(block.Bytes())))

				previousBlockNumber = blockNumber
				previousBlockHash = block.Hash()

				return nil
			}(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to handle batch")

				return err
			}
		}

		BatchProcessingDuration.WithLabelValues(vid, cid).Observe(time.Since(timer).Seconds())
	}

	return nil
}

func (a *Appchain[STI, appTx, R, AppBlock]) RunEmitterAPI(ctx context.Context, errCh chan error) {
	logger := log.Ctx(ctx)

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.config.EmitterPort)
	if err != nil {
		errCh <- fmt.Errorf("failed to create listener: %w, port %q", err, a.config.EmitterPort)

		return
	}

	close(errCh)

	logger.Info().Str("port", a.config.EmitterPort).Msg("Starting gRPC server")

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()
	emitterproto.RegisterEmitterServer(grpcServer, a.emiterAPI)
	emitterproto.RegisterHealthServer(grpcServer, &HealthServer{})

	serveErr := make(chan error, 1)

	go func() {
		serveErr <- grpcServer.Serve(lis)
	}()

	// Optional: make the grace period configurable.
	const gracePeriod = 10 * time.Second

	select {
	case <-ctx.Done():
		logger.Info().Err(ctx.Err()).Msg("Context canceled, shutting down gRPC server")

		// Try graceful stop first (drains existing RPCs).
		done := make(chan struct{})

		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			logger.Info().Msg("gRPC server stopped gracefully")
		case <-time.After(gracePeriod):
			logger.Warn().Dur("timeout", gracePeriod).Msg("Graceful stop timed out; forcing stop")

			// Immediately close listeners and cancel in-flight RPCs
			grpcServer.Stop()
		}

		// Drain Serve() error; it's expected to be nil or an internal "server stopped" error.
		<-serveErr

	case err = <-serveErr:
		// Serve exited on its own (listener error, etc.)
		if err != nil {
			logger.Panic().Err(err).Msg("gRPC server crashed")
		}

		logger.Info().Msg("gRPC server exited")
	}
}

func (a *Appchain[STI, appTx, R, AppBlock]) Shutdown() {
	if a.multichainDB != nil {
		a.multichainDB.Close()
	}

	if a.TxBatchDB != nil {
		a.TxBatchDB.Close()
	}

	if a.AppchainDB != nil {
		a.AppchainDB.Close()
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
func WriteExternalTransactions(
	dbTx kv.RwTx,
	blockNumber uint64,
	txs []apptypes.ExternalTransaction,
) ([32]byte, error) {
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

func TransactionToProto(
	txs []apptypes.ExternalTransaction,
	blockNumber uint64,
	root [32]byte,
) *emitterproto.GetExternalTransactionsResponse_BlockTransactions {
	protoTxs := make([]*emitterproto.ExternalTransaction, len(txs))

	for i, tx := range txs {
		protoTxs[i] = &emitterproto.ExternalTransaction{
			ChainId: uint64(tx.ChainID),
			Tx:      tx.Tx,
		}
	}

	return &emitterproto.GetExternalTransactionsResponse_BlockTransactions{
		BlockNumber:          blockNumber,
		TransactionsRootHash: root[:],
		ExternalTransactions: protoTxs,
	}
}

func CheckpointToProto(cp apptypes.Checkpoint) *emitterproto.CheckpointResponse_Checkpoint {
	return &emitterproto.CheckpointResponse_Checkpoint{
		LatestBlockNumber:  cp.BlockNumber,
		StateRoot:          cp.StateRoot[:],                // Преобразование массива в срез
		BlockHash:          cp.BlockHash[:],                // Преобразование массива в срез
		ExternalTxRootHash: cp.ExternalTransactionsRoot[:], // Преобразование массива в срез
	}
}

// todo: merklize transactions
func Merklize(_ []apptypes.ExternalTransaction) [32]byte {
	return [32]byte{}
}

func startPrometheusServer(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{Addr: addr, Handler: mux, ReadTimeout: 5 * time.Second}

	go func() {
		<-ctx.Done()
		// корректно останавливаем при отмене контекста
		_ = srv.Shutdown(context.Background()) //nolint:contextcheck //it's a shutdown
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Ctx(ctx).Panic().Err(err).Msg("metrics server crashed")
	}
}

// fixme надо писать чекпоинт в staged sync
// должен быть совместим с GetCheckpoints
func WriteCheckpoint(ctx context.Context, dbTx kv.RwTx, checkpoint apptypes.Checkpoint) error {
	logger := log.Ctx(ctx)

	// Генерируем ключ из LatestBlockNumber (8 байт)
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, checkpoint.BlockNumber)

	// Сериализуем чекпоинт
	value, err := cbor.Marshal(checkpoint)
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

func ReadTxSnapshotPosition(tx kv.Tx, epoch uint32) int64 {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val, err := tx.GetOne(TxSnapshot, key)
	if err != nil {
		log.Log().Err(err).Msgf("ReadTxSnapshotPosition: can't read tx. Epoch: %d", epoch)

		return 8 // fallback: пропускаем заголовок
	}

	if len(val) != 8 {
		return 8
	}

	return int64(binary.BigEndian.Uint64(val))
}

const currentEpoch = uint32(1)

func GetLastStreamPositions(
	ctx context.Context,
	appchainDB kv.RwDB,
) (startEventPos int64, startTxPos int64, err error) {
	startEventPos = int64(8)
	startTxPos = int64(8)

	err = appchainDB.View(ctx, func(tx kv.Tx) error {
		var dbErr error

		startEventPos, dbErr = ReadSnapshotPosition(tx, currentEpoch)
		if dbErr != nil {
			return dbErr
		}

		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("faild to get stream positions: %w", err)
	}

	return startEventPos, startTxPos, nil
}
