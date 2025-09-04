# RPC Package

The RPC package provides a JSON-RPC 2.0 server for Atelerix appchains, allowing external clients to interact with your blockchain through HTTP requests.

## Overview

The RPC server is automatically integrated into the appchain framework and starts when you call `appchain.Run()`. It provides standard blockchain methods out of the box and allows you to add custom methods specific to your application.

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/0xAtelerix/sdk/gosdk"
    // your other imports...
)

func main() {
    // Initialize your appchain components (state transition, block builder, txpool, config, database, etc.)
    // ... (initialization details depend on your specific appchain implementation)
    
    // Create appchain - this automatically includes RPC server
    appchain, err := gosdk.NewAppchain(
        stateTransition,
        blockBuilder, 
        txPool,
        config,        // includes RPC port configuration
        appchainDB,
        // optional parameters...
    )
    if err != nil {
        panic(err)
    }
    
    // Start both appchain and RPC server
    appchain.Run() // RPC server starts automatically
}
```

### Making RPC Calls

Once your appchain is running, you can make JSON-RPC 2.0 calls to `http://localhost:8080/rpc`:

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "sendTransaction",
    "params": ["0x1234..."],
    "id": 1
  }'
```

## Standard Methods

The RPC server provides these built-in methods:

### `sendTransaction`
Submit a new transaction to the transaction pool.

**Parameters:**
- `txData` (string): Hex-encoded transaction data

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "sendTransaction",
  "params": ["0x1234567890abcdef"],
  "id": 1
}
```

### `getTransactionByHash`
Retrieve a transaction by its hash.

**Parameters:**
- `hash` (string): Transaction hash

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "getTransactionByHash",
  "params": ["0xabcdef1234567890"],
  "id": 1
}
```

### `getTransactionStatus`
Get the status of a transaction.

**Parameters:**
- `hash` (string): Transaction hash

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "getTransactionStatus",
  "params": ["0xabcdef1234567890"],
  "id": 1
}
```

### `getPendingTransactions`
Get all pending transactions in the pool.

**Parameters:** None

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "getPendingTransactions",
  "params": [],
  "id": 1
}
```

### `getTransactionReceipt`
Get the receipt for a transaction.

**Parameters:**
- `hash` (string): Transaction hash

**Example:**
```json
{
  "jsonrpc": "2.0",
  "method": "getTransactionReceipt",
  "params": ["0xabcdef1234567890"],
  "id": 1
}
```

## Adding Custom Methods

You can extend the RPC server with custom methods specific to your appchain:

### Simple Custom Method

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/0xAtelerix/sdk/gosdk"
    "github.com/0xAtelerix/sdk/gosdk/rpc"
)

func main() {
    // Create appchain (with all required parameters)
    appchain, err := gosdk.NewAppchain(
        stateTransition,
        blockBuilder,
        txPool,
        config,
        appchainDB,
        // optional parameters...
    )
    if err != nil {
        panic(err)
    }
    
    // Get RPC server to add custom methods
    rpcServer := appchain.GetRPCServer()
    
    // Add a simple custom method
    rpcServer.AddCustomMethod("getChainName", func(ctx context.Context, params []any) (any, error) {
        return map[string]string{
            "name": "MyAppchain",
            "version": "1.0.0",
        }, nil
    })
    
    // Start appchain (RPC server starts automatically)
    appchain.Run()
}
```

### Custom Method with Parameters

```go
// Add a method that takes parameters
rpcServer.AddCustomMethod("getAccountBalance", func(ctx context.Context, params []any) (any, error) {
    if len(params) != 1 {
        return nil, fmt.Errorf("expected 1 parameter, got %d", len(params))
    }
    
    address, ok := params[0].(string)
    if !ok {
        return nil, fmt.Errorf("address parameter must be string")
    }
    
    // Your custom logic here
    balance := getBalanceFromDatabase(address)
    
    return map[string]any{
        "address": address,
        "balance": balance,
    }, nil
})
```

### Complex Custom Method with Database Access

```go
// Method that interacts with your appchain's database
rpcServer.AddCustomMethod("getCustomData", func(ctx context.Context, params []any) (any, error) {
    if len(params) < 1 {
        return nil, fmt.Errorf("missing required parameter 'key'")
    }
    
    key, ok := params[0].(string)
    if !ok {
        return nil, fmt.Errorf("key parameter must be string")
    }
    
    var block uint64
    if len(params) > 1 {
        if blockFloat, ok := params[1].(float64); ok {
            block = uint64(blockFloat)
        }
    }
    
    // Access your database through the appchain
    data, err := yourCustomDatabaseQuery(key, block)
    if err != nil {
        return nil, fmt.Errorf("database query failed: %w", err)
    }
    
    return data, nil
})
```

### Method with Error Handling

```go
rpcServer.AddCustomMethod("validateTransaction", func(ctx context.Context, params []any) (any, error) {
    if len(params) == 0 {
        return nil, &rpc.Error{
            Code:    -32602,
            Message: "Missing transaction data",
        }
    }
    
    txData, ok := params[0].(string)
    if !ok {
        return nil, &rpc.Error{
            Code:    -32602, // Invalid params
            Message: "Transaction data must be string",
        }
    }
    
    isValid, reason := validateTransactionData(txData)
    
    if !isValid {
        return nil, &rpc.Error{
            Code:    -32000, // Custom application error
            Message: "Transaction validation failed",
            Data:    reason,
        }
    }
    
    return map[string]bool{"valid": true}, nil
})
```

## Complete Example

Here's a complete example showing how to create an appchain with custom RPC methods:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    
    "github.com/0xAtelerix/sdk/gosdk"
    "github.com/0xAtelerix/sdk/gosdk/rpc"
)

func main() {
    // Initialize your appchain components
    // ... (setup your state transition, block builder, txpool, config, database, etc.)
    
    // Create appchain - RPC configuration is part of the config
    appchain, err := gosdk.NewAppchain(
        stateTransition,
        blockBuilder,
        txPool,
        config,        // config includes RPC port
        appchainDB,
        // optional parameters...
    )
    if err != nil {
        log.Fatal("Failed to create appchain:", err)
    }
    
    // Get RPC server to add custom methods
    rpcServer := appchain.GetRPCServer()
    
    // Add custom methods
    addCustomMethods(rpcServer)
    
    log.Println("Starting appchain with RPC server...")
    
    // Start appchain (includes RPC server)
    appchain.Run()
}

func addCustomMethods(rpcServer *rpc.StandardRPCServer) {
    // Chain information
    rpcServer.AddCustomMethod("getChainInfo", func(ctx context.Context, params []any) (any, error) {
        return map[string]any{
            "chainId":     1,
            "chainName":   "MyAppchain",
            "version":     "1.0.0",
            "blockHeight": db.GetLatestBlockHeight(),
        }, nil
    })
    
    // Account balance
    rpcServer.AddCustomMethod("getBalance", func(ctx context.Context, params []any) (any, error) {
        if len(params) != 1 {
            return nil, &rpc.Error{
                Code:    -32602,
                Message: "Expected 1 parameter (address)",
            }
        }
        
        address, ok := params[0].(string)
        if !ok {
            return nil, &rpc.Error{
                Code:    -32602,
                Message: "Address parameter must be string",
            }
        }
        
        balance, err := db.GetAccountBalance(address)
        if err != nil {
            return nil, &rpc.Error{
                Code:    -32000,
                Message: "Failed to get balance",
                Data:    err.Error(),
            }
        }
        
        return map[string]any{
            "address": address,
            "balance": balance,
        }, nil
    })
    
    // Custom transaction type
    rpcServer.AddCustomMethod("sendCustomTransaction", func(ctx context.Context, params []any) (any, error) {
        if len(params) != 1 {
            return nil, &rpc.Error{
                Code:    -32602,
                Message: "Expected 1 parameter (transaction object)",
            }
        }
        
        // Parse transaction object from params
        txMap, ok := params[0].(map[string]any)
        if !ok {
            return nil, &rpc.Error{
                Code:    -32602,
                Message: "Invalid transaction format",
            }
        }
        
        // Create and validate your custom transaction
        tx, err := createCustomTransactionFromMap(txMap)
        if err != nil {
            return nil, &rpc.Error{
                Code:    -32000,
                Message: "Failed to create transaction",
                Data:    err.Error(),
            }
        }
        
        // Submit to transaction pool
        hash, err := submitTransaction(tx)
        if err != nil {
            return nil, &rpc.Error{
                Code:    -32000,
                Message: "Failed to submit transaction",
                Data:    err.Error(),
            }
        }
        
        return map[string]string{
            "hash": hash,
        }, nil
    })
}
```

## Testing Your RPC Methods

You can test your custom methods using curl:

```bash
# Test getChainInfo
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getChainInfo",
    "params": [],
    "id": 1
  }'

# Test getBalance
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getBalance",
    "params": ["0x1234567890abcdef"],
    "id": 1
  }'
```

## Error Handling

The RPC server supports standard JSON-RPC 2.0 error codes:

- `-32700`: Parse error
- `-32600`: Invalid Request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `-32000` to `-32099`: Custom application errors

## Configuration

The RPC server configuration is part of the appchain config:

```go
// RPC port is configured through the AppchainConfig
config := gosdk.AppchainConfig{
    RPCPort: "8080",  // or your desired port
    // other config options...
}

appchain, err := gosdk.NewAppchain(
    stateTransition,
    blockBuilder,
    txPool,
    config,
    appchainDB,
)

// The server will be available at http://localhost:8080/rpc
```

## Security Considerations

- The RPC server accepts all connections by default
- Validate all input parameters in custom methods
- Use proper error handling to avoid information leakage
- Consider rate limiting for production deployments

## Best Practices

1. **Parameter Validation**: Always validate parameters in custom methods
2. **Error Handling**: Use appropriate RPC error codes
3. **Documentation**: Document your custom methods for API consumers
4. **Testing**: Write tests for your custom RPC methods
5. **Logging**: Add logging for debugging and monitoring
6. **Security**: Validate permissions and sanitize inputs

## Need Help?

- Check the existing RPC tests in `rpc_test.go` for more examples
- Review the standard method implementations in `rpc.go`
- Refer to the JSON-RPC 2.0 specification for protocol details
