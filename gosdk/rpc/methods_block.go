package rpc

import (
	"context"
	"strconv"
	"strings"

	"github.com/ledgerwatch/erigon-lib/kv"
	//"github.com/fxamacker/cbor/v2"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/block"
	//"github.com/0xAtelerix/sdk/gosdk/txpool"
)

// BlockMethods provides comprehensive block-related RPC methods
type BlockMethods struct {
	block      block.Block
	appchainDB kv.RwDB
}

// NewBlockMethods creates a new set of block methods
func NewBlockMethods(appchainDB kv.RwDB) *BlockMethods {
	return &BlockMethods{appchainDB: appchainDB}
}

// GetBlockByNumber retrieves a block by number (accepts decimal like "123", hex "0x7b", or a numeric JSON value).
func (m *BlockMethods) GetBlockByNumber(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		// Keep consistency with project-wide errors (mirrors existing style).
		return nil, ErrGetBlockByNumberRequires1Param
	}

	number, err := parseNumber(params[0])
	if err != nil {
		return nil, err
	}

	var (
		blockFieldsValuesResp block.BlockFieldsValues
		blockI                apptypes.AppchainBlock
	)
	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var getErr error
		blockFieldsValuesResp, getErr = block.GetBlock(tx, block.BlockNumberBucket, block.NumberToBytes(number), blockI)
		return getErr
	})
	if err != nil {
		return nil, ErrFailedToGetBlockByNumber
	}
	return blockFieldsValuesResp, nil
}

// GetBlockByHash retrieves a block by its hash (expects a 0x-prefixed hex string).
func (m *BlockMethods) GetBlockByHash(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetBlockByNumberRequires1Param
	}

	hashStr, ok := params[0].(string)
	if !ok {
		return nil, ErrHashParameterMustBeString
	}

	hash, err := parseHash(hashStr)
	if err != nil {
		return nil, ErrInvalidHashFormat
	}

	var (
		blockFieldsValuesResp block.BlockFieldsValues
		blockI                apptypes.AppchainBlock
	)
	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var getErr error
		blockFieldsValuesResp, getErr = block.GetBlock(tx, block.BlockHashBucket, hash[:], blockI)
		return getErr
	})
	if err != nil {
		return nil, ErrFailedToGetBlockByHash
	}
	return blockFieldsValuesResp, nil
}

// GetBlocks returns the N most recent blocks (newest first).
// The parameter accepts decimal string, hex string, or a numeric JSON value.
// The result is []block.BlockFieldsValues (wrapped as `any`) with aligned Fields/Values for each block.
// func (m *BlockMethods) GetBlocks(ctx context.Context, params []any) (any, error) {
// 	if len(params) != 1 {
// 		return nil, ErrGetBlockByNumberRequires1Param
// 	}
// 	countU64, err := parseNumber(params[0])
// 	if err != nil {
// 		return nil, err
// 	}

// 	var result any
// 	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
// 		var e error
// 		result, e = block.GetBlocks(tx, countU64)
// 		return e
// 	})
// 	if err != nil {
// 		return nil, ErrFailedToGetLatestBlocks
// 	}
// 	return result, nil
// }

// GetTransactionsByBlockNumber retrieves transactions for a given block number.
// Accepts decimal, hex "0x..", or numeric JSON; returns (any, error).
// func (m *BlockMethods) GetTransactionsByBlockNumber(ctx context.Context, params []any) (any, error) {
// 	if len(params) != 1 {
// 		return nil, ErrGetBlockByNumberRequires1Param
// 	}
// 	num, err := parseNumber(params[0])
// 	if err != nil {
// 		return nil, err
// 	}

// 	var result any
// 	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
// 		var e error
// 		result, e = block.GetTransactionsByBlockNumber(tx, num)
// 		return e
// 	})
// 	if err != nil {
// 		return nil, ErrFailedToGetTransactionsByBlockNumber
// 	}
// 	return result, nil
// }

// AddBlockMethods adds block-related methods to the RPC server.
func AddBlockMethods(server *StandardRPCServer, appchainDB kv.RwDB) {
	methods := NewBlockMethods(appchainDB)

	server.AddMethod("getBlockByNumber", methods.GetBlockByNumber)
	server.AddMethod("getBlockByHash", methods.GetBlockByHash)
	//server.AddMethod("getBlocks", methods.GetBlocks)
	//server.AddMethod("getTransactionsByBlockNumber", methods.GetTransactionsByBlockNumber)
}

// parseNumber converts a JSON-RPC parameter into a uint64 block number.
// Supports numeric values and strings in either decimal (e.g. "123") or hex ("0x7b").
// TODO consider to remove certain cases if they are not needed
func parseNumber(v any) (uint64, error) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, ErrNegativeBlockNumber
		}
		return uint64(n), nil
	case int:
		if n < 0 {
			return 0, ErrNegativeBlockNumber
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, ErrNegativeBlockNumber
		}
		return uint64(n), nil
	case uint64:
		return n, nil
	case string:
		s := strings.TrimSpace(n)
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			ui, err := strconv.ParseUint(s[2:], 16, 64)
			if err != nil {
				return 0, ErrInvalidBlockNumber
			}
			return ui, nil
		}
		ui, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, ErrInvalidBlockNumber
		}
		return ui, nil
	default:
		return 0, ErrInvalidBlockNumber
	}
}
