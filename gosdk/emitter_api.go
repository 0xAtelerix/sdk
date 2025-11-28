package gosdk

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

// Server с поддержкой MDBX
type AppchainEmitterServer[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	emitterproto.UnimplementedEmitterServer

	appchainDB kv.RwDB
	chainID    uint64
	txpool     apptypes.TxPoolInterface[appTx, R]
	logger     *zerolog.Logger
}

// Создание нового сервера с MDBX
func NewServer[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	db kv.RwDB,
	chainID uint64,
	txpool apptypes.TxPoolInterface[appTx, R],
) *AppchainEmitterServer[appTx, R] {
	return &AppchainEmitterServer[appTx, R]{
		appchainDB: db,
		chainID:    chainID,
		txpool:     txpool,
		logger:     &log.Logger,
	}
}

// Метод GetCheckpoints: выбираем все чекпоинты >= LatestBlockNumber
func (s *AppchainEmitterServer[appTx, R]) GetCheckpoints(
	ctx context.Context,
	req *emitterproto.GetCheckpointsRequest,
) (*emitterproto.CheckpointResponse, error) {
	s.logger.Debug().
		Str("method", "GetCheckpoints").
		Uint64("latest_previous_checkpoint_block_number", req.GetLatestPreviousCheckpointBlockNumber()).
		Uint32("limit", func() uint32 {
			if req.Limit != nil {
				return req.GetLimit()
			}

			return 0
		}()).
		Msg("Received request")

	txn, err := s.appchainDB.BeginRo(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to open DB transaction")

		return nil, fmt.Errorf("failed to open DB transaction: %w", err)
	}

	defer txn.Rollback()

	cursor, err := txn.Cursor(CheckpointBucket)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create cursor")

		return nil, fmt.Errorf("failed to create cursor: %w", err)
	}

	defer cursor.Close()

	var checkpoints []*emitterproto.CheckpointResponse_Checkpoint

	limit := uint32(100)
	if req.Limit != nil {
		limit = req.GetLimit()
	}

	// Начинаем поиск с блока >= LatestPreviousCheckpointBlockNumber
	startKey := make([]byte, 8)
	binary.BigEndian.PutUint64(startKey, req.GetLatestPreviousCheckpointBlockNumber())

	count := uint32(0)
	for k, v, err := cursor.Seek(startKey); err == nil && count < limit; k, v, err = cursor.Next() {
		if len(k) == 0 {
			break // Дошли до конца
		}

		checkpoint := &apptypes.Checkpoint{}
		if err = cbor.Unmarshal(v, &checkpoint); err != nil {
			s.logger.Error().Err(err).Msg("Checkpoint deserialization failed")

			return nil, fmt.Errorf("checkpoint deserialization failed: %w", err)
		}

		checkpoints = append(checkpoints, CheckpointToProto(*checkpoint))
		count++
	}

	if len(checkpoints) > 0 {
		s.logger.Debug().
			Str("method", "GetCheckpoints").
			Uint64("last checkpoint", checkpoints[len(checkpoints)-1].GetLatestBlockNumber()).
			Msg("New checkpoints")
	} else {
		s.logger.Debug().
			Str("method", "GetCheckpoints").
			Msg("No new checkpoints")
	}

	return &emitterproto.CheckpointResponse{Checkpoints: checkpoints}, nil
}

// Метод GetExternalTransactions: выбираем все транзакции >= LatestPreviousBlockNumber
func (s *AppchainEmitterServer[appTx, R]) GetExternalTransactions(
	ctx context.Context,
	req *emitterproto.GetExternalTransactionsRequest,
) (*emitterproto.GetExternalTransactionsResponse, error) {
	s.logger.Debug().
		Str("method", "GetExternalTransactions").
		Uint64("latest_previous_block_number", req.GetLatestPreviousBlockNumber()).
		Uint32("limit", func() uint32 {
			if req.Limit != nil {
				return req.GetLimit()
			}

			return 0
		}()).
		Msg("Received request")

	txn, err := s.appchainDB.BeginRo(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to open DB transaction")

		return nil, fmt.Errorf("failed to open DB transaction: %w", err)
	}

	defer txn.Rollback()

	cursor, err := txn.Cursor(ExternalTxBucket)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create cursor")

		return nil, fmt.Errorf("failed to create cursor: %w", err)
	}

	defer cursor.Close()

	limit := uint32(100)
	if req.Limit != nil {
		limit = req.GetLimit()
	}

	// Начинаем поиск с блока >= LatestPreviousBlockNumber
	startKey := make([]byte, 10)
	binary.BigEndian.PutUint64(startKey[:8], req.GetLatestPreviousBlockNumber())

	s.logger.Warn().
		Str("GetExternalTransactions", strconv.FormatUint(req.GetLatestPreviousBlockNumber(), 10))

	blockMap := make(map[uint64][]*emitterproto.ExternalTransaction)
	count := uint32(0)

	for k, v, err := cursor.Seek(startKey); err == nil && count < limit; k, v, err = cursor.Next() {
		if len(k) != 8 {
			break
		}

		blockNumber := binary.BigEndian.Uint64(k[:8])

		// Unmarshal CBOR directly to ExternalTransaction array
		var txs []apptypes.ExternalTransaction
		if err := cbor.Unmarshal(v, &txs); err != nil {
			s.logger.Error().Err(err).Msg("Transaction deserialization failed")

			return nil, fmt.Errorf("transaction deserialization failed: %w", err)
		}

		// Convert to proto format for response
		protoTxs := make([]*emitterproto.ExternalTransaction, len(txs))
		for i, tx := range txs {
			protoTxs[i] = &emitterproto.ExternalTransaction{
				ChainId: uint64(tx.ChainID),
				Tx:      tx.Tx,
			}
		}

		blockMap[blockNumber] = append(blockMap[blockNumber], protoTxs...)
		count++
	}

	if len(blockMap) == 0 {
		s.logger.Debug().
			Str("method", "GetExternalTransactions").
			Msg("No new external transactions")

		return nil, nil //nolint:nilnil // it's a loop
	}

	// Формируем список блоков с транзакциями
	blocks := make(
		[]*emitterproto.GetExternalTransactionsResponse_BlockTransactions,
		0,
		len(blockMap),
	)

	for blockNumber, txs := range blockMap {
		var rawTxs [][]byte
		for _, tx := range txs {
			rawTxs = append(rawTxs, tx.GetTx())
		}

		// Генерация корректного хеша
		txsFlat, err := utility.Flatten(rawTxs)
		if err != nil {
			return nil, fmt.Errorf("transaction flatten failed: %w", err)
		}

		hash := sha256.Sum256(txsFlat)

		blocks = append(blocks, &emitterproto.GetExternalTransactionsResponse_BlockTransactions{
			BlockNumber:          blockNumber,
			TransactionsRootHash: hash[:], // todo Нужно заменить на реальный root
			ExternalTransactions: txs,
		})

		s.logger.Debug().
			Uint64("block", blockNumber).
			Int("txCount", len(txs)).
			Str("hash", hex.EncodeToString(hash[:])).
			Msg("Generated hash for external transactions block")
	}

	return &emitterproto.GetExternalTransactionsResponse{Blocks: blocks}, nil
}

func (s *AppchainEmitterServer[appTx, R]) GetChainID(
	context.Context,
	*emptypb.Empty,
) (*emitterproto.GetChainIDResponse, error) {
	return &emitterproto.GetChainIDResponse{
		ChainId: s.chainID,
	}, nil
}

func (s *AppchainEmitterServer[appTx, R]) CreateInternalTransactionsBatch(
	ctx context.Context,
	_ *emptypb.Empty,
) (*emitterproto.CreateInternalTransactionsBatchResponse, error) {
	s.logger.Debug().
		Str("method", "CreateInternalTransactionsBatch").
		Msg("Received request")

	batchHash, txs, err := s.txpool.CreateTransactionBatch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	if len(txs) == 0 {
		s.logger.Debug().
			Str("method", "CreateInternalTransactionsBatch").
			Msg("No new transactions")

		return nil, nil //nolint:nilnil // it's a loop
	}

	txsBytes := make([]*emitterproto.ByteArray, len(txs))
	for i := range txs {
		txsBytes[i] = &emitterproto.ByteArray{Data: txs[i]}
	}

	resp := &emitterproto.CreateInternalTransactionsBatchResponse{
		BatchHash:            batchHash,
		InternalTransactions: txsBytes,
	}
	s.logger.Debug().
		Str("batch hash", hex.EncodeToString(resp.GetBatchHash())).
		Int("num of tx", len(resp.GetInternalTransactions())).
		Msg("New transaction batch")

	return resp, nil
}
