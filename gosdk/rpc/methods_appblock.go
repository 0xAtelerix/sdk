package rpc

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/appblock"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type AppBlockMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt, T any] struct {
	appchainDB kv.RwDB
	target     T
}

func NewAppBlockMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt, T any](
	appchainDB kv.RwDB,
	target T,
) *AppBlockMethods[appTx, R, T] {
	return &AppBlockMethods[appTx, R, T]{
		appchainDB: appchainDB,
		target:     target,
	}
}

// GetAppBlock returns the decoded application block fields for the requested
// block number, optionally enriching the payload with stored transactions when
// available.
func (m *AppBlockMethods[appTx, R, T]) GetAppBlock(
	ctx context.Context,
	params []any,
) (any, error) {
	if m == nil {
		return nil, errAppBlockMethodsNotInitialized
	}

	if len(params) != 1 {
		return nil, ErrWrongParamsCount
	}

	blockNumber, err := parseBlockNumber(params[0])
	if err != nil {
		return nil, err
	}

	target, err := appblock.CloneTarget(m.target)
	if err != nil {
		return nil, err
	}

	payload, err := appblock.LoadBlockPayload(ctx, m.appchainDB, blockNumber)
	if err != nil {
		return nil, err
	}

	fv, err := appblock.GetAppBlockByNumber(blockNumber, payload, target)
	if err != nil {
		return nil, err
	}

	return fv, nil
}

// GetTransactionsByBlockNumber returns all stored application transactions for
// the given block number.
func (m *AppBlockMethods[appTx, R, T]) GetTransactionsByBlockNumber(
	ctx context.Context,
	params []any,
) (any, error) {
	if m == nil {
		return nil, errAppBlockMethodsNotInitialized
	}

	if len(params) != 1 {
		return nil, ErrGetTransactionsByBlockNumberRequires1Param
	}

	blockNumber, err := parseBlockNumber(params[0])
	if err != nil {
		return nil, err
	}

	txs, ok, err := appblock.GetTransactionsFromBlock[appTx](
		ctx,
		m.appchainDB,
		blockNumber,
		m.target,
	)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrBlockNotFound
	}

	return txs, nil
}

func AddAppBlockMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt, T any](
	server *StandardRPCServer,
	appchainDB kv.RwDB,
	target T,
) {
	methods := NewAppBlockMethods[appTx, R](appchainDB, target)

	server.AddMethod("getAppBlock", methods.GetAppBlock)
	server.AddMethod("getTransactionsByBlockNumber", methods.GetTransactionsByBlockNumber)
}
