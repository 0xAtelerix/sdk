package rpc

import (
	"context"
	"fmt"

	"github.com/goccy/go-json"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// TxPoolMethods provides transaction pool related RPC methods
type TxPoolMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	txpool apptypes.TxPoolInterface[appTx, R]
}

// NewTxPoolMethods creates a new set of transaction pool methods
func NewTxPoolMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	txpool apptypes.TxPoolInterface[appTx, R],
) *TxPoolMethods[appTx, R] {
	return &TxPoolMethods[appTx, R]{
		txpool: txpool,
	}
}

// SendTransaction submits a transaction to the pool
func (m *TxPoolMethods[appTx, R]) SendTransaction(ctx context.Context, params []any) (any, error) {
	if len(params) != 1 {
		return nil, ErrSendTransactionRequires1Param
	}

	// Convert params to transaction (this will need type-specific handling)
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

// GetTransactionByHash retrieves a transaction by hash
func (m *TxPoolMethods[appTx, R]) GetTransactionByHash(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) != 1 {
		return nil, ErrGetTransactionByHashRequires1Param
	}

	hashStr, ok := params[0].(string)
	if !ok {
		return nil, ErrHashParameterMustBeString
	}

	hash, err := parseHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidHashFormat, err)
	}

	// Get transaction from txpool
	tx, err := m.txpool.GetTransaction(ctx, hash[:])
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	return tx, nil
}

// GetPendingTransactions retrieves all pending transactions
func (m *TxPoolMethods[appTx, R]) GetPendingTransactions(
	ctx context.Context,
	_ []any,
) (any, error) {
	transactions, err := m.txpool.GetPendingTransactions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToGetPendingTransactions, err)
	}

	return transactions, nil
}

// AddTxPoolMethods adds all transaction pool methods to the RPC server
func AddTxPoolMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	server *StandardRPCServer,
	txpool apptypes.TxPoolInterface[appTx, R],
) {
	methods := NewTxPoolMethods(txpool)

	server.AddMethod("sendTransaction", methods.SendTransaction)
	server.AddMethod("getTransactionByHash", methods.GetTransactionByHash)
	server.AddMethod("getPendingTransactions", methods.GetPendingTransactions)
}
