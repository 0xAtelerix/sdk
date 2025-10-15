package rpc

import (
	"context"
	"strconv"
	"strings"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/block"
)

// BlockMethods provides comprehensive block-related RPC methods
type BlockMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	appchainDB kv.RwDB
}

// NewBlockMethods creates a new set of block methods
func NewBlockMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	appchainDB kv.RwDB,
) *BlockMethods[appTx, R] {
	return &BlockMethods[appTx, R]{appchainDB: appchainDB}
}

// GetBlockByNumber retrieves a block by number (accepts decimal like "123", hex "0x7b", or a numeric JSON value).
func (m *BlockMethods[appTx, R]) GetBlockByNumber(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		// Keep consistency with project-wide errors (mirrors existing style).
		return nil, ErrGetBlockByNumberRequires1Param
	}

	number, err := parseNumber(params[0])
	if err != nil {
		return nil, err
	}

	var blockFieldsValuesResp block.FieldsValues

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var getErr error

		blockFieldsValuesResp, getErr = block.GetBlock(
			tx,
			block.BlockNumberBucket,
			block.NumberToBytes(number),
			nil,
		)

		return getErr
	})
	if err != nil {
		return nil, ErrFailedToGetBlockByNumber
	}

	return blockFieldsValuesResp, nil
}

// GetBlockByHash retrieves a block by its hash (expects a 0x-prefixed hex string).
func (m *BlockMethods[appTx, R]) GetBlockByHash(ctx context.Context, params []any) (any, error) {
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

	var blockFieldsValuesResp block.FieldsValues

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var getErr error

		blockFieldsValuesResp, getErr = block.GetBlock(tx, block.BlockHashBucket, hash[:], nil)

		return getErr
	})
	if err != nil {
		return nil, ErrFailedToGetBlockByHash
	}

	return blockFieldsValuesResp, nil
}

// GetBlocks returns the N most recent blocks (newest first).
func (m *BlockMethods[appTx, R]) GetBlocks(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetBlockByNumberRequires1Param
	}

	countU64, err := parseNumber(params[0])
	if err != nil {
		return nil, err
	}

	var result []block.FieldsValues

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var e error

		result, e = block.GetBlocks(tx, countU64)

		return e
	})
	if err != nil {
		return nil, ErrFailedToGetLatestBlocks
	}

	return result, nil
}

// GetTransactionsByBlockNumber retrieves transactions for a given block number.
// Accepts decimal, hex "0x..", or numeric JSON; returns (any, error).
func (m *BlockMethods[appTx, R]) GetTransactionsByBlockNumber(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetBlockByNumberRequires1Param
	}

	blockNum, err := parseNumber(params[0])
	if err != nil {
		return nil, err
	}

	var transactions []appTx

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var zero appTx

		var viewErr error

		transactions, viewErr = block.GetTransactionsByBlockNumber(tx, blockNum, zero)

		return viewErr
	})
	if err != nil {
		return nil, ErrFailedToGetTransactionsByBlockNumber
	}

	return transactions, nil
}

// GetTransactionsByBlockHash retrieves transactions for a given block hash.
func (m *BlockMethods[appTx, R]) GetTransactionsByBlockHash(
	ctx context.Context,
	params []any,
) (any, error) {
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

	var transactions []appTx

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		var zero appTx

		var viewErr error

		transactions, viewErr = block.GetTransactionsByBlockHash(tx, hash, zero)

		return viewErr
	})
	if err != nil {
		return nil, ErrFailedToGetTransactionsByBlockHash
	}

	return transactions, nil
}

// AddBlockMethods adds block-related methods to the RPC server.
func AddBlockMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	server *StandardRPCServer,
	appchainDB kv.RwDB,
) {
	methods := NewBlockMethods[appTx](appchainDB)

	server.AddMethod("getBlockByNumber", methods.GetBlockByNumber)
	server.AddMethod("getBlockByHash", methods.GetBlockByHash)
	server.AddMethod("getBlocks", methods.GetBlocks)
	server.AddMethod("getTransactionsByBlockNumber", methods.GetTransactionsByBlockNumber)
	server.AddMethod("getTransactionsByBlockHash", methods.GetTransactionsByBlockHash)
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
