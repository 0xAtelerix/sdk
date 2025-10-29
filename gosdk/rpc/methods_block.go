package rpc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type explorerResponse struct {
	Fields []apptypes.ExplorerField `json:"fields"`
	Rows   [][]string               `json:"rows"`
}

type BlockExplorerMethods struct {
	appchainDB  kv.RwDB
	decodeBlock apptypes.ExplorerBlockDecoder
}

func NewBlockExplorerMethods(appchainDB kv.RwDB, decodeBlock apptypes.ExplorerBlockDecoder) *BlockExplorerMethods {
	if decodeBlock == nil {
		panic("block explorer decoder cannot be nil")
	}

	return &BlockExplorerMethods{
		appchainDB:  appchainDB,
		decodeBlock: decodeBlock,
	}
}

func (m *BlockExplorerMethods) GetBlockByNumber(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetBlockByNumberRequires1Param
	}

	number, err := parseUint64(params[0])
	if err != nil {
		return nil, err
	}

	block, err := m.loadBlockByNumber(ctx, number)
	if err != nil {
		return nil, err
	}

	return explorerResponse{
		Fields: block.ExplorerFields(),
		Rows:   [][]string{block.ExplorerValues()},
	}, nil
}

func (m *BlockExplorerMethods) GetBlockByHash(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetBlockByHashRequires1Param
	}

	hashStr, ok := params[0].(string)
	if !ok {
		return nil, ErrHashParameterMustBeString
	}

	hash, err := parseHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidHashFormat, err)
	}

	number, err := m.lookupBlockNumberByHash(ctx, hash[:])
	if err != nil {
		return nil, err
	}

	block, err := m.loadBlockByNumber(ctx, number)
	if err != nil {
		return nil, err
	}

	return explorerResponse{
		Fields: block.ExplorerFields(),
		Rows:   [][]string{block.ExplorerValues()},
	}, nil
}

func (m *BlockExplorerMethods) GetBlocks(ctx context.Context, params []any) (any, error) {
	if len(params) != 2 {
		return nil, ErrGetBlocksRequires2Params
	}

	start, err := parseUint64(params[0])
	if err != nil {
		return nil, err
	}

	limit, err := parseUint64(params[1])
	if err != nil {
		return nil, err
	}

	if limit == 0 {
		return nil, ErrLimitMustBePositive
	}

	blocks := make([]apptypes.ExplorerBlock, 0)

	for i := uint64(0); i < limit; i++ {
		block, loadErr := m.loadBlockByNumber(ctx, start+i)
		if loadErr != nil {
			if errors.Is(loadErr, ErrBlockNotFound) {
				break
			}

			return nil, loadErr
		}

		blocks = append(blocks, block)
	}

	if len(blocks) == 0 {
		return explorerResponse{Fields: []apptypes.ExplorerField{}, Rows: [][]string{}}, nil
	}

	rows := make([][]string, 0, len(blocks))
	for _, block := range blocks {
		rows = append(rows, block.ExplorerValues())
	}

	return explorerResponse{
		Fields: blocks[0].ExplorerFields(),
		Rows:   rows,
	}, nil
}

func (m *BlockExplorerMethods) GetBlockTransactions(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetBlockTransactionsRequires1Param
	}

	number, err := parseUint64(params[0])
	if err != nil {
		return nil, err
	}

	block, err := m.loadBlockByNumber(ctx, number)
	if err != nil {
		return nil, err
	}

	transactions := block.ExplorerTransactions()
	rows := make([][]string, 0, len(transactions))
	for _, tx := range transactions {
		rows = append(rows, tx.ExplorerValues())
	}

	fields := block.ExplorerTransactionFields()
	if len(rows) > 0 {
		fields = transactions[0].ExplorerFields()
	}

	return explorerResponse{
		Fields: fields,
		Rows:   rows,
	}, nil
}

func (m *BlockExplorerMethods) loadBlockByNumber(ctx context.Context, number uint64) (apptypes.ExplorerBlock, error) {
	var blockBytes []byte
	err := m.appchainDB.View(ctx, func(tx kv.Tx) error {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, number)

		value, getErr := tx.GetOne(gosdk.BlocksBucket, key)
		if getErr != nil {
			return getErr
		}

		if len(value) == 0 {
			return ErrBlockNotFound
		}

		blockBytes = append([]byte(nil), value...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	block, err := m.decodeBlock(blockBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode block %d: %w", number, err)
	}

	return block, nil
}

func (m *BlockExplorerMethods) lookupBlockNumberByHash(ctx context.Context, hash []byte) (uint64, error) {
	var number uint64
	err := m.appchainDB.View(ctx, func(tx kv.Tx) error {
		value, getErr := tx.GetOne(gosdk.BlockHashesBucket, hash)
		if getErr != nil {
			return getErr
		}

		if len(value) == 0 {
			return ErrBlockNotFound
		}

		number = binary.BigEndian.Uint64(value)

		return nil
	})
	if err != nil {
		return 0, err
	}

	return number, nil
}

func parseUint64(value any) (uint64, error) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, ErrBlockNumberMustBeUnsigned
		}
		if v < 0 {
			return 0, ErrBlockNumberMustBeUnsigned
		}
		if math.Trunc(v) != v {
			return 0, ErrBlockNumberMustBeUnsigned
		}

		return uint64(v), nil
	case int:
		if v < 0 {
			return 0, ErrBlockNumberMustBeUnsigned
		}

		return uint64(v), nil
	case uint64:
		return v, nil
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, ErrBlockNumberMustBeUnsigned
		}

		return parsed, nil
	default:
		return 0, ErrBlockNumberMustBeUnsigned
	}
}

func AddBlockExplorerMethods(server *StandardRPCServer, appchainDB kv.RwDB, decodeBlock apptypes.ExplorerBlockDecoder) {
	methods := NewBlockExplorerMethods(appchainDB, decodeBlock)

	server.AddMethod("getBlockByNumber", methods.GetBlockByNumber)
	server.AddMethod("getBlockByHash", methods.GetBlockByHash)
	server.AddMethod("getBlocks", methods.GetBlocks)
	server.AddMethod("getBlockTransactions", methods.GetBlockTransactions)
}
