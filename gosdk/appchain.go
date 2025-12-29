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
	"strconv"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/library/subscriber"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
	"github.com/0xAtelerix/sdk/gosdk/scheme"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

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
	sub *subscriber.Subscriber,
	multichain MultichainStateAccessor,
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
		subscriber:             sub,
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
	multichainDB MultichainStateAccessor
	subscriber   *subscriber.Subscriber
}

func (a *Appchain[STI, appTx, R, AppBlock]) Run(
	ctx context.Context,
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

	startEventPos, epoch, err := GetLastStreamPositions(ctx, a.AppchainDB)
	if err != nil {
		return err
	}

	logger.Info().
		Str("dir", a.config.EventStreamDir).
		Int64("start event", startEventPos).
		Uint32("epoch", epoch).
		Msg("Initializing event readers")

	err = WaitFile(ctx, a.config.EventStreamDir, logger)
	if err != nil {
		return err
	}

	err = WaitFile(ctx, a.config.TxStreamDir, logger)
	if err != nil {
		return err
	}

	if a.TxBatchDB == nil {
		return library.ErrEmptyTxBatchDB
	}

	var (
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

	eventStream, err := NewMdbxEventStreamWrapper[appTx, R](
		a.config.EventStreamDir,
		uint32(a.config.ChainID),
		a.TxBatchDB,
		logger,
		a.AppchainDB,
		votingBlocks,
		votingCheckpoints,
	)
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
			logger.Debug().Int("batch", i).Int("tx", len(batch.Transactions)).Int("blocks", len(batch.ExternalBlocks)).Msg("received new batch")

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

				blockBytes, marshalErr := cbor.Marshal(block)
				if marshalErr != nil {
					logger.Error().Err(marshalErr).Msg("Failed to marshal block")

					return fmt.Errorf("%w: %w", library.ErrBlockMarshalling, marshalErr)
				}

				if err = WriteBlock(rwtx, blockNumber, blockBytes); err != nil {
					logger.Error().Err(err).Msg("Failed to write block")

					return fmt.Errorf("%w: %w", library.ErrBlockWrite, err)
				}

				// Store transactions with relation to the block
				if err = WriteBlockTransactions(rwtx, blockNumber, batch.Transactions); err != nil {
					logger.Error().Err(err).Msg("Failed to write block transactions")

					return fmt.Errorf("%w: %w", library.ErrBlockTransactionsWrite, err)
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

				err = WriteSnapshotPosition(rwtx, eventStream.currentEpoch, batch.EndOffset)
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
					WithLabelValues(vid, cid, strconv.FormatUint(uint64(eventStream.currentEpoch), 10)).
					Set(float64(batch.EndOffset))
				BlockExternalTxs.WithLabelValues(vid, cid).Observe(float64(len(extTxs)))
				BlockInternalTxs.WithLabelValues(vid, cid).Observe(float64(len(batch.Transactions)))
				BlockBytes.WithLabelValues(vid, cid).Observe(float64(len(blockBytes)))

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

func WaitFile(ctx context.Context, filePath string, logger *zerolog.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := os.Stat(filePath)
		if err != nil {
			logger.Warn().Err(err).Str("file", filePath).Msg("waiting file")
			time.Sleep(5 * time.Second)

			continue
		}

		break
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

	return rwtx.Put(scheme.BlocksBucket, number, blockBytes)
}

// WriteBlockTransactions stores the transactions for a block in CBOR format.
// Storage strategy: Transactions are stored once in BlockTransactionsBucket,
// while TxLookupBucket maintains a lightweight index (txHash -> blockNumber + txIndex).
func WriteBlockTransactions[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	rwtx kv.RwTx,
	blockNumber uint64,
	txs []appTx,
) error {
	blockNumBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blockNumBytes, blockNumber)

	// Marshal transactions to CBOR - this is the only place we store full transaction data
	txsBytes, err := cbor.Marshal(txs)
	if err != nil {
		return fmt.Errorf("%w: %w", library.ErrTransactionsMarshalling, err)
	}

	// Store in BlockTransactionsBucket (primary storage)
	if err := rwtx.Put(scheme.BlockTransactionsBucket, blockNumBytes, txsBytes); err != nil {
		return fmt.Errorf("%w: %w", library.ErrBlockTransactionsWrite, err)
	}

	// Create lookup entries: txHash -> (blockNumber, txIndex)
	for i, tx := range txs {
		txHash := tx.Hash()

		// Encode: blockNumber (8 bytes) + txIndex (4 bytes)
		lookupEntry := make([]byte, 12)
		binary.BigEndian.PutUint64(lookupEntry[0:8], blockNumber)
		binary.BigEndian.PutUint32(lookupEntry[8:12], uint32(i))

		if err := rwtx.Put(scheme.TxLookupBucket, txHash[:], lookupEntry); err != nil {
			return fmt.Errorf("%w (tx %x): %w", library.ErrTransactionLookupWrite, txHash[:4], err)
		}
	}

	return nil
}

func WriteLastBlock(rwtx kv.RwTx, number uint64, hash [32]byte) error {
	value := make([]byte, 8+32)
	binary.BigEndian.PutUint64(value[:8], number)
	copy(value[8:], hash[:])

	return rwtx.Put(scheme.ConfigBucket, []byte(scheme.LastBlockKey), value)
}

func GetLastBlock(tx kv.Tx) (uint64, [32]byte, error) {
	value, err := tx.GetOne(scheme.ConfigBucket, []byte(scheme.LastBlockKey))
	if err != nil {
		return 0, [32]byte{}, err
	}

	if len(value) != 8+32 {
		return 0, [32]byte{}, nil
	}

	number := binary.BigEndian.Uint64(value[:8])

	return number, ([32]byte)(value[8:]), err
}

// WriteExternalTransactions writes external transactions to the database in CBOR format.
// Should be called strictly once per block
func WriteExternalTransactions(
	dbTx kv.RwTx,
	blockNumber uint64,
	txs []apptypes.ExternalTransaction,
) ([32]byte, error) {
	root := Merklize(txs)

	// Marshal to CBOR (used by both validators and explorer)
	value, err := cbor.Marshal(txs)
	if err != nil {
		log.Error().Err(err).Msg("Transaction serialization failed")

		return [32]byte{}, fmt.Errorf("transaction serialization failed: %w", err)
	}

	// Write to database
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNumber)

	if err = dbTx.Put(scheme.ExternalTxBucket, key, value); err != nil {
		return [32]byte{}, fmt.Errorf("can't write external transactions to the DB: error %w", err)
	}

	// todo: store the root in DB
	return root, nil
}

// ReadExternalTransactions reads external transactions from the database.
// This is used by both the validator/emitter API and the block explorer RPC.
func ReadExternalTransactions(
	tx kv.Tx,
	blockNumber uint64,
) ([]apptypes.ExternalTransaction, error) {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNumber)

	value, err := tx.GetOne(scheme.ExternalTxBucket, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", library.ErrExternalTransactionsGet, err)
	}

	if len(value) == 0 {
		return []apptypes.ExternalTransaction{}, nil
	}

	var txs []apptypes.ExternalTransaction
	if err := cbor.Unmarshal(value, &txs); err != nil {
		return nil, fmt.Errorf("%w: %w", library.ErrExternalTransactionsUnmarshal, err)
	}

	return txs, nil
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
	return dbTx.Put(scheme.CheckpointBucket, key, value)
}

func WriteSnapshotPosition(rwtx kv.RwTx, epoch uint32, pos int64) error {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, uint64(pos))

	return rwtx.Put(scheme.Snapshot, key, val)
}

func ReadSnapshotPosition(tx kv.Tx, epoch uint32) (int64, error) {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, epoch)

	val, err := tx.GetOne(scheme.Snapshot, key)
	if err != nil {
		return 0, err
	}

	if len(val) != 8 {
		return 0, nil // default to beginning
	}

	return int64(binary.BigEndian.Uint64(val)), nil
}

func GetLastStreamPositions(
	ctx context.Context,
	appchainDB kv.RwDB,
) (int64, uint32, error) {
	startEventPos := int64(8)
	epoch := uint32(1)

	err := appchainDB.View(ctx, func(tx kv.Tx) error {
		c, err := tx.Cursor(scheme.Snapshot)
		if err != nil {
			return err
		}

		k, v, err := c.Last()
		if err != nil {
			return err
		}

		if len(k) != 4 {
			return nil
		}

		epoch = binary.BigEndian.Uint32(k)

		if len(v) != 8 {
			return nil
		}

		startEventPos = int64(binary.BigEndian.Uint64(v))

		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("faild to get stream positions: %w", err)
	}

	return startEventPos, epoch, nil
}
