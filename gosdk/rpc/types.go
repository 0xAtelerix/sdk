package rpc // JSONRPCRequest represents a standard JSON-RPC 2.0 request

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      any    `json:"id"`
}

// JSONRPCResponse represents a standard JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
	ID      any    `json:"id"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// StandardRPCServer provides standard blockchain RPC methods
type StandardRPCServer[appTx apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	appchainDB    kv.RwDB
	txpool        apptypes.TxPoolInterface[appTx, R]
	logger        *zerolog.Logger
	customMethods map[string]func(context.Context, []any) (any, error)
}
