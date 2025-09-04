package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	healthStatusHealthy   = "healthy"
	healthStatusUnhealthy = "unhealthy"
	healthStatusDegraded  = "degraded"
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
func (s *StandardRPCServer[appTx, R]) StartHTTPServer(ctx context.Context, addr string) error {
	s.logger = log.Ctx(ctx)
	http.HandleFunc("/rpc", s.handleRPC)
	http.HandleFunc("/health", s.healthcheck)

	s.logger.Info().Msgf("Starting Standard RPC server on %s\n", addr)
	fmt.Printf("Available methods: %d custom methods registered\n", len(s.customMethods))
	fmt.Printf("Health endpoint available at: %s/health\n", addr)

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
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// healthcheck handles HTTP health check requests
func (s *StandardRPCServer[appTx, R]) healthcheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	healthStatus := map[string]any{
		"status":    healthStatusHealthy,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"services":  map[string]string{},
	}

	if services, ok := healthStatus["services"].(map[string]string); ok {
		// Check database connection
		err := s.appchainDB.View(r.Context(), func(_ kv.Tx) error {
			return nil // Just test if we can create a read transaction
		})
		if err != nil {
			services["database"] = healthStatusUnhealthy
			healthStatus["status"] = healthStatusUnhealthy

			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			services["database"] = healthStatusHealthy
		}

		// Check txpool availability
		if s.txpool == nil {
			services["txpool"] = "unavailable"
		} else {
			// Try to get pending transactions to test txpool health
			_, err := s.txpool.GetPendingTransactions(r.Context())
			if err != nil {
				services["txpool"] = healthStatusUnhealthy

				if healthStatus["status"] == healthStatusHealthy {
					healthStatus["status"] = healthStatusDegraded
				}
			} else {
				services["txpool"] = healthStatusHealthy
			}
		}
	}

	// Add method count for monitoring
	healthStatus["rpc_methods"] = len(s.customMethods)

	if err := json.NewEncoder(w).Encode(healthStatus); err != nil {
		http.Error(w, "Failed to encode health response", http.StatusInternalServerError)
	}
}

// AddStandardMethods adds all standard blockchain methods to the RPC server
func AddStandardMethods[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	server *StandardRPCServer[appTx, R],
	appchainDB kv.RwDB,
	txpool apptypes.TxPoolInterface[appTx, R],
) {
	AddTxPoolMethods(server, txpool)
	AddReceiptMethods(server, appchainDB)
}
