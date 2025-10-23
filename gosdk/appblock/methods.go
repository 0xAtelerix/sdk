package appblock

import (
	"context"
	"errors"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var (
	errTargetNil           = errors.New("target cannot be nil")
	errTargetNotPointer    = errors.New("target must be a pointer")
	errTargetNilPointer    = errors.New("target must be a non-nil pointer")
	errBlockPayloadEmpty   = errors.New("block payload is empty")
	errAppchainDatabase    = errors.New("appchain database cannot be nil")
	errAppBlockValueNil    = errors.New("block cannot be nil")
	errBlockNotFound       = errors.New("block not found")
	errTransactionsMissing = errors.New("block does not store transactions in payload")
)

// GetAppBlockByNumber decodes a CBOR-encoded payload into target and returns its fields and values.
func GetAppBlockByNumber[T any](
	blockNumber uint64,
	payload []byte,
	target T,
) (FieldsValues, error) {
	if err := unmarshallIntoTarget(payload, target); err != nil {
		return FieldsValues{}, err
	}

	cb := NewAppBlock(blockNumber, target)

	return cb.ToFieldsAndValues(), nil
}

// GetTransactionsFromBlock hydrates the provided target template from the block
// payload stored for blockNumber and extracts transactions via the target's Txs
// field when present. It falls back to the block transactions bucket and
// distinguishes between missing data and an explicitly nil Txs field.
func GetTransactionsFromBlock[appTx apptypes.AppTransaction[R], R apptypes.Receipt, T any](
	ctx context.Context,
	db kv.RwDB,
	blockNumber uint64,
	rootTemplate T,
) ([]appTx, bool, error) {
	target, err := CloneTarget(rootTemplate)
	if err != nil {
		return nil, false, err
	}

	if err := decodeBlockIntoTarget(ctx, db, blockNumber, target); err != nil {
		return nil, false, err
	}

	if txs, ok, wasNil := extractTransactions[appTx](target); ok {
		if wasNil {
			return make([]appTx, 0), true, nil
		}

		return txs, true, nil
	}

	return nil, false, errTransactionsMissing
}
