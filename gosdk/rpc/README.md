# Atelerix RPC Server

A lightweight, composable JSON-RPC 2.0 server for blockchain applications with HTTP middleware support.
standard methods/headers)
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

    // Add standard blockchain methods
    rpc.AddStandardMethods[MyTransaction, MyReceipt](server, appchainDB, txpool, func() *MyBlock {
        // MyBlock is your application-specific block payload type.
        return &MyBlock{}
    })

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
rpc.AddStandardMethods[MyTransaction, MyReceipt](server, appchainDB, txpool, func() *MyBlock {
    return &MyBlock{}
})
```

Includes: transaction submission, status queries, receipts, and txpool operations.

### Individual Method Sets

```go
// Transaction operations
rpc.AddTransactionMethods[MyTransaction, MyReceipt](server, txpool, appchainDB)

// TxPool operations
rpc.AddTxPoolMethods[MyTransaction, MyReceipt](server, txpool)

// Receipt queries
rpc.AddReceiptMethods[MyReceipt](server, appchainDB)
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
rpc.AddStandardMethods[MyTransaction, MyReceipt](server, db, txpool, func() *MyBlock {
    return &MyBlock{}
})

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

- `AddStandardMethods[T, R](server, db, txpool, func() *YourBlock { return &YourBlock{} })` - All methods
- `AddTransactionMethods[T, R](server, txpool, db)` - Transaction status during lifecycle
- `AddTxPoolMethods[T, R](server, txpool)` - Submit/query transactions to/from pool
- `AddReceiptMethods[R](server, db)` - Receipt queries once processed/failed

### Available RPC Methods

| Method | Description |
|--------|-------------|
| `sendTransaction` | Submit transaction to pool |
| `getTransactionByHash` | Get transaction from pool |
| `getTransactionStatus` | Get comprehensive status |
| `getPendingTransactions` | List pending transactions |
| `getTransactionReceipt` | Get transaction receipt |

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

```go
// Test with middleware
server := rpc.NewStandardRPCServer(nil)
server.AddMiddleware(&MyMiddleware{})
server.AddMethod("test", func(ctx context.Context, params []any) (any, error) {
    return "ok", nil
})

// Test request
req := httptest.NewRequest("POST", "/rpc", strings.NewReader(`{"jsonrpc":"2.0","method":"test","id":1}`))
w := httptest.NewRecorder()
server.handleRPC(w, req)
```
