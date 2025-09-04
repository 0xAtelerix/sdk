package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
)

// NewStandardRPCServer creates a new standard RPC server
func NewStandardRPCServer[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	appchainDB kv.RwDB,
	txpool apptypes.TxPoolInterface[appTx, R],
) *StandardRPCServer[appTx, R] {
	return &StandardRPCServer[appTx, R]{
		appchainDB:    appchainDB,
		txpool:        txpool,
		customMethods: make(map[string]func(context.Context, []any) (any, error)),
	}
}

// AddCustomMethod allows adding custom RPC methods
func (s *StandardRPCServer[appTx, R]) AddCustomMethod(
	method string,
	handler func(context.Context, []any) (any, error),
) {
	s.customMethods[method] = handler
}

// StartHTTPServer starts the HTTP JSON-RPC server
func (s *StandardRPCServer[appTx, R]) StartHTTPServer(port string) error {
	http.HandleFunc("/rpc", s.handleRPC)
	port = strings.TrimPrefix(port, ":")
	if port == "" {
		port = "8545" // Default port
	}

	addr := fmt.Sprintf(":%s", port)
	fmt.Printf("Starting Standard RPC server on %s\n", addr)
	fmt.Println("Available methods:")
	fmt.Println("  - getTransactionReceipt")
	fmt.Println("  - getTransactionByHash")
	fmt.Println("  - sendTransaction")
	fmt.Println("  - getTransactionStatus")
	fmt.Println("  - getPendingTransactions")
	fmt.Println("  - getChainInfo")

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}

// handleRPC handles incoming JSON-RPC requests
func (s *StandardRPCServer[appTx, R]) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, -32700, "Parse error", req.ID)

		return
	}

	if req.JSONRPC != "2.0" {
		s.writeError(w, -32600, "Invalid Request", req.ID)

		return
	}

	result, err := s.handleMethod(r.Context(), req.Method, req.Params)
	if err != nil {
		s.writeError(w, -32603, err.Error(), req.ID)

		return
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleMethod routes method calls to appropriate handlers
func (s *StandardRPCServer[appTx, R]) handleMethod(
	ctx context.Context,
	method string,
	params []any,
) (any, error) {
	switch method {
	case "getTransactionReceipt":
		return s.getTransactionReceipt(ctx, params)
	case "getTransactionByHash":
		return s.getTransactionByHash(ctx, params)
	case "sendTransaction":
		return s.sendTransaction(ctx, params)
	case "getTransactionStatus":
		return s.getTransactionStatus(ctx, params)
	case "getPendingTransactions":
		return s.getPendingTransactions(ctx, params)
	default:
		// Check for custom methods
		if handler, exists := s.customMethods[method]; exists {
			return handler(ctx, params)
		}

		return nil, fmt.Errorf("%w: %s", ErrMethodNotFound, method)
	}
}

// getTransactionReceipt retrieves a transaction receipt by hash
func (s *StandardRPCServer[appTx, R]) getTransactionReceipt(
	ctx context.Context,
	params []any,
) (any, error) {
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

	err = s.appchainDB.View(ctx, func(tx kv.Tx) error {
		value, getErr := tx.GetOne(receipt.ReceiptBucket, hash[:])
		if getErr != nil {
			return getErr
		}

		if len(value) == 0 {
			return ErrReceiptNotFound
		}

		return receiptResp.Unmarshal(value)
	})
	if err != nil {
		if errors.Is(err, ErrReceiptNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %w", ErrFailedToGetReceipt, err)
	}

	return receiptResp, nil
}

// getTransactionByHash retrieves a transaction by hash
func (s *StandardRPCServer[appTx, R]) getTransactionByHash(
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
	tx, err := s.txpool.GetTransaction(ctx, hash[:])
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	return tx, nil
}

// sendTransaction submits a transaction to the pool
func (s *StandardRPCServer[appTx, R]) sendTransaction(
	ctx context.Context,
	params []any,
) (any, error) {
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
	if err := s.txpool.AddTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToAddTransaction, err)
	}

	// Return transaction hash
	hash := tx.Hash()

	return fmt.Sprintf("0x%x", hash[:]), nil
}

// getTransactionStatus retrieves transaction status
func (s *StandardRPCServer[appTx, R]) getTransactionStatus(
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

	// Use txpool's GetTransactionStatus method
	status, err := s.txpool.GetTransactionStatus(ctx, hash[:])
	if err != nil {
		return "not_found", ErrTransactionNotFound
	}

	// Convert TxStatus to string
	return status.String(), nil
}

// getPendingTransactions retrieves all pending transactions
func (s *StandardRPCServer[appTx, R]) getPendingTransactions(
	ctx context.Context,
	_ []any,
) (any, error) {
	transactions, err := s.txpool.GetPendingTransactions(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToGetPendingTransactions, err)
	}

	return transactions, nil
}
