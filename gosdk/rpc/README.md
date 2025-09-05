# Atelerix RPC Server

A lightweight, composable JSON-RPC 2.0 server for blockchain applications.

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
    server := rpc.NewStandardRPCServer()
    
    // Add methods you need - specify your transaction and receipt types
    rpc.AddStandardMethods[MyTransaction, MyReceipt](server, appchainDB, txpool)
    
    // Start server
    log.Fatal(server.StartHTTPServer(context.Background(), ":8080"))
}
```

## Key Features

- **Empty by default** - Add only the methods you need
- **Pre-built method sets** - Common blockchain operations ready to use
- **Custom methods** - Easy to extend with your own functionality
- **Health monitoring** - Built-in `/health` endpoint

## Available Method Sets

### Transaction Pool Methods
```go
rpc.AddTxPoolMethods[MyTransaction, MyReceipt](server, txpool)
```
- `sendTransaction` - Submit transactions
- `getTransactionByHash` - Retrieve transactions
- `getTransactionStatus` - Check transaction status
- `getPendingTransactions` - List pending transactions

### Receipt Methods
```go
rpc.AddReceiptMethods[MyReceipt](server, appchainDB)
```
- `getTransactionReceipt` - Get transaction receipts

### All Standard Methods
```go
rpc.AddStandardMethods[MyTransaction, MyReceipt](server, appchainDB, txpool)
```
Adds all transaction pool and receipt methods at once.

## Complete Integration Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/0xAtelerix/sdk/gosdk/rpc"
    // your other imports...
)

func main() {
    // Initialize your blockchain components
    appchainDB := initDatabase()
    txpool := initTxPool()
    
    // Create RPC server
    server := rpc.NewStandardRPCServer()
    
    // Option 1: Add all standard methods
    rpc.AddStandardMethods[MyTransaction, MyReceipt](server, appchainDB, txpool)
    
    // Option 2: Add only specific method sets
    // rpc.AddTxPoolMethods[MyTransaction, MyReceipt](server, txpool)
    // rpc.AddReceiptMethods[MyReceipt](server, appchainDB)
    
    // Add custom methods
    server.AddCustomMethod("getNetworkInfo", func(ctx context.Context, params []any) (any, error) {
        return map[string]any{
            "chainId": 1001,
            "networkName": "MyAppchain",
            "blockNumber": getCurrentBlockNumber(),
        }, nil
    })
    
    // Start server
    fmt.Println("Starting RPC server on :8080")
    if err := server.StartHTTPServer(context.Background(), ":8080"); err != nil {
        log.Fatal("Failed to start RPC server:", err)
    }
}

func initDatabase() kv.RwDB {
    // Your database initialization logic
    return nil
}

func initTxPool() apptypes.TxPoolInterface {
    // Your txpool initialization logic
    return nil
}

func getCurrentBlockNumber() uint64 {
    // Your logic to get current block number
    return 12345
}
```

## Configuration Examples

### Minimal Server (Custom Methods Only)

```go
server := rpc.NewStandardRPCServer()

server.AddCustomMethod("ping", func(ctx context.Context, params []any) (any, error) {
    return "pong", nil
})

server.StartHTTPServer(context.Background(), ":8080")
```

### Transaction-Only Server

```go
server := rpc.NewStandardRPCServer()
rpc.AddTxPoolMethods[MyTransaction, MyReceipt](server, txpool)
server.StartHTTPServer(context.Background(), ":8080")
```

### Full-Featured Server

```go
server := rpc.NewStandardRPCServer()
rpc.AddStandardMethods[MyTransaction, MyReceipt](server, appchainDB, txpool)

// Add custom business logic
server.AddCustomMethod("getStats", func(ctx context.Context, params []any) (any, error) {
    return getChainStatistics(), nil
})

server.StartHTTPServer(context.Background(), ":8080")
```

## Custom Methods

Add your own RPC methods:

```go
server.AddCustomMethod("getChainInfo", func(ctx context.Context, params []any) (any, error) {
    return map[string]string{
        "name": "MyChain",
        "version": "1.0.0",
    }, nil
})
```

### Advanced Custom Method Example

```go
// Method with parameter validation and database access
server.AddCustomMethod("getAccountBalance", func(ctx context.Context, params []any) (any, error) {
    if len(params) != 1 {
        return nil, fmt.Errorf("expected 1 parameter, got %d", len(params))
    }
    
    address, ok := params[0].(string)
    if !ok {
        return nil, fmt.Errorf("address must be a string")
    }
    
    // Your custom logic here
    balance := getBalanceFromDB(address)
    
    return map[string]any{
        "address": address,
        "balance": balance,
        "currency": "ETH",
    }, nil
})
```

### Error Handling in Custom Methods

```go
server.AddCustomMethod("validateTransaction", func(ctx context.Context, params []any) (any, error) {
    if len(params) == 0 {
        return nil, fmt.Errorf("transaction data required")
    }
    
    // Validate transaction
    if !isValidTransaction(params[0]) {
        return nil, fmt.Errorf("invalid transaction format")
    }
    
    return map[string]bool{"valid": true}, nil
})
```

## Health Check

Access server health at `GET /health`:

```json
{
  "status": "healthy",
  "timestamp": "2023-09-05T10:30:00Z",
  "rpc_methods": 5
}
```

## Method Names

⚠️ **Important**: Use exact method names when calling via JSON-RPC:

- ✅ `sendTransaction` 
- ✅ `getTransactionByHash`
- ✅ `getTransactionStatus`
- ✅ `getPendingTransactions`
- ✅ `getTransactionReceipt`

## Example Request

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "sendTransaction", 
    "params": [{"from": "0x...", "to": "0x...", "value": 100}],
    "id": 1
  }'
```

### More Example Requests

```bash
# Get transaction by hash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getTransactionByHash",
    "params": ["0x1234abcd..."],
    "id": 2
  }'

# Get pending transactions
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getPendingTransactions",
    "params": [],
    "id": 3
  }'

# Custom method call
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getChainInfo",
    "params": [],
    "id": 4
  }'
```

## Error Handling

The server returns standard JSON-RPC 2.0 error responses:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32601,
    "message": "method not found: invalidMethod"
  },
  "id": 1
}
```

## Response Examples

### Successful Response

```json
{
  "jsonrpc": "2.0",
  "result": {
    "transactionHash": "0x1234abcd...",
    "status": "pending"
  },
  "id": 1
}
```

### Error Response

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "Invalid params"
  },
  "id": 1
}
```
