package rpc

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/chainblock"
)

const defaultChainBlockBucket = chainblock.Bucket

type ChainBlockMethods struct {
	appchainDB kv.RwDB
	bucket     string
}

func NewChainBlockMethods(appchainDB kv.RwDB) *ChainBlockMethods {
	return &ChainBlockMethods{
		appchainDB: appchainDB,
		bucket:     defaultChainBlockBucket,
	}
}

// GetChainBlockByNumber retrieves a chain block by its block number.
func (m *ChainBlockMethods) GetChainBlockByNumber(ctx context.Context, params []any) (any, error) {
	chainType, number, err := parseChainParams(params)
	if err != nil {
		return nil, err
	}

	var fv chainblock.FieldsValues

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		res, getErr := chainblock.GetBlock(
			tx,
			m.bucket,
			chainType,
			chainblock.Key(chainType, number),
			nil,
		)
		if getErr != nil {
			return getErr
		}

		fv = res

		return nil
	})
	if err != nil {
		return nil, ErrFailedToGetChainBlockByNumber
	}

	return fv, nil
}

// GetChainBlocks returns up to `count` most recent chain blocks (newest first).
func (m *ChainBlockMethods) GetChainBlocks(ctx context.Context, params []any) (any, error) {
	chainType, count, err := parseChainParams(params)
	if err != nil {
		return nil, err
	}

	var out []chainblock.FieldsValues

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		fv, viewErr := chainblock.GetBlocks(tx, m.bucket, chainType, count)
		if viewErr != nil {
			return viewErr
		}

		out = fv

		return nil
	})
	if err != nil {
		return nil, ErrFailedToGetChainBlocks
	}

	return out, nil
}

func AddChainBlockMethods(server *StandardRPCServer, appchainDB kv.RwDB) {
	methods := NewChainBlockMethods(appchainDB)

	server.AddMethod("getChainBlockByNumber", methods.GetChainBlockByNumber)
	server.AddMethod("getChainBlocks", methods.GetChainBlocks)
}

func parseChainParams(params []any) (apptypes.ChainType, uint64, error) {
	if len(params) != 2 {
		return 0, 0, ErrWrongParamsCount
	}

	chainID, err := parseNumber(params[0])
	if err != nil {
		return 0, 0, ErrInvalidChainType
	}

	value, err := parseNumber(params[1])
	if err != nil {
		return 0, 0, err
	}

	return apptypes.ChainType(chainID), value, nil
}
