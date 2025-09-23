package rpc // JSONRPCRequest represents a standard JSON-RPC 2.0 request

import (
	"context"

	"github.com/rs/zerolog"
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

// CORSConfig holds CORS configuration for the RPC server
type CORSConfig struct {
	AllowOrigin  string
	AllowMethods string
	AllowHeaders string
}

// StandardRPCServer provides standard blockchain RPC methods
type StandardRPCServer struct {
	logger        *zerolog.Logger
	customMethods map[string]func(context.Context, []any) (any, error)
	corsConfig    *CORSConfig
}
