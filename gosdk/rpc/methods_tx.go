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

// TransactionMethods provides comprehensive transaction-related RPC methods
// This handles both finalized transactions (via receipts) and pending transactions (via txpool)
type TransactionMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	txpool     apptypes.TxPoolInterface[appTx, R]
	appchainDB kv.RwDB
}

// NewTransactionMethods creates a new set of comprehensive transaction methods
func NewTransactionMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	txpool apptypes.TxPoolInterface[appTx, R],
	appchainDB kv.RwDB,
) *TransactionMethods[appTx, R] {
	return &TransactionMethods[appTx, R]{
		txpool:     txpool,
		appchainDB: appchainDB,
	}
}

// GetTransactionStatus retrieves comprehensive transaction status
// 1. First checks receipts - if found, returns "confirmed" or "failed" based on receipt status
// 2. If no receipt found, checks txpool for pending status
func (m *TransactionMethods[appTx, R]) GetTransactionStatus(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetTransactionStatusRequires1Param
	}

	hashStr, ok := params[0].(string)
	if !ok {
		return nil, ErrHashParameterMustBeString
	}

	hash, err := parseHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidHashFormat, err)
	}

	// Step 1: Check if receipt exists (transaction is finalized)
	var receiptResp R

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		value, getErr := tx.GetOne(receipt.ReceiptBucket, hash[:])
		if getErr != nil {
			return getErr
		}

		if len(value) == 0 {
			return receipt.ErrNoReceipts // No receipt found
		}

		return cbor.Unmarshal(value, &receiptResp)
	})
	if err == nil {
		// Receipt found - return status based on receipt
		switch receiptResp.Status() {
		case apptypes.ReceiptConfirmed:
			return apptypes.Processed.String(), nil
		case apptypes.ReceiptFailed:
			return apptypes.Failed.String(), nil
		case apptypes.ReceiptUnknown:
			return apptypes.Unknown.String(), nil
		default:
			return apptypes.Unknown.String(), nil
		}
	}

	// Step 2: No receipt found, check txpool for pending status
	if errors.Is(err, receipt.ErrNoReceipts) {
		// Try to get transaction status from txpool
		status, txpoolErr := m.txpool.GetTransactionStatus(ctx, hash[:])
		if txpoolErr != nil {
			return nil, fmt.Errorf("failed to check txpool status: %w", txpoolErr)
		}
		// Convert TxStatus to string (pending, batched, etc.)
		return status.String(), nil
	}

	// Database error occurred
	return nil, fmt.Errorf("failed to check transaction status: %w", err)
}

// AddTransactionMethods adds all comprehensive transaction methods to the RPC server
func AddTransactionMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	server *StandardRPCServer,
	txpool apptypes.TxPoolInterface[appTx, R],
	appchainDB kv.RwDB,
) {
	methods := NewTransactionMethods(txpool, appchainDB)

	server.AddCustomMethod("getTransactionStatus", methods.GetTransactionStatus)
}
