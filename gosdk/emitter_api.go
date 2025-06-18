package gosdk

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
	"github.com/0xAtelerix/sdk/gosdk/types"
	"github.com/0xAtelerix/sdk/gosdk/utility"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server с поддержкой MDBX
type AppchainEmitterServer[appTx types.AppTransaction] struct {
	emitterproto.UnimplementedEmitterServer
	appchainDB kv.RwDB
	chainID    uint64
	txpool     types.TxPoolInterface[appTx]
	logger     *zerolog.Logger
}

// Создание нового сервера с MDBX
func NewServer[appTx types.AppTransaction](db kv.RwDB, chainID uint64, txpool types.TxPoolInterface[appTx]) *AppchainEmitterServer[appTx] {
	return &AppchainEmitterServer[appTx]{appchainDB: db, chainID: chainID, txpool: txpool, logger: &log.Logger}
}

// Метод GetCheckpoints: выбираем все чекпоинты >= LatestBlockNumber
func (s *AppchainEmitterServer[appTx]) GetCheckpoints(ctx context.Context, req *emitterproto.GetCheckpointsRequest) (*emitterproto.CheckpointResponse, error) {
	s.logger.Debug().
		Str("method", "GetCheckpoints").
		Uint64("latest_previous_checkpoint_block_number", req.LatestPreviousCheckpointBlockNumber).
		Uint32("limit", func() uint32 {
			if req.Limit != nil {
				return *req.Limit
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
	limit := uint32(10)
	if req.Limit != nil {
		limit = *req.Limit
	}

	// Начинаем поиск с блока >= LatestPreviousCheckpointBlockNumber
	startKey := make([]byte, 8)
	binary.BigEndian.PutUint64(startKey, req.LatestPreviousCheckpointBlockNumber)

	count := uint32(0)
	for k, v, err := cursor.Seek(startKey); err == nil && count < limit; k, v, err = cursor.Next() {

		if len(k) == 0 {
			break // Дошли до конца
		}
		checkpoint := &types.Checkpoint{}
		if err := json.Unmarshal(v, &checkpoint); err != nil {
			s.logger.Error().Err(err).Msg("Checkpoint deserialization failed")
			return nil, fmt.Errorf("checkpoint deserialization failed: %w", err)
		}
		checkpoints = append(checkpoints, CheckpointToProto(*checkpoint))
		count++
	}
	if len(checkpoints) > 0 {
		s.logger.Debug().
			Str("method", "GetCheckpoints").
			Uint64("last checkpoint", checkpoints[len(checkpoints)-1].LatestBlockNumber).
			Msg("New checkpoints")

	} else {
		s.logger.Debug().
			Str("method", "GetCheckpoints").
			Msg("No new checkpoints")
	}

	return &emitterproto.CheckpointResponse{Checkpoints: checkpoints}, nil
}

// Метод GetExternalTransactions: выбираем все транзакции >= LatestPreviousBlockNumber
func (s *AppchainEmitterServer[appTx]) GetExternalTransactions(ctx context.Context, req *emitterproto.GetExternalTransactionsRequest) (*emitterproto.GetExternalTransactionsResponse, error) {
	s.logger.Debug().
		Str("method", "GetExternalTransactions").
		Uint64("latest_previous_block_number", req.LatestPreviousBlockNumber).
		Uint32("limit", func() uint32 {
			if req.Limit != nil {
				return *req.Limit
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

	limit := uint32(10)
	if req.Limit != nil {
		limit = *req.Limit
	}

	// Начинаем поиск с блока >= LatestPreviousBlockNumber
	startKey := make([]byte, 10)
	binary.BigEndian.PutUint64(startKey[:8], req.LatestPreviousBlockNumber)

	blockMap := make(map[uint64][]*emitterproto.ExternalTransaction)
	count := uint32(0)

	for k, v, err := cursor.Seek(startKey); err == nil && count < limit; k, v, err = cursor.Next() {
		if len(k) < 10 {
			break
		}

		blockNumber := binary.BigEndian.Uint64(k[:8])

		tx := &emitterproto.ExternalTransaction{}
		if err := proto.Unmarshal(v, tx); err != nil {
			s.logger.Error().Err(err).Msg("Transaction deserialization failed")
			return nil, fmt.Errorf("transaction deserialization failed: %w", err)
		}

		blockMap[blockNumber] = append(blockMap[blockNumber], tx)
		count++
	}

	if len(blockMap) == 0 {
		s.logger.Debug().
			Str("method", "GetExternalTransactions").
			Msg("No new external transactions")

		return nil, nil
	}

	// Формируем список блоков с транзакциями
	var blocks []*emitterproto.GetExternalTransactionsResponse_BlockTransactions
	for blockNumber, txs := range blockMap {
		var rawTxs [][]byte
		for _, tx := range txs {
			rawTxs = append(rawTxs, tx.Tx)
		}
		// Генерация корректного хеша
		txsFlat := utility.Flatten(rawTxs)
		hash := sha256.Sum256(txsFlat)

		blocks = append(blocks, &emitterproto.GetExternalTransactionsResponse_BlockTransactions{
			BlockNumber:          blockNumber,
			TransactionsRootHash: hash[:], //todo Нужно заменить на реальный root
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

func (s *AppchainEmitterServer[appTx]) GetChainId(context.Context, *emptypb.Empty) (*emitterproto.GetChainIDResponse, error) {
	return &emitterproto.GetChainIDResponse{
		ChainId: s.chainID,
	}, nil
}

func (s *AppchainEmitterServer[appTx]) CreateInternalTransactionsBatch(context.Context, *emptypb.Empty) (*emitterproto.CreateInternalTransactionsBatchResponse, error) {
	s.logger.Debug().
		Str("method", "CreateInternalTransactionsBatch").
		Msg("Received request")

	batchHash, txs, err := s.txpool.CreateTransactionBatch()
	if err != nil {
		return nil, fmt.Errorf("Failed to get transactions: %w", err)
	}
	if len(txs) == 0 {
		s.logger.Debug().
			Str("method", "CreateInternalTransactionsBatch").
			Msg("No new transactions")
		return nil, nil
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
		Str("batch hash", hex.EncodeToString(resp.BatchHash)).
		Int("num of tx", len(resp.InternalTransactions)).
		Msg("New transaction batch")

	return resp, nil
}
