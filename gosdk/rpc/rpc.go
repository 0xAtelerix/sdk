package rpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	healthStatusHealthy = "healthy"
	jsonRPCVersion      = "2.0"
)

// NewStandardRPCServer creates a new standard RPC server with optional CORS configuration.
func NewStandardRPCServer(corsConfig *CORSConfig) *StandardRPCServer {
	return &StandardRPCServer{
		customMethods: make(map[string]func(context.Context, []any) (any, error)),
		corsConfig:    corsConfig,
	}
}

// AddCustomMethod allows adding custom RPC methods
func (s *StandardRPCServer) AddCustomMethod(
	method string,
	handler func(context.Context, []any) (any, error),
) {
	s.customMethods[method] = handler
}

// StartHTTPServer starts the HTTP JSON-RPC server
func (s *StandardRPCServer) StartHTTPServer(ctx context.Context, addr string) error {
	s.logger = log.Ctx(ctx)
	http.HandleFunc("/rpc", s.handleRPC)
	http.HandleFunc("/health", s.healthcheck)

	s.logger.Info().Msgf("Starting Standard RPC server on %s\n", addr)
	s.logger.Info().Msgf("Available methods: %d custom methods registered\n", len(s.customMethods))
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
		w.WriteHeader(http.StatusOK)

		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	s.setCORSHeaders(w, "POST, OPTIONS")

	// Read the entire request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, -32700, "Parse error", nil)

		return
	}
	defer r.Body.Close()

	// Try to decode as batch request (array) first
	var batchReq []JSONRPCRequest
	if err := json.Unmarshal(body, &batchReq); err != nil {
		// If not an array, try single request
		var singleReq JSONRPCRequest
		if err := json.Unmarshal(body, &singleReq); err != nil {
			s.writeError(w, -32700, "Parse error", nil)

			return
		}
		// Handle single request
		s.handleSingleRequest(w, r, singleReq)

		return
	}

	// Handle batch request
	s.handleBatchRequest(w, r, batchReq)
}

// handleSingleRequest processes a single JSON-RPC request
func (s *StandardRPCServer) handleSingleRequest(
	w http.ResponseWriter,
	r *http.Request,
	req JSONRPCRequest,
) {
	if req.JSONRPC != jsonRPCVersion {
		s.writeError(w, -32600, "Invalid Request", req.ID)

		return
	}

	handler, exists := s.customMethods[req.Method]
	if !exists {
		s.writeError(w, -32601, fmt.Errorf("%w: %s", ErrMethodNotFound, req.Method).Error(), req.ID)

		return
	}

	result, err := handler(r.Context(), req.Params)
	if err != nil {
		s.writeError(w, -32603, err.Error(), req.ID)

		return
	}

	response := JSONRPCResponse{
		JSONRPC: jsonRPCVersion,
		Result:  result,
		ID:      req.ID,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleBatchRequest processes a batch of JSON-RPC requests
func (s *StandardRPCServer) handleBatchRequest(
	w http.ResponseWriter,
	r *http.Request,
	batchReq []JSONRPCRequest,
) {
	if len(batchReq) == 0 {
		s.writeError(w, -32600, "Invalid Request - empty batch", nil)

		return
	}

	responses := make([]JSONRPCResponse, 0, len(batchReq))

	for _, req := range batchReq {
		if req.JSONRPC != jsonRPCVersion {
			responses = append(responses, JSONRPCResponse{
				JSONRPC: jsonRPCVersion,
				Error:   &Error{Code: -32600, Message: "Invalid Request"},
				ID:      req.ID,
			})

			continue
		}

		handler, exists := s.customMethods[req.Method]
		if !exists {
			responses = append(responses, JSONRPCResponse{
				JSONRPC: jsonRPCVersion,
				Error: &Error{
					Code:    -32601,
					Message: fmt.Errorf("%w: %s", ErrMethodNotFound, req.Method).Error(),
				},
				ID: req.ID,
			})

			continue
		}

		result, err := handler(r.Context(), req.Params)
		if err != nil {
			responses = append(responses, JSONRPCResponse{
				JSONRPC: jsonRPCVersion,
				Error:   &Error{Code: -32603, Message: err.Error()},
				ID:      req.ID,
			})

			continue
		}

		responses = append(responses, JSONRPCResponse{
			JSONRPC: jsonRPCVersion,
			Result:  result,
			ID:      req.ID,
		})
	}

	if err := json.NewEncoder(w).Encode(responses); err != nil {
		http.Error(w, "Failed to encode batch response", http.StatusInternalServerError)
	}
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
		"rpc_methods": len(s.customMethods),
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
		}

		if s.corsConfig.AllowMethods != "" {
			w.Header().Set("Access-Control-Allow-Methods", s.corsConfig.AllowMethods)
		} else {
			w.Header().Set("Access-Control-Allow-Methods", defaultMethods)
		}

		if s.corsConfig.AllowHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", s.corsConfig.AllowHeaders)
		}
	} else {
		// Default CORS headers for backward compatibility
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", defaultMethods)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}
}

// AddStandardMethods adds all standard blockchain methods to the RPC server
func AddStandardMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	server *StandardRPCServer,
	appchainDB kv.RwDB,
	txpool apptypes.TxPoolInterface[appTx, R],
) {
	AddTxPoolMethods(server, txpool)
	AddReceiptMethods[R](server, appchainDB)
	AddTransactionMethods(server, txpool, appchainDB)
}
