package rpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"


	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/block"
)

// BlockMethods provides comprehensive block-related RPC methods
type BlockMethods struct {
	block      apptypes.AppchainBlock
	appchainDB kv.RwDB
}

// NewBlockMethods creates a new set of block methods
func NewBlockMethods(appchainDB kv.RwDB) *BlockMethods {
	return &BlockMethods{appchainDB: appchainDB}
}

// TODO tests for all methods

// GetBlockByNumber retrieves a block by number (accepts decimal like "123", hex "0x7b", or a numeric JSON value).
func (m *BlockMethods) GetBlockByNumber(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		// Keep consistency with project-wide errors (mirrors existing style).
		return nil, ErrGetBlockByNumberRequires1Param
	}

	number, err := parseBlockNumber(params[0])
	if err != nil {
		return nil, err
	}

	var blk apptypes.AppchainBlock
	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var getErr error
		blk, getErr = block.GetBlock(tx, block.BlockNumberBucket, block.NumberToBytes(number), blk)
		return getErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get block by number %d: %w", number, err)
	}
	return blk, nil
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
		return nil, fmt.Errorf("%w: %w", ErrInvalidHashFormat, err)
	}

	var blk apptypes.AppchainBlock
	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var getErr error
		blk, getErr = block.GetBlock(tx, block.BlockHashBucket, hash[:], blk)
		return getErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get block by hash %s: %w", hashStr, err)
	}
	return blk, nil
}

// // GetLatestBlocks returns the N most recent blocks (newest first).
// // Parameter accepts decimal string, hex string, or numeric JSON.
// func (m *BlockMethods) GetLatestBlocks(ctx context.Context, params []any) (any, error) {
// 	if len(params) != 1 {
// 		return nil, ErrGetBlockByNumberRequires1Param
// 	}
// 	countU64, err := parseBlockNumber(params[0])
// 	if err != nil {
// 		return nil, err
// 	}
// 	if countU64 == 0 {
// 		return []apptypes.AppchainBlock{}, nil
// 	}
// 	count := int(countU64)

// 	var blocks []apptypes.AppchainBlock
// 	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
// 		var getErr error
// 		blocks, getErr = block.GetLatestBlocks(tx, count)
// 		return getErr
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get latest %d blocks: %w", count, err)
// 	}
// 	return blocks, nil
// }

// // GetTransactionsByBlockNumber retrieves transaction hashes belonging to a given block number.
// func (m *BlockMethods) GetTransactionsByBlockNumber(ctx context.Context, params []any) (any, error) {
// 	if len(params) != 1 {
// 		return nil, fmt.Errorf("getTransactionsByBlockNumber requires exactly 1 parameter")
// 	}

// 	number, err := parseBlockNumber(params[0])
// 	if err != nil {
// 		return nil, err
// 	}

// 	var txs []string
// 	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
// 		var getErr error
// 		txs, getErr = block.GetTransactionsByBlockNumber(tx, number)
// 		return getErr
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get transactions for block %d: %w", number, err)
// 	}
// 	return txs, nil
// }

// // GetTransactionsByBlockHash retrieves transaction hashes belonging to a given block hash.
// func (m *BlockMethods) GetTransactionsByBlockHash(ctx context.Context, params []any) (any, error) {
// 	if len(params) != 1 {
// 		return nil, fmt.Errorf("getTransactionsByBlockHash requires exactly 1 parameter")
// 	}

// 	hashStr, ok := params[0].(string)
// 	if !ok {
// 		return nil, ErrHashParameterMustBeString
// 	}

// 	hash, err := parseHash(hashStr)
// 	if err != nil {
// 		return nil, fmt.Errorf("%w: %w", ErrInvalidHashFormat, err)
// 	}

// 	var txs []string
// 	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
// 		var getErr error
// 		txs, getErr = block.GetTransactionsByBlockHash(tx, hash[:])
// 		return getErr
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get transactions for block %s: %w", hashStr, err)
// 	}
// 	return txs, nil
// }

// AddBlockMethods adds block-related methods to the RPC server.
func AddBlockMethods(server *StandardRPCServer, appchainDB kv.RwDB) {
	methods := NewBlockMethods(appchainDB)

	server.AddMethod("getBlockByNumber", methods.GetBlockByNumber)
	server.AddMethod("getBlockByHash", methods.GetBlockByHash)
	// server.AddMethod("getLatestBlocks", methods.GetLatestBlocks)
	// server.AddMethod("getTransactionsByBlockNumber", methods.GetTransactionsByBlockNumber)
	// server.AddMethod("getTransactionsByBlockHash", methods.GetTransactionsByBlockHash)
}

// parseBlockNumber converts a JSON-RPC parameter into a uint64 block number.
// Supports numeric values and strings in either decimal (e.g. "123") or hex ("0x7b").
func parseBlockNumber(v any) (uint64, error) {
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, fmt.Errorf("invalid block number: negative")
		}
		return uint64(n), nil
	case int:
		if n < 0 {
			return 0, fmt.Errorf("invalid block number: negative")
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("invalid block number: negative")
		}
		return uint64(n), nil
	case uint64:
		return n, nil
	case string:
		s := strings.TrimSpace(n)
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			ui, err := strconv.ParseUint(s[2:], 16, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid hex block number %q: %w", s, err)
			}
			return ui, nil
		}
		ui, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid decimal block number %q: %w", s, err)
		}
		return ui, nil
	default:
		return 0, fmt.Errorf("invalid block number param of type %T", v)
	}
}
