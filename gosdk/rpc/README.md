# Atelerix RPC Server

A lightweight, composable JSON-RPC 2.0 server for blockchain applications with HTTP middleware support, CORS configuration, and modular method sets for transactions, receipts, blocks, and schema discovery.
## Table of Contents

- [Quick Start](#quick-start)
- [Key Features](#key-features)
- [CORS Configuration](#cors-configuration)
- [Middleware](#middleware)
- [Method Sets](#method-sets)
- [Custom Methods](#custom-methods)
- [Batch Requests](#batch-requests)
- [Health Check](#health-check)
- [Complete Example](#complete-example)
- [API Reference](#api-reference)
- [Error Handling](#error-handling)
- [Testing](#testing)
- [Architecture](#architecture)

## Quick Start

```go
package main

import (
    "context"
    "log"
    "github.com/0xAtelerix/sdk/gosdk/rpc"
)

func main() {
    // Create server
    server := rpc.NewStandardRPCServer(nil) // nil enables default CORS

    // Add middleware (optional)
    server.AddMiddleware(&LoggingMiddleware{})

    // Add standard methods (transactions, receipts, blocks, schema)
    rpc.AddStandardMethods[MyTransaction, MyReceipt, MyBlock](server, appchainDB, txpool, chainID)

    // Start server
    log.Println("RPC server starting on :8080")
    if err := server.StartHTTPServer(context.Background(), ":8080"); err != nil {
        log.Fatal(err)
    }
}
```

## Key Features

- **JSON-RPC 2.0 compliant** with batch request support
- **HTTP middleware** for request/response processing
- **Modular method sets** - add only what you need
- **Health monitoring** endpoint
- **CORS support** for web applications
- **Type-safe** with Go generics

## CORS Configuration

Configure Cross-Origin Resource Sharing for web applications:

```go
// Default CORS (allows all origins - POST,OPTIONS for RPC, GET for health)
server := rpc.NewStandardRPCServer(nil)

// Custom CORS configuration
server := rpc.NewStandardRPCServer(&rpc.CORSConfig{
    AllowOrigin:  "https://myapp.com",           // Specific origin
    AllowMethods: "GET, POST, PUT, OPTIONS",    // Allowed methods
    AllowHeaders: "Content-Type, Authorization", // Allowed headers
})

// Note: CORS cannot be disabled - headers are always set for security
```

**CORS Behavior:**
- `nil` config: Default CORS (`*` origin, `POST, OPTIONS` methods for RPC, `GET` for health)
- Custom config: CORS with your specified settings
- **CORS headers are always enabled** for browser compatibility

## Middleware

Add HTTP-level middleware for authentication, logging, rate limiting, etc.

```go
// Request middleware - runs before JSON-RPC processing
type AuthMiddleware struct{}

func (m *AuthMiddleware) ProcessRequest(w http.ResponseWriter, r *http.Request) error {
    if r.Header.Get("Authorization") == "" {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return fmt.Errorf("missing auth header")
    }
    return nil
}

func (m *AuthMiddleware) ProcessResponse(w http.ResponseWriter, r *http.Request, resp rpc.JSONRPCResponse) error {
    // Add response headers
    w.Header().Set("X-Processed-By", "AuthMiddleware")
    return nil
}

// Usage
server := rpc.NewStandardRPCServer(nil)
server.AddMiddleware(&AuthMiddleware{})
```

**Middleware Execution:**
- Request middleware runs before JSON parsing
- Can block requests early (auth, rate limiting)
- Response middleware runs per response in batches
- Individual batch responses fail independently

## Method Sets

### Standard Methods (Recommended)

```go
rpc.AddStandardMethods[MyTransaction, MyReceipt, MyBlock](
    server,
    appchainDB,
    txpool,
    chainID,
)
```

Includes: transaction submission, status queries, receipts, txpool operations, block queries, and schema discovery.

### Individual Method Sets

Add only the method sets you need:

```go
// Transaction operations (all transaction-related methods)
rpc.AddTransactionMethods[MyTransaction, MyReceipt](server, txpool, appchainDB)
// Provides: sendTransaction, getPendingTransactions, getTransaction,
//           getTransactionStatus, getTransactionsByBlockNumber, getExternalTransactions

// Receipt queries (finalized transaction receipts)
rpc.AddReceiptMethods[MyReceipt](server, appchainDB)
// Provides: getTransactionReceipt

// Block queries
rpc.AddBlockMethods[MyTransaction, MyReceipt, MyBlock](server, appchainDB, chainID)
// Provides: getBlock

// Schema discovery (JSON Schema for types)
rpc.AddSchemaMethod[MyTransaction, MyReceipt, MyBlock](server, chainID)
// Provides: getAppchainSchema
```

## Custom Methods

```go
server.AddMethod("getNetworkInfo", func(ctx context.Context, params []any) (any, error) {
    return map[string]any{
        "chainId": 1001,
        "version": "1.0.0",
        "peers": getPeerCount(),
    }, nil
})
```

## Batch Requests

Process multiple RPC calls in one request:

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '[
    {"jsonrpc": "2.0", "method": "getTransactionStatus", "params": ["0x123"], "id": 1},
    {"jsonrpc": "2.0", "method": "getPendingTransactions", "params": [], "id": 2}
  ]'
```

## Usage Examples

### Get Latest Block

```bash
# Get the latest block (no params)
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "getBlock", "params": [], "id": 1}'

# Get a specific block by number
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "getBlock", "params": [100], "id": 1}'
```

### Query Transaction Status

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "getTransactionStatus", "params": ["0xabc..."], "id": 1}'
```

## Health Check

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "healthy",
  "timestamp": "2025-09-25T10:30:00Z",
  "rpc_methods": 8
}
```

## Complete Example

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/0xAtelerix/sdk/gosdk/rpc"
)

type LoggingMiddleware struct{}

func (m *LoggingMiddleware) ProcessRequest(w http.ResponseWriter, r *http.Request) error {
    log.Printf("RPC Request: %s %s", r.Method, r.URL.Path)
    return nil
}

func (m *LoggingMiddleware) ProcessResponse(w http.ResponseWriter, r *http.Request, resp rpc.JSONRPCResponse) error {
    log.Printf("RPC Response ID: %v", resp.ID)
    return nil
}

func main() {
    server := rpc.NewStandardRPCServer(nil) // nil enables default CORS

    // Add middleware
    server.AddMiddleware(&LoggingMiddleware{})

    // Add standard methods
    rpc.AddStandardMethods[MyTransaction, MyReceipt, MyBlock](server, db, txpool, chainID)

    // Add custom method
    server.AddMethod("ping", func(ctx context.Context, params []any) (any, error) {
        return "pong", nil
    })

    log.Fatal(server.StartHTTPServer(context.Background(), ":8080"))
}
```

## API Reference

### Server Methods

- `NewStandardRPCServer(corsConfig *CORSConfig) *StandardRPCServer` - Create server with CORS config (nil = default CORS, custom config = specified CORS)
- `AddMethod(name string, handler func(context.Context, []any) (any, error))` - Add custom RPC method
- `AddMiddleware(middleware Middleware)` - Add HTTP middleware
- `StartHTTPServer(ctx context.Context, addr string) error` - Start HTTP server

### Method Sets

- `AddStandardMethods[T, R, Block](server, db, txpool, chainID)` - All methods including transactions, receipts, blocks, and schema discovery
- `AddTransactionMethods[T, R](server, txpool, db)` - All transaction-related methods (submit, query pending/finalized, status)
- `AddReceiptMethods[R](server, db)` - Query transaction receipts (finalized only)
- `AddBlockMethods[T, R, Block](server, db, chainID)` - Block query methods
- `AddSchemaMethod[T, R, Block](server, chainID)` - Schema discovery (JSON Schema for appchain types for explorer integration)

### Available RPC Methods

| Method | Category | Description |
|--------|----------|-------------|
| `sendTransaction` | Transactions | Submit transaction to pool |
| `getPendingTransactions` | Transactions | List pending transactions |
| `getTransaction` | Transactions | Get transaction by hash (checks finalized blocks + pending txpool) |
| `getTransactionsByBlockNumber` | Transactions | Get all transactions in a block |
| `getExternalTransactions` | Transactions | Get cross-chain transactions from a block |
| `getTransactionStatus` | Transactions | Get comprehensive transaction status (finalized + pending) |
| `getTransactionReceipt` | Receipts | Get transaction receipt |
| `getBlock` | Blocks | Get block by number (omit params to get latest block) |
| `getAppchainSchema` | Schema | Discover appchain block/transaction/receipt structure (JSON Schema) |

### Middleware Interface

```go
type Middleware interface {
    ProcessRequest(w http.ResponseWriter, r *http.Request) error
    ProcessResponse(w http.ResponseWriter, r *http.Request, response JSONRPCResponse) error
}
```

## Error Handling

- Request middleware errors block the entire request
- Response middleware errors affect individual batch responses
- Standard JSON-RPC error codes (-32601 method not found, -32700 parse error, etc.)

## Testing

The RPC package includes comprehensive test coverage with both unit and integration tests.

### Test Organization

**Unit Tests** (`methods_*_test.go`):
- Test method handlers directly without HTTP overhead
- Fast execution and easy debugging
- Focus on edge cases and error conditions
- Files:
  - `methods_receipt_test.go` - Receipt methods
  - `methods_tx_test.go` - All transaction methods (submit, pending, queries, status)
  - `methods_block_test.go` - Block methods
  - `methods_schema_test.go` - Schema discovery methods

**Integration Tests** (`rpc_test.go`):
- Test full HTTP/JSON-RPC stack
- Verify protocol compliance
- Test middleware, CORS, batch requests
- Ensure end-to-end functionality

### Running Tests

```bash
# Run all tests
go test ./gosdk/rpc -v

# Run specific test suite
go test ./gosdk/rpc -v -run TestReceiptMethods
go test ./gosdk/rpc -v -run TestTransactionMethods
go test ./gosdk/rpc -v -run TestBlockMethods
go test ./gosdk/rpc -v -run TestSchemaMethods

# Run integration tests only
go test ./gosdk/rpc -v -run TestStandardRPCServer
```

### Example Test

```go
// Unit test - direct method testing
func TestGetTransactionReceipt(t *testing.T) {
    methods := NewReceiptMethods[MyReceipt](appchainDB)

    result, err := methods.GetTransactionReceipt(context.Background(), []any{txHash})

    require.NoError(t, err)
    assert.NotNil(t, result)
}

// Integration test - HTTP/JSON-RPC testing
func TestRPCServer_GetReceipt(t *testing.T) {
    server := NewStandardRPCServer(nil)
    AddReceiptMethods[MyReceipt](server, appchainDB)

    req := httptest.NewRequest("POST", "/rpc",
        strings.NewReader(`{"jsonrpc":"2.0","method":"getTransactionReceipt","params":["0x123"],"id":1}`))
    w := httptest.NewRecorder()

    server.handleRPC(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}
```

## Architecture

### File Structure

```
rpc/
├── rpc.go                    # Core RPC server + schema discovery
├── types.go                  # Server types and interfaces
├── errors.go                 # Error definitions
├── utils.go                  # Utility functions
├── methods_receipt.go        # Receipt RPC methods
├── methods_tx.go             # All transaction RPC methods (submit, pending, queries, status)
├── methods_block.go          # Block query RPC methods
├── rpc_test.go               # All tests (integration + unit)
├── methods_receipt_test.go   # Receipt unit tests
├── methods_tx_test.go        # Transaction unit tests
├── methods_block_test.go     # Block unit tests
└── README.md                 # This file
```

### Design Principles

1. **Modularity**: Method sets are independent and can be added separately
2. **Type Safety**: Leverages Go generics for compile-time type checking
3. **Separation of Concerns**: Clear separation between HTTP layer, JSON-RPC protocol, and business logic
4. **Testability**: Both unit and integration test coverage
5. **Extensibility**: Easy to add custom methods and middleware

### Request Flow

```
HTTP Request
    ↓
CORS Headers Applied
    ↓
Request Middleware (auth, logging, etc.)
    ↓
JSON-RPC Parsing
    ↓
Method Router
    ↓
Method Handler (your business logic)
    ↓
Response Middleware
    ↓
JSON-RPC Response
    ↓
HTTP Response
```

### Method Organization

Methods are organized by their data source and purpose:

- **Transaction Methods**: All transaction operations (submit to pool, query pending, query finalized, get status)
- **Receipt Methods**: Query finalized transaction results
- **Block Methods**: Query blockchain blocks
- **Schema Methods**: Discover type structures

This organization ensures:
- Clear responsibility boundaries
- Single place for all transaction-related operations
- Efficient data access patterns
- Easy maintenance and testing
