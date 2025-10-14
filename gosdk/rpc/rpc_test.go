package rpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	//"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
	"github.com/0xAtelerix/sdk/gosdk/txpool"

	"github.com/0xAtelerix/sdk/gosdk/block"
)

const (
	testSuccessMessage        = "success"
	testShouldNotReachMessage = "should not reach here"
)

var (
	errInvalidParameterType = errors.New("invalid parameter type")
	errMiddlewareFailed     = errors.New("middleware failed")
)

// TestTransaction - test transaction implementation
// TODO rremove my implementation and use this one
type TestTransaction[R TestReceipt] struct {
	From  string `json:"from"  cbor:"1,keyasint"`
	To    string `json:"to"    cbor:"2,keyasint"`
	Value int    `json:"value" cbor:"3,keyasint"`
}

func (t TestTransaction[R]) Hash() [32]byte {
	s := t.From + t.To + strconv.Itoa(t.Value)

	return sha256.Sum256([]byte(s))
}

func (TestTransaction[R]) Process(
	_ kv.RwTx,
) (rec R, txs []apptypes.ExternalTransaction, err error) {
	return
}

// TestReceipt - test receipt implementation
type TestReceipt struct {
	ReceiptStatus   apptypes.TxReceiptStatus `json:"status" cbor:"1,keyasint"`
	TransactionHash [32]byte                 `json:"txHash" cbor:"2,keyasint"`
}

func (r TestReceipt) TxHash() [32]byte {
	return r.TransactionHash
}

func (r TestReceipt) Status() apptypes.TxReceiptStatus {
	return r.ReceiptStatus
}

func (r TestReceipt) Error() string {
	if r.ReceiptStatus == apptypes.ReceiptFailed {
		return "transaction failed"
	}

	return ""
}

// setupTestEnvironment creates a test environment with databases and server
func setupTestEnvironment(
	t *testing.T,
) (server *StandardRPCServer, appchainDB kv.RwDB, cleanup func()) {
	t.Helper()

	// Create temporary directories for databases
	localDBPath := t.TempDir()
	appchainDBPath := t.TempDir()

	// Create local database for txpool
	localDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(localDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	require.NoError(t, err)

	// Create appchain database
	appchainDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(appchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				receipt.ReceiptBucket:         {},
				block.BlockNumberBucket:       {},
				block.BlockHashBucket:         {},
				block.BlockTransactionsBucket: {},
			}
		}).
		Open()
	require.NoError(t, err)

	// Create txpool
	txPool := txpool.NewTxPool[TestTransaction[TestReceipt]](localDB)

	// Create RPC server
	server = NewStandardRPCServer(nil)

	// Add standard methods to maintain compatibility with existing tests
	AddStandardMethods(server, appchainDB, txPool)
	AddBlockMethods(server, appchainDB)

	cleanup = func() {
		localDB.Close()
		appchainDB.Close()
	}

	return server, appchainDB, cleanup
}

// makeJSONRPCRequest creates a JSON-RPC request and returns the response
func makeJSONRPCRequest(
	t *testing.T,
	server *StandardRPCServer,
	method string,
	params []any,
) *httptest.ResponseRecorder {
	t.Helper()

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqBody, err := json.Marshal(request)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	return rr
}

// ============= BASIC RPC METHODS =============

func TestStandardRPCServer_sendTransaction(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a test transaction
	tx := &TestTransaction[TestReceipt]{
		From:  "0x1234",
		To:    "0x5678",
		Value: 100,
	}

	rr := makeJSONRPCRequest(t, server, "sendTransaction", []any{tx})

	assert.Equal(t, http.StatusOK, rr.Code)

	var response JSONRPCResponse

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Nil(t, response.Error)
	assert.NotNil(t, response.Result)

	// Check that the result is a hex string (transaction hash)
	hashStr, ok := response.Result.(string)
	require.True(t, ok)
	assert.NotEmpty(t, hashStr)
	assert.Equal(t, "0x", hashStr[:2])
}

func TestStandardRPCServer_getTransactionByHash(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// First, send a transaction
	tx := &TestTransaction[TestReceipt]{
		From:  "0x1234",
		To:    "0x5678",
		Value: 100,
	}

	// Send the transaction
	sendRR := makeJSONRPCRequest(t, server, "sendTransaction", []any{tx})
	require.Equal(t, http.StatusOK, sendRR.Code)

	var sendResponse JSONRPCResponse

	err := json.Unmarshal(sendRR.Body.Bytes(), &sendResponse)
	require.NoError(t, err)

	hashStr, ok := sendResponse.Result.(string)
	require.True(t, ok, "sendResponse should be string", sendResponse.Result, sendRR.Body)

	// Now get the transaction by hash
	getRR := makeJSONRPCRequest(t, server, "getTransactionByHash", []any{hashStr})

	assert.Equal(t, http.StatusOK, getRR.Code)

	var getResponse JSONRPCResponse

	err = json.Unmarshal(getRR.Body.Bytes(), &getResponse)
	require.NoError(t, err)

	assert.Equal(t, "2.0", getResponse.JSONRPC)
	assert.Nil(t, getResponse.Error)
	assert.NotNil(t, getResponse.Result)
}

func TestStandardRPCServer_getTransactionStatus(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// First, send a transaction
	tx := &TestTransaction[TestReceipt]{
		From:  "0x1234",
		To:    "0x5678",
		Value: 100,
	}

	// Send the transaction
	sendRR := makeJSONRPCRequest(t, server, "sendTransaction", []any{tx})
	require.Equal(t, http.StatusOK, sendRR.Code)

	var sendResponse JSONRPCResponse

	err := json.Unmarshal(sendRR.Body.Bytes(), &sendResponse)
	require.NoError(t, err)

	hashStr, ok := sendResponse.Result.(string)
	require.True(t, ok)

	// Now get the transaction status
	statusRR := makeJSONRPCRequest(t, server, "getTransactionStatus", []any{hashStr})

	assert.Equal(t, http.StatusOK, statusRR.Code)

	var statusResponse JSONRPCResponse

	err = json.Unmarshal(statusRR.Body.Bytes(), &statusResponse)
	require.NoError(t, err)

	assert.Equal(t, "2.0", statusResponse.JSONRPC)
	assert.Nil(t, statusResponse.Error)

	status, ok := statusResponse.Result.(string)
	require.True(t, ok)
	assert.Contains(
		t,
		[]string{"Pending", "Batched", "ReadyToProcess", "Processed", "Unknown"},
		status,
	)
}

func TestStandardRPCServer_getPendingTransactions(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// First, send some transactions
	txs := []TestTransaction[TestReceipt]{
		{From: "0x1111", To: "0x2222", Value: 100},
		{From: "0x3333", To: "0x4444", Value: 200},
	}

	for _, tx := range txs {
		sendRR := makeJSONRPCRequest(t, server, "sendTransaction", []any{tx})
		require.Equal(t, http.StatusOK, sendRR.Code)
	}

	// Get pending transactions
	pendingRR := makeJSONRPCRequest(t, server, "getPendingTransactions", []any{})

	assert.Equal(t, http.StatusOK, pendingRR.Code)

	var pendingResponse JSONRPCResponse

	err := json.Unmarshal(pendingRR.Body.Bytes(), &pendingResponse)
	require.NoError(t, err)

	assert.Equal(t, "2.0", pendingResponse.JSONRPC)
	assert.Nil(t, pendingResponse.Error)
	assert.NotNil(t, pendingResponse.Result)

	// The result should be an array
	pendingTxs, ok := pendingResponse.Result.([]any)
	require.True(t, ok)
	assert.Len(t, pendingTxs, 2)
}

func TestStandardRPCServer_getTransactionReceipt(t *testing.T) {
	server, appchainDB, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a test receipt
	testReceipt := TestReceipt{ReceiptStatus: apptypes.ReceiptConfirmed}
	receiptData, err := cbor.Marshal(testReceipt)
	require.NoError(t, err)

	// Create a test hash
	hash := sha256.Sum256([]byte("test-receipt"))

	// Store receipt in database
	err = appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.Put(receipt.ReceiptBucket, hash[:], receiptData)
	})
	require.NoError(t, err)

	// Get the receipt
	hashStr := "0x" + hex.EncodeToString(hash[:])
	receiptRR := makeJSONRPCRequest(t, server, "getTransactionReceipt", []any{hashStr})

	assert.Equal(t, http.StatusOK, receiptRR.Code)

	var receiptResponse JSONRPCResponse

	err = json.Unmarshal(receiptRR.Body.Bytes(), &receiptResponse)
	require.NoError(t, err)

	assert.Equal(t, "2.0", receiptResponse.JSONRPC)
	assert.Nil(t, receiptResponse.Error)
	assert.NotNil(t, receiptResponse.Result)
}

// ============= CUSTOM METHODS & ERROR HANDLING =============

func TestStandardRPCServer_customMethod(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Add a custom method
	server.AddMethod("customTest", func(_ context.Context, params []any) (any, error) {
		return map[string]any{
			"message": "custom method works",
			"params":  params,
		}, nil
	})

	// Call the custom method
	customRR := makeJSONRPCRequest(t, server, "customTest", []any{"param1", 42})

	assert.Equal(t, http.StatusOK, customRR.Code)

	var customResponse JSONRPCResponse

	err := json.Unmarshal(customRR.Body.Bytes(), &customResponse)
	require.NoError(t, err)

	assert.Equal(t, "2.0", customResponse.JSONRPC)
	assert.Nil(t, customResponse.Error)

	result, ok := customResponse.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "custom method works", result["message"])
}

func TestStandardRPCServer_invalidMethod(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	rr := makeJSONRPCRequest(t, server, "nonExistentMethod", []any{})

	assert.Equal(t, http.StatusOK, rr.Code)

	var response JSONRPCResponse

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "2.0", response.JSONRPC)
	assert.NotNil(t, response.Error)
	assert.Contains(t, response.Error.Message, "method not found")
}

func TestStandardRPCServer_invalidJSONRPC(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response JSONRPCResponse

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response.Error)
	assert.Equal(t, -32700, response.Error.Code) // Parse error
}

func TestStandardRPCServer_wrongHTTPMethod(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/rpc", nil)
	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

// ============= HEALTH ENDPOINT =============

func TestStandardRPCServer_healthEndpoint(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a direct HTTP request to the health endpoint
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.healthcheck(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var healthResp map[string]any

	err = json.Unmarshal(rr.Body.Bytes(), &healthResp)
	require.NoError(t, err)

	// Check health response structure
	status, exists := healthResp["status"]
	require.True(t, exists)
	assert.Contains(t, []string{"healthy", "degraded", "unhealthy"}, status)

	// Check that timestamp is present
	_, hasTimestamp := healthResp["timestamp"]
	assert.True(t, hasTimestamp)

	// Check that rpc_methods count is present
	methodCount, hasMethodCount := healthResp["rpc_methods"]
	assert.True(t, hasMethodCount)
	assert.IsType(t, float64(0), methodCount) // JSON numbers are float64
}

func TestStandardRPCServer_healthEndpoint_wrongMethod(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test with POST method (should fail)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	server.healthcheck(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

// ============= BATCH REQUESTS =============

func TestStandardRPCServer_batchRequests(t *testing.T) {
	server := NewStandardRPCServer(nil)

	// Add test methods
	server.AddMethod("add", func(_ context.Context, params []any) (any, error) {
		a, ok := params[0].(float64)
		if !ok {
			return nil, errInvalidParameterType
		}

		b, ok := params[1].(float64)
		if !ok {
			return nil, errInvalidParameterType
		}

		return a + b, nil
	})

	server.AddMethod("multiply", func(_ context.Context, params []any) (any, error) {
		a, ok := params[0].(float64)
		if !ok {
			return nil, errInvalidParameterType
		}

		b, ok := params[1].(float64)
		if !ok {
			return nil, errInvalidParameterType
		}

		return a * b, nil
	})

	// Test batch request with multiple valid calls
	batchReq := []JSONRPCRequest{
		{
			JSONRPC: "2.0",
			Method:  "add",
			Params:  []any{2, 3},
			ID:      1,
		},
		{
			JSONRPC: "2.0",
			Method:  "multiply",
			Params:  []any{4, 5},
			ID:      2,
		},
	}

	reqBody, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var batchResp []JSONRPCResponse

	err = json.Unmarshal(rr.Body.Bytes(), &batchResp)
	require.NoError(t, err)

	assert.Len(t, batchResp, 2)
	assert.InDelta(t, float64(5), batchResp[0].Result, 0.001) // 2 + 3 = 5
	assert.InDelta(t, float64(1), batchResp[0].ID, 0.001)
	assert.InDelta(t, float64(20), batchResp[1].Result, 0.001) // 4 * 5 = 20
	assert.InDelta(t, float64(2), batchResp[1].ID, 0.001)
}

func TestStandardRPCServer_batchRequestsWithErrors(t *testing.T) {
	server := NewStandardRPCServer(nil)

	// Add only one method
	server.AddMethod("add", func(_ context.Context, params []any) (any, error) {
		a, ok := params[0].(float64)
		if !ok {
			return nil, errInvalidParameterType
		}

		b, ok := params[1].(float64)
		if !ok {
			return nil, errInvalidParameterType
		}

		return a + b, nil
	})

	// Test batch request with mix of valid and invalid calls
	batchReq := []JSONRPCRequest{
		{
			JSONRPC: "2.0",
			Method:  "add",
			Params:  []any{2, 3},
			ID:      1,
		},
		{
			JSONRPC: "2.0",
			Method:  "nonexistent",
			Params:  []any{},
			ID:      2,
		},
		{
			JSONRPC: "2.0",
			Method:  "add",
			Params:  []any{4, 5},
			ID:      3,
		},
	}

	reqBody, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var batchResp []JSONRPCResponse

	err = json.Unmarshal(rr.Body.Bytes(), &batchResp)
	require.NoError(t, err)

	assert.Len(t, batchResp, 3)

	// First request should succeed
	assert.NotNil(t, batchResp[0].Result)
	assert.InDelta(t, float64(5), batchResp[0].Result, 0.001)
	assert.InDelta(t, float64(1), batchResp[0].ID, 0.001)
	assert.Nil(t, batchResp[0].Error)

	// Second request should fail
	assert.Nil(t, batchResp[1].Result)
	assert.InDelta(t, float64(2), batchResp[1].ID, 0.001)
	assert.NotNil(t, batchResp[1].Error)
	assert.Equal(t, -32601, batchResp[1].Error.Code)

	// Third request should succeed
	assert.NotNil(t, batchResp[2].Result)
	assert.InDelta(t, float64(9), batchResp[2].Result, 0.001)
	assert.InDelta(t, float64(3), batchResp[2].ID, 0.001)
	assert.Nil(t, batchResp[2].Error)
}

func TestStandardRPCServer_emptyBatchRequest(t *testing.T) {
	server := NewStandardRPCServer(nil)

	// Test empty batch request
	batchReq := []JSONRPCRequest{}

	reqBody, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var errorResp []JSONRPCResponse

	err = json.Unmarshal(rr.Body.Bytes(), &errorResp)
	require.NoError(t, err)

	assert.Len(t, errorResp, 1)
	assert.NotNil(t, errorResp[0].Error)
	assert.Equal(t, -32600, errorResp[0].Error.Code)
	assert.Contains(t, errorResp[0].Error.Message, "empty batch")
}

// Test middleware functionality

type testMiddleware struct {
	requestProcessed  bool
	responseProcessed bool
	shouldError       bool
	contextKey        string
	contextValue      string
}

func (m *testMiddleware) ProcessRequest(
	_ http.ResponseWriter,
	r *http.Request,
) error {
	m.requestProcessed = true
	if m.shouldError {
		return &Error{Code: -32000, Message: "Middleware blocked request"}
	}
	// Add something to request headers for testing
	if m.contextKey != "" {
		r.Header.Set(m.contextKey, m.contextValue)
	}

	return nil
}

func (m *testMiddleware) ProcessResponse(
	w http.ResponseWriter,
	_ *http.Request,
	_ JSONRPCResponse,
) error {
	m.responseProcessed = true
	// Add a header to the response to verify middleware ran
	if m.contextKey != "" {
		w.Header().Set("X-Middleware-Test", m.contextValue)
	}

	return nil
}

// Middleware that fails response processing for a specific ID
type failingResponseMiddleware struct {
	failID any
}

func (*failingResponseMiddleware) ProcessRequest(_ http.ResponseWriter, _ *http.Request) error {
	return nil
}

func (m *failingResponseMiddleware) ProcessResponse(
	_ http.ResponseWriter,
	_ *http.Request,
	resp JSONRPCResponse,
) error {
	if resp.ID == m.failID {
		return fmt.Errorf("middleware failed for response ID %v: %w", m.failID, errMiddlewareFailed)
	}

	return nil
}

// ============= MIDDLEWARE =============

func TestStandardRPCServer_middleware(t *testing.T) {
	mw := &testMiddleware{
		contextKey:   "test_key",
		contextValue: "test_value",
	}
	server := NewStandardRPCServer(nil)
	server.middlewares = []Middleware{mw}

	// Add a method that just returns success
	server.AddMethod("test", func(_ context.Context, _ []any) (any, error) {
		return testSuccessMessage, nil
	})

	// Test single request
	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"method": "test",
		"params": [],
		"id": 1
	}`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, mw.requestProcessed)
	assert.True(t, mw.responseProcessed)
	assert.Equal(t, "test_value", rr.Header().Get("X-Middleware-Test"))

	var response JSONRPCResponse

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Result)
}

func TestStandardRPCServer_middlewareBlocksRequest(t *testing.T) {
	mw := &testMiddleware{shouldError: true}
	server := NewStandardRPCServer(nil)
	server.middlewares = []Middleware{mw}

	server.AddMethod("test", func(_ context.Context, _ []any) (any, error) {
		return testShouldNotReachMessage, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"method": "test",
		"params": [],
		"id": 1
	}`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, mw.requestProcessed)
	assert.False(t, mw.responseProcessed) // Response middleware shouldn't run on request error

	var response JSONRPCResponse

	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32000, response.Error.Code)
	assert.Equal(t, "Middleware blocked request", response.Error.Message)
}

func TestStandardRPCServer_middlewareBatch(t *testing.T) {
	mw := &testMiddleware{
		contextKey:   "batch_test",
		contextValue: "batch_value",
	}
	server := NewStandardRPCServer(nil)
	server.middlewares = []Middleware{mw}

	server.AddMethod("batch_test", func(_ context.Context, _ []any) (any, error) {
		return "batch_success", nil
	})

	batchReq := []JSONRPCRequest{
		{
			JSONRPC: "2.0",
			Method:  "batch_test",
			Params:  []any{},
			ID:      1,
		},
	}
	reqBody, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleRPC(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, mw.requestProcessed)
	assert.True(t, mw.responseProcessed)

	var responses []JSONRPCResponse

	err = json.Unmarshal(rr.Body.Bytes(), &responses)
	require.NoError(t, err)
	assert.Len(t, responses, 1)
	require.NoError(t, err)
	assert.Len(t, responses, 1)
	assert.Equal(t, "batch_success", responses[0].Result)
}

func TestStandardRPCServer_middlewareBatchPartialFailure(t *testing.T) {
	server := NewStandardRPCServer(nil)
	server.middlewares = []Middleware{&failingResponseMiddleware{failID: float64(2)}}

	server.AddMethod("test_method", func(_ context.Context, _ []any) (any, error) {
		return testSuccessMessage, nil
	})

	// Batch with 3 requests - middleware will fail for ID 2
	batchReq := []JSONRPCRequest{
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 1},
		{
			JSONRPC: "2.0",
			Method:  "test_method",
			Params:  []any{},
			ID:      2,
		}, // This will fail middleware
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 3},
	}
	reqBody, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	server.handleRPC(w, req)

	assert.Equal(t, 200, w.Code)

	var responses []JSONRPCResponse

	err = json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)
	assert.Len(t, responses, 3)

	// First response should succeed
	assert.InEpsilon(t, float64(1), responses[0].ID, 0.001)
	assert.Equal(t, testSuccessMessage, responses[0].Result)
	assert.Nil(t, responses[0].Error)

	// Second response should fail due to middleware
	assert.InEpsilon(t, float64(2), responses[1].ID, 0.001)
	assert.Nil(t, responses[1].Result)
	assert.NotNil(t, responses[1].Error)
	assert.Equal(t, -32603, responses[1].Error.Code)
	assert.Contains(t, responses[1].Error.Message, "middleware failed")

	// Third response should succeed
	assert.InEpsilon(t, float64(3), responses[2].ID, 0.001)
	assert.Equal(t, testSuccessMessage, responses[2].Result)
	assert.Nil(t, responses[2].Error)
}

func TestStandardRPCServer_middlewareRequestBlocksBatch(t *testing.T) {
	mw := &testMiddleware{shouldError: true}
	server := NewStandardRPCServer(nil)
	server.middlewares = []Middleware{mw}

	server.AddMethod("test_method", func(_ context.Context, _ []any) (any, error) {
		return testShouldNotReachMessage, nil
	})

	// Batch request - request middleware should block the entire batch
	batchReq := []JSONRPCRequest{
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 1},
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 2},
	}
	reqBody, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	server.handleRPC(w, req)

	// Request middleware blocks the entire request, so we should get an error response
	assert.Equal(t, 200, w.Code)
	assert.True(t, mw.requestProcessed)
	assert.False(t, mw.responseProcessed) // Response middleware shouldn't run

	var response JSONRPCResponse

	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32000, response.Error.Code)
	assert.Equal(t, "Middleware blocked request", response.Error.Message)
}

func TestStandardRPCServer_middlewareMultipleResponseFailures(t *testing.T) {
	server := NewStandardRPCServer(nil)
	server.middlewares = []Middleware{
		&failingResponseMiddleware{failID: float64(2)},
		&failingResponseMiddleware{failID: float64(4)},
	}

	server.AddMethod("test_method", func(_ context.Context, _ []any) (any, error) {
		return testSuccessMessage, nil
	})

	// Batch with 5 requests - middleware will fail for IDs 2 and 4
	batchReq := []JSONRPCRequest{
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 1},
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 2}, // Will fail
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 3},
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 4}, // Will fail
		{JSONRPC: "2.0", Method: "test_method", Params: []any{}, ID: 5},
	}
	reqBody, err := json.Marshal(batchReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	server.handleRPC(w, req)

	assert.Equal(t, 200, w.Code)

	var responses []JSONRPCResponse

	err = json.Unmarshal(w.Body.Bytes(), &responses)
	require.NoError(t, err)
	assert.Len(t, responses, 5)

	// First response should succeed
	assert.InEpsilon(t, float64(1), responses[0].ID, 0.001)
	assert.Equal(t, testSuccessMessage, responses[0].Result)
	assert.Nil(t, responses[0].Error)

	// Second response should fail
	assert.InEpsilon(t, float64(2), responses[1].ID, 0.001)
	assert.Nil(t, responses[1].Result)
	assert.NotNil(t, responses[1].Error)
	assert.Equal(t, -32603, responses[1].Error.Code)

	// Third response should succeed
	assert.InEpsilon(t, float64(3), responses[2].ID, 0.001)
	assert.Equal(t, testSuccessMessage, responses[2].Result)
	assert.Nil(t, responses[2].Error)

	// Fourth response should fail
	assert.InEpsilon(t, float64(4), responses[3].ID, 0.001)
	assert.Nil(t, responses[3].Result)
	assert.NotNil(t, responses[3].Error)
	assert.Equal(t, -32603, responses[3].Error.Code)

	// Fifth response should succeed
	assert.InEpsilon(t, float64(5), responses[4].ID, 0.001)
	assert.Equal(t, testSuccessMessage, responses[4].Result)
	assert.Nil(t, responses[4].Error)
}

// ============= CORS TESTS =============

func TestStandardRPCServer_corsHeaders(t *testing.T) {
	tests := []struct {
		name            string
		corsConfig      *CORSConfig
		expectedOrigin  string
		expectedMethods string
		expectedHeaders string
	}{
		{
			name: "custom CORS config",
			corsConfig: &CORSConfig{
				AllowOrigin:  "https://example.com",
				AllowMethods: "GET, POST, PUT",
				AllowHeaders: "Content-Type, Authorization",
			},
			expectedOrigin:  "https://example.com",
			expectedMethods: "GET, POST, PUT",
			expectedHeaders: "Content-Type, Authorization",
		},
		{
			name: "partial CORS config - only origin",
			corsConfig: &CORSConfig{
				AllowOrigin: "https://test.com",
			},
			expectedOrigin:  "https://test.com",
			expectedMethods: "POST, OPTIONS",
			expectedHeaders: "Content-Type",
		},
		{
			name: "partial CORS config - only methods",
			corsConfig: &CORSConfig{
				AllowMethods: "GET, POST, OPTIONS",
			},
			expectedOrigin:  "*",
			expectedMethods: "GET, POST, OPTIONS",
			expectedHeaders: "Content-Type",
		},
		{
			name: "partial CORS config - only headers",
			corsConfig: &CORSConfig{
				AllowHeaders: "X-Custom-Header, Content-Type",
			},
			expectedOrigin:  "*",
			expectedMethods: "POST, OPTIONS",
			expectedHeaders: "X-Custom-Header, Content-Type",
		},
		{
			name:            "nil CORS config - defaults",
			corsConfig:      nil,
			expectedOrigin:  "*",
			expectedMethods: "POST, OPTIONS",
			expectedHeaders: "Content-Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewStandardRPCServer(tt.corsConfig)

			// Test RPC endpoint
			req := httptest.NewRequest(
				http.MethodPost,
				"/rpc",
				bytes.NewReader(
					[]byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`),
				),
			)
			w := httptest.NewRecorder()

			server.handleRPC(w, req)

			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
			assert.Equal(t, tt.expectedMethods, w.Header().Get("Access-Control-Allow-Methods"))
			assert.Equal(t, tt.expectedHeaders, w.Header().Get("Access-Control-Allow-Headers"))
		})
	}
}

func TestStandardRPCServer_corsOptionsPreflight(t *testing.T) {
	server := NewStandardRPCServer(&CORSConfig{
		AllowOrigin:  "https://example.com",
		AllowMethods: "GET, POST, PUT, OPTIONS",
		AllowHeaders: "Content-Type, Authorization",
	})

	req := httptest.NewRequest(http.MethodOptions, "/rpc", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	w := httptest.NewRecorder()
	server.handleRPC(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, 200, w.Code)
}

func TestStandardRPCServer_corsHealthEndpoint(t *testing.T) {
	server := NewStandardRPCServer(&CORSConfig{
		AllowOrigin:  "https://health.example.com",
		AllowMethods: "GET, HEAD",
		AllowHeaders: "Accept",
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.healthcheck(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "https://health.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, HEAD", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Accept", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, 200, w.Code)
}

// ============= BLOCK METHODS TESTS =============

// func TestStandardRPCServer_getBlockByNumber(t *testing.T) {
// 	server, _, cleanup := setupTestEnvironment(t)
// 	defer cleanup()

// 	t.Run("requires 1 param", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByNumber", []any{})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 	})

// 	t.Run("invalid param type", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByNumber", []any{true})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid block number")
// 	})

// 	t.Run("invalid hex string", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByNumber", []any{"0xZZ"})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid hex block number")
// 	})

// 	t.Run("negative numeric", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByNumber", []any{-1})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid block number: negative")
// 	})

// 	t.Run("valid decimal but not found", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByNumber", []any{"123"})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "failed to get block by number 123")
// 	})

// 	t.Run("valid hex but not found", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByNumber", []any{"0x7b"})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		// Parsed as decimal internally
// 		assert.Contains(t, resp.Error.Message, "failed to get block by number 123")
// 	})
// }

// func TestStandardRPCServer_getBlockByHash(t *testing.T) {
// 	server, _, cleanup := setupTestEnvironment(t)
// 	defer cleanup()

// 	t.Run("requires 1 param", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByHash", []any{})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 	})

// 	t.Run("param must be string", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByHash", []any{123})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		// matches ErrHashParameterMustBeString message
// 		assert.Contains(t, resp.Error.Message, "hash parameter must be a string")
// 	})

// 	t.Run("invalid hash format", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlockByHash", []any{"0x123"})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid hash format")
// 	})

// 	t.Run("valid hash but not found", func(t *testing.T) {
// 		h := sha256.Sum256([]byte("non-existent-block"))
// 		hashStr := "0x" + hex.EncodeToString(h[:])

// 		rr := makeJSONRPCRequest(t, server, "getBlockByHash", []any{hashStr})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "failed to get block by hash")
// 		assert.Contains(t, resp.Error.Message, hashStr)
// 	})
// }

// func TestStandardRPCServer_getBlocks(t *testing.T) {
// 	server, appchainDB, cleanup := setupTestEnvironment(t)
// 	defer cleanup()

// 	// Ensure the RPC method is registered for these tests even if
// 	// AddBlockMethods doesn't register it yet.
// 	bm := NewBlockMethods(appchainDB)
// 	server.AddMethod("getBlocks", bm.GetBlocks) // uses methods in methods_block.go

// 	t.Run("requires 1 param", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlocks", []any{})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 	})

// 	t.Run("invalid param type", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlocks", []any{true})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid block number")
// 	})

// 	t.Run("invalid hex string", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlocks", []any{"0xZZ"})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid hex block number")
// 	})

// 	t.Run("zero count returns empty array", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlocks", []any{0})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.Nil(t, resp.Error)

// 		blocks, ok := resp.Result.([]any)
// 		require.True(t, ok, "result should be array, got: %T (%v)", resp.Result, resp.Result)
// 		assert.Len(t, blocks, 0)
// 	})

// 	t.Run("empty DB returns error", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getBlocks", []any{5})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "failed to get latest 5 blocks")
// 	})

// 	t.Run("order & shape: newest-first fields/values", func(t *testing.T) {
// 		// Seed 10 blocks numbered 0..9 into BlockNumberBucket.
// 		// Each value is CBOR-encoded block.Block with fields:
// 		// number, hash, stateroot, timestamp. The state root is computed via method.
// 		err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
// 			for i := 0; i < 10; i++ {
// 				// Prepare a realistic block.Block
// 				h := sha256.Sum256([]byte(fmt.Sprintf("block-%d", i)))
// 				b := block.Block{
// 					BlockNumber:    uint64(i),
// 					BlockHash:      h,
// 					Timestamp: uint64(1630000000 + i*10),
// 				}
// 				b.BlockRoot = b.StateRoot()

// 				enc, e := cbor.Marshal(b)
// 				if e != nil {
// 					return e
// 				}
// 				if e := tx.Put(block.BlockNumberBucket, block.NumberToBytes(b.Number()), enc); e != nil {
// 					return e
// 				}
// 			}
// 			return nil
// 		})
// 		require.NoError(t, err)

// 		// Ask for the latest 5 → expect 9,8,7,6,5 (newest first)
// 		rr := makeJSONRPCRequest(t, server, "getBlocks", []any{5})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.Nil(t, resp.Error)

// 		result, ok := resp.Result.([]any)
// 		require.True(t, ok, "result should be array, got: %T", resp.Result)
// 		require.Len(t, result, 5)

// 		wantNums := []uint64{9, 8, 7, 6, 5}
// 		expectedFields := []string{"number", "hash", "stateroot", "timestamp"}

// 		for i, entryAny := range result {
// 			entry, ok := entryAny.(map[string]any)
// 			require.True(t, ok, "entry should be object with Fields/Values, got: %T", entryAny)

// 			fieldsAny, ok := entry["Fields"].([]any)
// 			require.True(t, ok, "Fields should be array, got: %T", entry["Fields"])

// 			valuesAny, ok := entry["Values"].([]any)
// 			require.True(t, ok, "Values should be array, got: %T", entry["Values"])

// 			// Convert []any -> []string
// 			toStrings := func(a []any) []string {
// 				out := make([]string, len(a))
// 				for i := range a {
// 					out[i], _ = a[i].(string)
// 				}
// 				return out
// 			}
// 			fields := toStrings(fieldsAny)
// 			values := toStrings(valuesAny)

// 			// Shape checks
// 			assert.Equal(t, expectedFields, fields)
// 			require.Len(t, values, 4)

// 			// number
// 			assert.Equal(t, fmt.Sprintf("%d", wantNums[i]), values[0])
// 			// hash, stateroot must be 0x-prefixed hex
// 			assert.True(t, strings.HasPrefix(values[1], "0x"))
// 			assert.True(t, strings.HasPrefix(values[2], "0x"))
// 			// timestamp follows deterministic pattern used in seeding
// 			wantTs := 1630000000 + wantNums[i]*10
// 			assert.Equal(t, fmt.Sprintf("%d", wantTs), values[3])
// 		}
// 	})
// }

// // ============= BLOCK TRANSACTIONS BY NUMBER (RPC) =============

// // TODO replace it with TestTransaction
// type rpcTx struct {
// 	From  string `json:"from"  cbor:"1,keyasint"`
// 	To    string `json:"to"    cbor:"2,keyasint"`
// 	Value int    `json:"value" cbor:"3,keyasint"`
// }

// func normalizeRPCTxs(t *testing.T, v any) []rpcTx {
// 	t.Helper()
// 	b, err := json.Marshal(v)
// 	require.NoError(t, err)
// 	var out []rpcTx
// 	require.NoError(t, json.Unmarshal(b, &out))
// 	return out
// }

// func TestStandardRPCServer_getTransactionsByBlockNumber_Params(t *testing.T) {
// 	server, appchainDB, cleanup := setupTestEnvironment(t)
// 	defer cleanup()

// 	// Ensure the method is registered even if AddBlockMethods changes.
// 	bm := NewBlockMethods(appchainDB)
// 	server.AddMethod("getTransactionsByBlockNumber", bm.GetTransactionsByBlockNumber)

// 	t.Run("requires 1 param", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 	})

// 	t.Run("invalid param type", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{true})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid block number")
// 	})

// 	t.Run("invalid hex string", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{"0xZZ"})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid hex block number")
// 	})

// 	t.Run("negative numeric", func(t *testing.T) {
// 		rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{-1})
// 		require.Equal(t, http.StatusOK, rr.Code)

// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "invalid block number: negative")
// 	})

// 	t.Run("valid but block not found (decimal & hex)", func(t *testing.T) {
// 		// decimal
// 		rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{"123"})
// 		require.Equal(t, http.StatusOK, rr.Code)
// 		var resp JSONRPCResponse
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "failed to get transactions for block 123")

// 		// hex 0x7b → 123
// 		rr = makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{"0x7b"})
// 		require.Equal(t, http.StatusOK, rr.Code)
// 		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 		require.NotNil(t, resp.Error)
// 		assert.Equal(t, -32603, resp.Error.Code)
// 		assert.Contains(t, resp.Error.Message, "failed to get transactions for block 123")
// 	})
// }

// func TestStandardRPCServer_getTransactionsByBlockNumber_DirectEncoding(t *testing.T) {
// 	server, appchainDB, cleanup := setupTestEnvironment(t)
// 	defer cleanup()

// 	// Ensure the method is registered.
// 	bm := NewBlockMethods(appchainDB)
// 	server.AddMethod("getTransactionsByBlockNumber", bm.GetTransactionsByBlockNumber)

// 	// 1) Seed a block in BlockNumberBucket
// 	const num = uint64(77)
// 	h := sha256.Sum256([]byte(fmt.Sprintf("block-%d-direct", num)))
// 	blk := block.Block{BlockNumber: num, BlockHash: h, Timestamp: 1630000000 + num*10}
// 	blk.BlockRoot = blk.StateRoot()

// 	blkEnc, err := cbor.Marshal(blk)
// 	require.NoError(t, err)

// 	require.NoError(t, appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(block.BlockNumberBucket, block.NumberToBytes(num), blkEnc)
// 	}))

// 	// 2) Seed transactions (direct CBOR slice)
// 	want := []rpcTx{
// 		{From: "isaac", To: "judy", Value: 10},
// 		{From: "kate", To: "liam", Value: 11},
// 	}
// 	txsEnc, err := cbor.Marshal(want)
// 	require.NoError(t, err)

// 	require.NoError(t, appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(block.BlockTransactionsBucket, block.NumberToBytes(num), txsEnc)
// 	}))

// 	// 3) Call RPC with decimal param
// 	rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{int(num)})
// 	require.Equal(t, http.StatusOK, rr.Code)

// 	var resp JSONRPCResponse
// 	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 	require.Nil(t, resp.Error)

// 	got := normalizeRPCTxs(t, resp.Result)
// 	assert.Equal(t, want, got)

// 	// 4) Call RPC with hex param ("0x4d" == 77)
// 	rr = makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{"0x4d"})
// 	require.Equal(t, http.StatusOK, rr.Code)

// 	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 	require.Nil(t, resp.Error)

// 	got = normalizeRPCTxs(t, resp.Result)
// 	assert.Equal(t, want, got)
// }

// func TestStandardRPCServer_getTransactionsByBlockNumber_NestedEncoding(t *testing.T) {
// 	server, appchainDB, cleanup := setupTestEnvironment(t)
// 	defer cleanup()

// 	bm := NewBlockMethods(appchainDB)
// 	server.AddMethod("getTransactionsByBlockNumber", bm.GetTransactionsByBlockNumber)

// 	// 1) Seed a block
// 	const num = uint64(78)
// 	h := sha256.Sum256([]byte(fmt.Sprintf("block-%d-nested", num)))
// 	blk := block.Block{BlockNumber: num, BlockHash: h, Timestamp: 1630000000 + num*10}
// 	blk.BlockRoot = blk.StateRoot()
// 	blkEnc, err := cbor.Marshal(blk)
// 	require.NoError(t, err)

// 	require.NoError(t, appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(block.BlockNumberBucket, block.NumberToBytes(num), blkEnc)
// 	}))

// 	// 2) Seed transactions as CBOR([][]byte{ CBOR(tx), ... })
// 	want := []rpcTx{
// 		{From: "eve", To: "frank", Value: 7},
// 		{From: "grace", To: "heidi", Value: 8},
// 	}
// 	var nested [][]byte
// 	for _, txv := range want {
// 		b, e := cbor.Marshal(txv)
// 		require.NoError(t, e)
// 		nested = append(nested, b)
// 	}
// 	nestedEnc, err := cbor.Marshal(nested)
// 	require.NoError(t, err)

// 	require.NoError(t, appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(block.BlockTransactionsBucket, block.NumberToBytes(num), nestedEnc)
// 	}))

// 	// 3) Call RPC
// 	rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{num})
// 	require.Equal(t, http.StatusOK, rr.Code)

// 	var resp JSONRPCResponse
// 	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 	require.Nil(t, resp.Error)

// 	got := normalizeRPCTxs(t, resp.Result)
// 	assert.Equal(t, want, got)
// }

// func TestStandardRPCServer_getTransactionsByBlockNumber_EmptyTxs(t *testing.T) {
// 	server, appchainDB, cleanup := setupTestEnvironment(t)
// 	defer cleanup()

// 	bm := NewBlockMethods(appchainDB)
// 	server.AddMethod("getTransactionsByBlockNumber", bm.GetTransactionsByBlockNumber)

// 	// Seed a block but DO NOT write any txs entry
// 	const num = uint64(79)
// 	h := sha256.Sum256([]byte(fmt.Sprintf("block-%d-empty", num)))
// 	blk := block.Block{BlockNumber: num, BlockHash: h, Timestamp: 1630000000 + num*10}
// 	blk.BlockRoot = blk.StateRoot()
// 	blkEnc, err := cbor.Marshal(blk)
// 	require.NoError(t, err)
// 	require.NoError(t, appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(block.BlockNumberBucket, block.NumberToBytes(num), blkEnc)
// 	}))

// 	rr := makeJSONRPCRequest(t, server, "getTransactionsByBlockNumber", []any{num})
// 	require.Equal(t, http.StatusOK, rr.Code)

// 	var resp JSONRPCResponse
// 	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
// 	require.Nil(t, resp.Error)

// 	// Should be an empty array
// 	arr, ok := resp.Result.([]any)
// 	require.True(t, ok, "result should be array, got %T (%v)", resp.Result, resp.Result)
// 	assert.Len(t, arr, 0)
// }
