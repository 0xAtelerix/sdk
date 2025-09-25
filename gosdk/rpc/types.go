package rpc // JSONRPCRequest represents a standard JSON-RPC 2.0 request

import (
	"context"
	"net/http"

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

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}

// CORSConfig holds CORS configuration for the RPC server
type CORSConfig struct {
	AllowOrigin  string
	AllowMethods string
	AllowHeaders string
}

type handler func(context.Context, []any) (any, error)

// StandardRPCServer provides standard blockchain RPC methods
type StandardRPCServer struct {
	logger      *zerolog.Logger
	methods     map[string]handler
	corsConfig  *CORSConfig
	middlewares []Middleware
}

type Middleware interface {
	// ProcessRequest processes an incoming HTTP request before JSON-RPC parsing
	// Called immediately after handleRPC starts, before reading request body
	// Can access all HTTP details (headers, method, etc.) and write early responses
	ProcessRequest(w http.ResponseWriter, r *http.Request) error

	// ProcessResponse processes an outgoing JSON-RPC response after handler execution
	// response parameter can be JSONRPCResponse (single) or []JSONRPCResponse (batch)
	// Can modify response, add headers, or write custom responses
	ProcessResponse(w http.ResponseWriter, r *http.Request, response JSONRPCResponse) error
}
