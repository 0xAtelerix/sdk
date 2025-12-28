package gosdk

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

// Default configuration for appchain services.
const (
	DefaultDataDir     = "/data"
	DefaultEmitterPort = ":9090"
	DefaultRPCPort     = ":8080"
	DefaultAppchainID  = uint64(42)
)

// AppchainConfig holds runtime configuration for the appchain.
// This is created during InitApp and passed to NewAppchain.
type AppchainConfig struct {
	ChainID        uint64
	DataDir        string
	EmitterPort    string
	PrometheusPort string
	ValidatorID    string // Optional validator identifier for metrics/logs
	Logger         *zerolog.Logger
}

type Appchain[AppTx apptypes.AppTransaction[R], R apptypes.Receipt, AppBlock apptypes.AppchainBlock] struct {
	storage        *Storage[AppTx, R]
	config         *AppchainConfig
	batchProcessor BatchProcessor[AppTx, R]
	blockBuilder   apptypes.AppchainBlockConstructor[AppTx, R, AppBlock]
	rootCalculator apptypes.RootCalculator
	emitterAPI     emitterproto.EmitterServer
}

func NewAppchain[AppTx apptypes.AppTransaction[R],
	R apptypes.Receipt,
	AppBlock apptypes.AppchainBlock](
	storage *Storage[AppTx, R],
	config *AppchainConfig,
	batchProcessor BatchProcessor[AppTx, R],
	blockBuilder apptypes.AppchainBlockConstructor[AppTx, R, AppBlock],
	options ...func(a *Appchain[AppTx, R, AppBlock]),
) Appchain[AppTx, R, AppBlock] {
	emitterAPI := NewServer(storage.appchainDB, config.ChainID, storage.txPool)
	emitterAPI.logger = config.Logger

	appchain := Appchain[AppTx, R, AppBlock]{
		storage:        storage,
		config:         config,
		batchProcessor: batchProcessor,
		blockBuilder:   blockBuilder,
		rootCalculator: NewStubRootCalculator(),
		emitterAPI:     emitterAPI,
	}

	for _, option := range options {
		option(&appchain)
	}

	config.Logger.Info().Msg("Appchain initialized successfully")

	return appchain
}

func (a *Appchain[AppTx, R, AppBlock]) SetRootCalculator(rc apptypes.RootCalculator) {
	a.rootCalculator = rc
}

func (a *Appchain[AppTx, R, AppBlock]) Close() {
	if a.storage != nil {
		a.storage.Close()
	}
}

func (a *Appchain[AppTx, R, AppBlock]) Run(ctx context.Context) error {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Appchain run started")

	ctx = utility.CtxWithValidatorID(ctx, a.config.ValidatorID)
	ctx = utility.CtxWithChainID(ctx, a.config.ChainID)
	vid := utility.ValidatorIDFromCtx(ctx)
	cid := utility.ChainIDFromCtx(ctx)

	emitterErrChan := make(chan error)

	go a.runEmitterAPI(ctx, emitterErrChan)

	err := <-emitterErrChan
	if err != nil {
		return fmt.Errorf("emitter API failed to start: %w", err)
	}

	if a.config.PrometheusPort != "" {
		go startPrometheusServer(ctx, a.config.PrometheusPort)
	}

	startEventPos, epoch, err := GetLastStreamPositions(ctx, a.storage.appchainDB)
	if err != nil {
		return err
	}

	logger.Info().
		Str("dir", a.storage.eventStreamDir).
		Int64("start event", startEventPos).
		Uint32("epoch", epoch).
		Msg("Initializing event readers")

	err = WaitFile(ctx, a.storage.eventStreamDir, logger)
	if err != nil {
		return err
	}

	err = WaitFile(ctx, a.storage.txStreamDir, logger)
	if err != nil {
		return err
	}

	if a.storage.txBatchDB == nil {
		return ErrEmptyTxBatchDB
	}

	var (
		votingBlocks      *Voting[apptypes.ExternalBlock]
		votingCheckpoints *Voting[apptypes.Checkpoint]
	)

	err = a.storage.appchainDB.View(ctx, func(tx kv.Tx) error {
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

	eventStream, err := NewMdbxEventStreamWrapper[AppTx, R](
		a.storage.eventStreamDir,
		uint32(a.config.ChainID),
		a.storage.txBatchDB,
		logger,
		a.storage.appchainDB,
		a.storage.subscriber,
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

	err = a.storage.appchainDB.View(ctx, func(tx kv.Tx) error {
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

			start := time.Now()

			err = a.processBatch(ctx, batch, &previousBlockNumber, &previousBlockHash, eventStream, votingBlocks, votingCheckpoints, vid, cid)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to handle batch")

				return err
			}

			BlockProcessingDuration.WithLabelValues(vid, cid).Observe(time.Since(start).Seconds())
		}

		BatchProcessingDuration.WithLabelValues(vid, cid).Observe(time.Since(timer).Seconds())
	}

	return nil
}

func (a *Appchain[AppTx, R, AppBlock]) processBatch(
	ctx context.Context,
	batch apptypes.Batch[AppTx, R],
	previousBlockNumber *uint64,
	previousBlockHash *[32]byte,
	eventStream *MdbxEventStreamWrapper[AppTx, R],
	votingBlocks *Voting[apptypes.ExternalBlock],
	votingCheckpoints *Voting[apptypes.Checkpoint],
	vid, cid string,
) error {
	logger := log.Ctx(ctx)

	rwtx, err := a.storage.appchainDB.BeginRw(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to begin write transaction")

		return fmt.Errorf("failed to begin write tx: %w", err)
	}
	defer rwtx.Rollback()

	// Process Batch
	logger.Debug().Int("tx", len(batch.Transactions)).Msg("Process batch")

	processedReceipts, extTxs, err := a.batchProcessor.ProcessBatch(ctx, batch, rwtx)
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

	// Calculate state root
	stateRoot, err := a.rootCalculator.StateRootCalculator(rwtx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to calculate state root")

		return fmt.Errorf("failed to calculate state root: %w", err)
	}

	// Build block
	blockNumber := *previousBlockNumber + 1
	block := a.blockBuilder(blockNumber, stateRoot, *previousBlockHash, batch)

	// Store block
	logger.Debug().Uint64("block_number", blockNumber).Msg("Write block")

	blockBytes, marshalErr := cbor.Marshal(block)
	if marshalErr != nil {
		logger.Error().Err(marshalErr).Msg("Failed to marshal block")

		return fmt.Errorf("%w: %w", ErrBlockMarshalling, marshalErr)
	}

	if err = WriteBlock(rwtx, blockNumber, blockBytes); err != nil {
		logger.Error().Err(err).Msg("Failed to write block")

		return fmt.Errorf("%w: %w", ErrBlockWrite, err)
	}

	// Store transactions with relation to the block
	if err = WriteBlockTransactions(rwtx, blockNumber, batch.Transactions); err != nil {
		logger.Error().Err(err).Msg("Failed to write block transactions")

		return fmt.Errorf("%w: %w", ErrBlockTransactionsWrite, err)
	}

	blockHash := block.Hash()

	externalTXRoot, err := WriteExternalTransactions(rwtx, blockNumber, extTxs)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to write external transactions")

		return fmt.Errorf("failed to write external transactions: %w", err)
	}

	checkpoint := apptypes.Checkpoint{
		ChainID:                  a.config.ChainID,
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

	// Write voting
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

	ProcessedBlocks.WithLabelValues(vid, cid).Inc()
	ProcessedTransactions.WithLabelValues(vid, cid).Add(float64(len(batch.Transactions)))

	HeadBlockNumber.WithLabelValues(vid, cid).Set(float64(blockNumber))
	EventStreamPosition.
		WithLabelValues(vid, cid, strconv.FormatUint(uint64(eventStream.currentEpoch), 10)).
		Set(float64(batch.EndOffset))
	BlockExternalTxs.WithLabelValues(vid, cid).Observe(float64(len(extTxs)))
	BlockInternalTxs.WithLabelValues(vid, cid).Observe(float64(len(batch.Transactions)))
	BlockBytes.WithLabelValues(vid, cid).Observe(float64(len(blockBytes)))

	*previousBlockNumber = blockNumber
	*previousBlockHash = block.Hash()

	return nil
}

func (a *Appchain[AppTx, R, AppBlock]) runEmitterAPI(ctx context.Context, errCh chan error) {
	logger := log.Ctx(ctx)

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.config.EmitterPort)
	if err != nil {
		errCh <- fmt.Errorf("failed to create listener: %w, port %q", err, a.config.EmitterPort)

		return
	}

	close(errCh)

	logger.Info().Str("port", a.config.EmitterPort).Msg("Starting gRPC server")

	grpcServer := grpc.NewServer()
	emitterproto.RegisterEmitterServer(grpcServer, a.emitterAPI)
	emitterproto.RegisterHealthServer(grpcServer, &HealthServer{})

	serveErr := make(chan error, 1)

	go func() {
		serveErr <- grpcServer.Serve(lis)
	}()

	const gracePeriod = 10 * time.Second

	select {
	case <-ctx.Done():
		logger.Info().Err(ctx.Err()).Msg("Context canceled, shutting down gRPC server")

		done := make(chan struct{})

		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			logger.Info().Msg("gRPC server stopped gracefully")
		case <-time.After(gracePeriod):
			logger.Warn().
				Dur("timeout", gracePeriod).
				Msg("Graceful stop timed out; forcing stop")

			grpcServer.Stop()
		}

		<-serveErr

	case err = <-serveErr:
		if err != nil {
			logger.Panic().Err(err).Msg("gRPC server crashed")
		}

		logger.Info().Msg("gRPC server exited")
	}
}

func startPrometheusServer(ctx context.Context, addr string) {
	logger := log.Ctx(ctx)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{Addr: addr, Handler: mux, ReadTimeout: 5 * time.Second}

	go func() {
		<-ctx.Done()

		_ = srv.Shutdown(context.Background()) //nolint:contextcheck //it's a shutdown
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Panic().Err(err).Msg("metrics server crashed")
	}
}
