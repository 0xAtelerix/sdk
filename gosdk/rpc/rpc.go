package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// NewStandardRPCServer creates a new standard RPC server with optional CORS configuration.
func NewStandardRPCServer(corsConfig *CORSConfig) *StandardRPCServer {
	return &StandardRPCServer{
		methods:     make(map[string]handler),
		corsConfig:  corsConfig,
		middlewares: make([]Middleware, 0),
	}
}

// AddMethod allows adding custom RPC methods
func (s *StandardRPCServer) AddMethod(method string, handler handler) {
	s.methods[method] = handler
}

// AddMiddleware allows adding custom RPC middleware
func (s *StandardRPCServer) AddMiddleware(middleware Middleware) {
	s.middlewares = append(s.middlewares, middleware)
}

// StartHTTPServer starts the HTTP JSON-RPC server
func (s *StandardRPCServer) StartHTTPServer(ctx context.Context, addr string) error {
	s.logger = log.Ctx(ctx)
	http.HandleFunc("/rpc", s.handleRPC)
	http.HandleFunc("/health", s.healthcheck)

	s.logger.Info().Msgf("Starting Standard RPC server on %s\n", addr)
	s.logger.Info().Msgf("Available methods: %d methods registered\n", len(s.methods))
	s.logger.Info().Msgf("Health endpoint available at: %s/health\n", addr)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}

// handleRPC handles incoming JSON-RPC requests
func (s *StandardRPCServer) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.setCORSHeaders(w, "POST, OPTIONS")
		w.WriteHeader(http.StatusOK)

		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	s.setCORSHeaders(w, "POST, OPTIONS")

	// Process request middlewares as early as possible, before reading body
	if s.processRequestMiddlewares(w, r) {
		return
	}

	// Read the entire request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, -32700, "Parse error")

		return
	}
	defer r.Body.Close()

	// Try to decode as batch request (array) first
	var batchReq []JSONRPCRequest
	if err := json.Unmarshal(body, &batchReq); err != nil {
		// If not an array, try single request
		var singleReq JSONRPCRequest
		if err := json.Unmarshal(body, &singleReq); err != nil {
			s.writeError(w, -32700, "Parse error")

			return
		}
		// Handle single request
		response := s.executeRequest(r.Context(), singleReq)
		// Process response middlewares - single response
		if err := s.processResponseMiddlewares(w, r, response); err != nil {
			middlewareErr := &Error{}
			if errors.As(err, &middlewareErr) {
				response = newErrorResponse(middlewareErr, response.ID)
			} else {
				response = newErrorResponse(&Error{Code: -32603, Message: err.Error()}, response.ID)
			}
		}

		// Encode the final response (either original or middleware error)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			s.writeError(w, http.StatusInternalServerError, "Failed to encode response")
		}

		return
	}

	// Handle batch request
	responses := s.handleBatchRequest(r, batchReq)
	// Process response middlewares for each response in batch
	for i := range responses {
		if err := s.processResponseMiddlewares(w, r, responses[i]); err != nil {
			// Handle error per response - replace with error response
			middlewareErr := &Error{}
			if errors.As(err, &middlewareErr) {
				responses[i] = newErrorResponse(middlewareErr, responses[i].ID)
			} else {
				responses[i] = newErrorResponse(&Error{Code: -32603, Message: err.Error()}, responses[i].ID)
			}
		}
	}

	if err := json.NewEncoder(w).Encode(responses); err != nil {
		http.Error(w, "Failed to encode batch response", http.StatusInternalServerError)
	}
}

// handleBatchRequest processes a batch of JSON-RPC requests
func (s *StandardRPCServer) handleBatchRequest(
	r *http.Request,
	batchReq []JSONRPCRequest,
) []JSONRPCResponse {
	if len(batchReq) == 0 {
		return []JSONRPCResponse{
			newErrorResponse(&Error{
				Code:    -32600,
				Message: "Invalid Request - empty batch",
			}, nil),
		}
	}

	responses := make([]JSONRPCResponse, 0, len(batchReq))

	for _, req := range batchReq {
		responses = append(responses, s.executeRequest(r.Context(), req))
	}

	return responses
}

// executeRequest runs a single JSON-RPC request through handler and returns a response
// (middlewares are now handled at HTTP level)
func (s *StandardRPCServer) executeRequest(
	ctx context.Context,
	req JSONRPCRequest,
) JSONRPCResponse {
	if req.JSONRPC != jsonRPCVersion {
		return newErrorResponse(&Error{Code: -32600, Message: "Invalid Request"}, req.ID)
	}

	// Check if method exists
	methodHandler, exists := s.methods[req.Method]
	if !exists {
		return newErrorResponse(&Error{
			Code:    -32601,
			Message: fmt.Errorf("%w: %s", ErrMethodNotFound, req.Method).Error(),
		}, req.ID)
	}

	if methodHandler == nil {
		return newErrorResponse(&Error{
			Code:    -32601,
			Message: fmt.Sprintf("Method %s not implemented", req.Method),
		}, req.ID)
	}

	// Process method handlers
	result, err := methodHandler(ctx, req.Params)
	if err != nil {
		return newErrorResponse(&Error{Code: -32603, Message: err.Error()}, req.ID)
	}

	return JSONRPCResponse{JSONRPC: jsonRPCVersion, Result: result, ID: req.ID}
}

// processRequestMiddlewares runs all request middlewares with HTTP access
func (s *StandardRPCServer) processRequestMiddlewares(
	w http.ResponseWriter,
	r *http.Request,
) bool {
	for _, mw := range s.middlewares {
		if err := mw.ProcessRequest(w, r); err != nil {
			middlewareErr := &Error{}
			if errors.As(err, &middlewareErr) {
				s.writeError(w, middlewareErr.Code, middlewareErr.Message)
			} else {
				s.writeError(w, -32603, err.Error())
			}

			return true // Indicate that the request was blocked
		}
	}

	return false
}

// processResponseMiddlewares runs all response middlewares for a single JSON-RPC response
func (s *StandardRPCServer) processResponseMiddlewares(
	w http.ResponseWriter,
	r *http.Request,
	resp JSONRPCResponse,
) error {
	for _, mw := range s.middlewares {
		if err := mw.ProcessResponse(w, r, resp); err != nil {
			return err
		}
	}

	return nil
}

// healthcheck handles HTTP health check requests
func (s *StandardRPCServer) healthcheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	s.setCORSHeaders(w, "GET")

	healthStatus := map[string]any{
		"status":      healthStatusHealthy,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"rpc_methods": len(s.methods),
	}

	if err := json.NewEncoder(w).Encode(healthStatus); err != nil {
		http.Error(w, "Failed to encode health response", http.StatusInternalServerError)
	}
}

// setCORSHeaders sets CORS headers based on the configuration
func (s *StandardRPCServer) setCORSHeaders(w http.ResponseWriter, defaultMethods string) {
	w.Header().Set("Content-Type", "application/json")

	if s.corsConfig != nil {
		if s.corsConfig.AllowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.corsConfig.AllowOrigin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		if s.corsConfig.AllowMethods != "" {
			w.Header().Set("Access-Control-Allow-Methods", s.corsConfig.AllowMethods)
		} else {
			w.Header().Set("Access-Control-Allow-Methods", defaultMethods)
		}

		if s.corsConfig.AllowHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", s.corsConfig.AllowHeaders)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
	} else {
		// Default CORS headers for backward compatibility
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", defaultMethods)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}
}

// AddStandardMethods adds all standard blockchain methods to the RPC server
func AddStandardMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt, T any](
	server *StandardRPCServer,
	appchainDB kv.RwDB,
	txpool apptypes.TxPoolInterface[appTx, R],
	targetFactory func() T,
) {
	AddTxPoolMethods(server, txpool)
	AddReceiptMethods[R](server, appchainDB)
	AddTransactionMethods(server, txpool, appchainDB)
	AddAppBlockMethods[appTx](server, appchainDB, targetFactory)
}
