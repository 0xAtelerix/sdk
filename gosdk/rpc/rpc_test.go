package rpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

// TestTransaction - test transaction implementation
type TestTransaction[R TestReceipt] struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value int    `json:"value"`
}

func (t *TestTransaction[R]) Unmarshal(b []byte) error {
	return json.Unmarshal(b, t)
}

func (t TestTransaction[R]) Marshal() ([]byte, error) {
	return json.Marshal(t)
}

func (t TestTransaction[R]) Hash() [32]byte {
	s := t.From + t.To + strconv.Itoa(t.Value)

	return sha256.Sum256([]byte(s))
}

func (TestTransaction[R]) Process(
	_ kv.RwTx,
) (r TestReceipt, txs []apptypes.ExternalTransaction, err error) {
	return TestReceipt{ReceiptStatus: apptypes.ReceiptConfirmed}, nil, nil
}

// TestReceipt - test receipt implementation
type TestReceipt struct {
	ReceiptStatus   apptypes.TxReceiptStatus `json:"status"`
	TransactionHash [32]byte                 `json:"txHash"`
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
				receipt.ReceiptBucket: {},
			}
		}).
		Open()
	require.NoError(t, err)

	// Create txpool
	txPool := txpool.NewTxPool[*TestTransaction[TestReceipt]](localDB)

	// Create RPC server
	server = NewStandardRPCServer()

	// Add standard methods to maintain compatibility with existing tests
	AddStandardMethods(server, appchainDB, txPool)

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
	require.True(t, ok)

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
	txs := []*TestTransaction[TestReceipt]{
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
	receiptData, err := json.Marshal(testReceipt)
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

func TestStandardRPCServer_customMethod(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Add a custom method
	server.AddCustomMethod("customTest", func(_ context.Context, params []any) (any, error) {
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

// Benchmark tests
func BenchmarkRPCServer_getChainInfo(b *testing.B) {
	server, _, cleanup := setupTestEnvironment(&testing.T{})
	defer cleanup()

	b.ResetTimer()

	for range b.N {
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString(`{
			"jsonrpc": "2.0",
			"method": "getChainInfo",
			"params": [],
			"id": 1
		}`))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		server.handleRPC(rr, req)
	}
}

// Example usage test
func ExampleStandardRPCServer() {
	// This example shows how to set up and use the StandardRPCServer

	// Create RPC server
	server := NewStandardRPCServer()

	// Add custom method
	server.AddCustomMethod("ping", func(_ context.Context, _ []any) (any, error) {
		return "pong", nil
	})

	// Start server (commented out for example)
	// server.StartHTTPServer(8545)

	fmt.Println("RPC server configured successfully")
	// Output: RPC server configured successfully
}

func TestStandardRPCServer_healthEndpoint(t *testing.T) {
	server, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a direct HTTP request to the health endpoint
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
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
