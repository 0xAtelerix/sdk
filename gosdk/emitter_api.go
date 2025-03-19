package gosdk

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	emitterproto "github.com/0xAtelerix/sdk/proto"
	"github.com/0xAtelerix/sdk/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server с поддержкой MDBX
type AppchainEmitterServer struct {
	emitterproto.UnimplementedEmitterServer
	appchainDB kv.RwDB
	chainID    uint64
}

// Создание нового сервера с MDBX
// todo add txpool?
func NewServer(db kv.RwDB, chainID uint64) *AppchainEmitterServer {
	return &AppchainEmitterServer{appchainDB: db, chainID: chainID}
}

// Метод GetCheckpoints: выбираем все чекпоинты >= LatestBlockNumber
func (s *AppchainEmitterServer) GetCheckpoints(ctx context.Context, req *emitterproto.GetCheckpointsRequest) (*emitterproto.CheckpointResponse, error) {
	fmt.Printf("Получен запрос: latest_previous_checkpoint_block_number=%d, limit=%d\n",
		req.LatestPreviousCheckpointBlockNumber, req.Limit)

	txn, err := s.appchainDB.BeginRo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия транзакции БД: %w", err)
	}
	defer txn.Rollback()

	cursor, err := txn.Cursor(checkpointBucket)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания курсора: %w", err)
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
			return nil, fmt.Errorf("ошибка десериализации чекпоинта: %w", err)
		}
		checkpoints = append(checkpoints, CheckpointToProto(*checkpoint))
		count++
	}

	return &emitterproto.CheckpointResponse{Checkpoints: checkpoints}, nil
}

// Метод GetExternalTransactions: выбираем все транзакции >= LatestPreviousBlockNumber
func (s *AppchainEmitterServer) GetExternalTransactions(ctx context.Context, req *emitterproto.GetExternalTransactionsRequest) (*emitterproto.GetExternalTransactionsResponse, error) {
	fmt.Printf("Получен запрос: latest_previous_block_number=%d, limit=%d\n",
		req.LatestPreviousBlockNumber, req.Limit)

	txn, err := s.appchainDB.BeginRo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия транзакции БД: %w", err)
	}
	defer txn.Rollback()

	cursor, err := txn.Cursor(externalTxBucket)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания курсора: %w", err)
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
			return nil, fmt.Errorf("ошибка десериализации транзакции: %w", err)
		}

		blockMap[blockNumber] = append(blockMap[blockNumber], tx)
		count++
	}

	// Формируем список блоков с транзакциями
	var blocks []*emitterproto.GetExternalTransactionsResponse_BlockTransactions
	for blockNumber, txs := range blockMap {
		blocks = append(blocks, &emitterproto.GetExternalTransactionsResponse_BlockTransactions{
			BlockNumber:          blockNumber,
			TransactionsRootHash: []byte("fake_tx_hash"), // Можно заменить на реальный хеш
			ExternalTransactions: txs,
		})
	}

	return &emitterproto.GetExternalTransactionsResponse{Blocks: blocks}, nil
}

func (s *AppchainEmitterServer) GetChainId(context.Context, *emptypb.Empty) (*emitterproto.GetChainIDResponse, error) {
	return &emitterproto.GetChainIDResponse{
		ChainId: s.chainID,
	}, nil
}
