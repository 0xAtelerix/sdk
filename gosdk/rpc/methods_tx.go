package rpc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

// TransactionMethods provides comprehensive transaction-related RPC methods.
// This handles:
// - Submitting transactions to the pool
// - Querying pending transactions
// - Querying finalized transactions
// - Transaction status queries across pending and finalized states
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

// SendTransaction submits a transaction to the pool.
// Params: [transaction]
// Returns: transaction hash as hex string
func (m *TransactionMethods[appTx, R]) SendTransaction(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrWrongParamsCount
	}

	// Convert params to transaction
	txData, err := json.Marshal(params[0])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTransactionData, err)
	}

	var tx appTx
	if err := json.Unmarshal(txData, &tx); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToParseTransaction, err)
	}

	// Add to txpool
	if err := m.txpool.AddTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToAddTransaction, err)
	}

	// Return transaction hash
	hash := tx.Hash()

	return fmt.Sprintf("0x%x", hash[:]), nil
}

// GetPendingTransactions retrieves all pending transactions from the pool.
// Params: []
// Returns: []Transaction
func (m *TransactionMethods[appTx, R]) GetPendingTransactions(
	ctx context.Context,
	_ []any,
) (any, error) {
	transactions, err := m.txpool.GetPendingTransactions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToGetPendingTransactions, err)
	}

	return transactions, nil
}

// GetTransaction retrieves a transaction by hash from either finalized blocks or txpool.
// Searches in this order: 1) Finalized blocks (via TxLookupBucket index), 2) Txpool (pending)
// Params: [txHash]
// Returns: Transaction object
func (m *TransactionMethods[appTx, R]) GetTransaction(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrWrongParamsCount
	}

	hashStr, ok := params[0].(string)
	if !ok {
		return nil, ErrHashParameterMustBeString
	}

	txHash, err := parseHash(hashStr)
	if err != nil {
		return nil, err
	}

	// Step 1: Check finalized blocks first (most common case)
	var transaction appTx

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		// Lookup index: txHash -> (blockNumber, txIndex)
		lookupData, getErr := tx.GetOne(gosdk.TxLookupBucket, txHash[:])
		if getErr != nil {
			return ErrTransactionNotFound
		}

		if len(lookupData) != 12 {
			return ErrTransactionNotFound // Not in blocks or corrupted entry
		}

		// Decode lookup entry: blockNumber (8 bytes) + txIndex (4 bytes)
		blockNumber := binary.BigEndian.Uint64(lookupData[0:8])
		txIndex := binary.BigEndian.Uint32(lookupData[8:12])

		// Get block transactions
		txsBytes, getErr := tx.GetOne(gosdk.BlockTransactionsBucket, utility.Uint64ToBytes(blockNumber))
		if getErr != nil {
			return fmt.Errorf("get block transactions: %w", getErr)
		}

		if len(txsBytes) == 0 {
			return fmt.Errorf("block %d has no transactions", blockNumber)
		}

		// Unmarshal transactions array
		var txs []appTx
		if unmarshalErr := cbor.Unmarshal(txsBytes, &txs); unmarshalErr != nil {
			return fmt.Errorf("decode transactions: %w", unmarshalErr)
		}

		// Bounds check
		if txIndex >= uint32(len(txs)) {
			return fmt.Errorf("tx index %d out of bounds (block has %d txs)", txIndex, len(txs))
		}

		// Get the specific transaction
		transaction = txs[txIndex]

		return nil
	})
	if err == nil {
		// Found in finalized blocks
		return transaction, nil
	}

	// Step 2: Not in blocks, check txpool for pending transaction
	if errors.Is(err, ErrTransactionNotFound) {
		tx, txpoolErr := m.txpool.GetTransaction(ctx, txHash[:])
		if txpoolErr == nil {
			return tx, nil
		}
		// Not in txpool either
		return nil, ErrTransactionNotFound
	}

	// Other database error
	return nil, fmt.Errorf("failed to get transaction: %w", err)
}

// GetTransactionsByBlockNumber retrieves all transactions in a block.
// Params: [blockNumber]
// Returns: []Transaction
func (m *TransactionMethods[appTx, R]) GetTransactionsByBlockNumber(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrWrongParamsCount
	}

	blockNumber, err := parseBlockNumber(params[0])
	if err != nil {
		return nil, err
	}

	var txs []appTx

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		payload, getErr := tx.GetOne(
			gosdk.BlockTransactionsBucket,
			utility.Uint64ToBytes(blockNumber),
		)
		if getErr != nil {
			return getErr
		}

		if len(payload) == 0 {
			txs = []appTx{}

			return nil
		}

		if unmarshalErr := cbor.Unmarshal(payload, &txs); unmarshalErr != nil {
			return fmt.Errorf("decode transactions: %w", unmarshalErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return txs, nil
}

// GetExternalTransactions retrieves external transactions generated by a block.
// These are cross-chain transactions emitted to other blockchains.
// Params: [blockNumber]
// Returns: []ExternalTransaction
func (m *TransactionMethods[appTx, R]) GetExternalTransactions(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrWrongParamsCount
	}

	blockNumber, err := parseBlockNumber(params[0])
	if err != nil {
		return nil, err
	}

	var externalTxs []apptypes.ExternalTransaction

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		// Use the shared ReadExternalTransactions helper
		var readErr error

		externalTxs, readErr = gosdk.ReadExternalTransactions(tx, blockNumber)

		return readErr
	})
	if err != nil {
		return nil, err
	}

	return externalTxs, nil
}

// GetTransactionStatus retrieves comprehensive transaction status
// 1. First checks receipts - if found, returns "confirmed" or "failed" based on receipt status
// 2. If no receipt found, checks txpool for pending status
func (m *TransactionMethods[appTx, R]) GetTransactionStatus(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrWrongParamsCount
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

// AddTransactionMethods adds all transaction-related methods to the RPC server.
// This includes both txpool operations (submit, query pending) and
// comprehensive transaction queries (finalized + pending).
func AddTransactionMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	server *StandardRPCServer,
	txpool apptypes.TxPoolInterface[appTx, R],
	appchainDB kv.RwDB,
) {
	methods := NewTransactionMethods(txpool, appchainDB)

	// Transaction submission
	server.AddMethod("sendTransaction", methods.SendTransaction)

	// Pending transactions
	server.AddMethod("getPendingTransactions", methods.GetPendingTransactions)

	// Transaction queries (checks both finalized + pending)
	server.AddMethod("getTransaction", methods.GetTransaction)
	server.AddMethod("getTransactionStatus", methods.GetTransactionStatus)

	// Finalized transaction queries
	server.AddMethod("getTransactionsByBlockNumber", methods.GetTransactionsByBlockNumber)
	server.AddMethod("getExternalTransactions", methods.GetExternalTransactions)
}
