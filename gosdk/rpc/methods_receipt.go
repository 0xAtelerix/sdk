package rpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
)

// ReceiptMethods provides receipt related RPC methods
type ReceiptMethods[R apptypes.Receipt] struct {
	appchainDB kv.RwDB
}

// NewReceiptMethods creates a new set of receipt methods
func NewReceiptMethods[R apptypes.Receipt](appchainDB kv.RwDB) *ReceiptMethods[R] {
	return &ReceiptMethods[R]{
		appchainDB: appchainDB,
	}
}

// GetTransactionReceipt retrieves a transaction receipt by hash
func (m *ReceiptMethods[R]) GetTransactionReceipt(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetTransactionReceiptRequires1Param
	}

	hashStr, ok := params[0].(string)
	if !ok {
		return nil, ErrHashParameterMustBeString
	}

	hash, err := parseHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidHashFormat, err)
	}

	// Get receipt using the database
	var receiptResp R

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		value, getErr := tx.GetOne(receipt.ReceiptBucket, hash[:])
		if getErr != nil {
			return getErr
		}

		if len(value) == 0 {
			return ErrReceiptNotFound
		}

		return cbor.Unmarshal(value, &receiptResp)
	})
	if err != nil {
		if errors.Is(err, ErrReceiptNotFound) {
			return nil, err
		}

		return nil, fmt.Errorf("%w: %w", ErrFailedToGetReceipt, err)
	}

	return receiptResp, nil
}

// AddReceiptMethods adds all receipt methods to the RPC server
func AddReceiptMethods[R apptypes.Receipt](
	server *StandardRPCServer,
	appchainDB kv.RwDB,
) {
	methods := NewReceiptMethods[R](appchainDB)

	server.AddCustomMethod("getTransactionReceipt", methods.GetTransactionReceipt)
}
