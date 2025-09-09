# External Transaction Builder

This package provides a unified interface for building external transactions in the Pelagos SDK. It supports EVM-compatible chains and Solana with a simple, Account Abstraction-style API.

## Features

✅ **No Signer Dependencies** - TSS handles all signing  
✅ **No Gas Management** - TSS handles gas estimation and fees  
✅ **Cross-Chain Support** - EVM and Solana with unified interface  
✅ **Intent-Based** - Express what you want, not how to do it  
✅ **Chain-Agnostic** - Same API works across all supported chains  
✅ **Unified Intent Structure** - Both EVM and Solana use the same `BaseTxIntent` format

## Intent Structure

All external transactions use a unified intent structure:

```go
type BaseTxIntent struct {
    ChainID uint64 `json:"chainId"` // Target blockchain
    To      string `json:"to"`      // Recipient address (hex for EVM, base58 for Solana)
    Value   string `json:"value"`   // Amount (wei for EVM, lamports for Solana)
    Data    string `json:"data"`    // Transaction data (hex encoded)
}
```

This unified structure ensures consistency across all supported blockchains.  

## Quick Start

### EVM Transactions

```go
import "github.com/0xAtelerix/sdk/gosdk/external"

// Simple ETH transfer
extTx, err := external.NewEVMTxBuilder().
    SetChainID(1). // Ethereum mainnet
    SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
    SetValue(big.NewInt(1000000000000000000)). // 1 ETH in wei
    Build(ctx)
```

## Contract Calls

For contract interactions, you need to handle ABI encoding externally:

```go
// Contract call - use ABI encoding externally
import "github.com/ethereum/go-ethereum/accounts/abi"

// Developer encodes contract call data using ABI
contractABI, _ := abi.JSON(strings.NewReader(contractABIString))
callData, _ := contractABI.Pack("transfer", toAddress, amount)

extTx, err := external.NewEVMTxBuilder().
    SetChainID(137). // Polygon
    SetTo("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"). // USDC contract
    SetValue(big.NewInt(0)).
    SetData(callData). // Pre-encoded ABI data
    Build(ctx)
```

**Note**: The SDK does not provide ABI encoding utilities as these require contract-specific knowledge. Use external libraries like `go-ethereum/accounts/abi` for proper ABI encoding.

### Solana Transactions

```go
// Simple SOL transfer
extTx, err := external.NewSolanaTxBuilder().
    SetChainID(101).
    SetTo("11111111111111111111111111111112").
    SetValue(big.NewInt(1000000000)). // 1 SOL in lamports
    Build(ctx)

// Program instruction
extTx, err := external.NewSolanaTxBuilder().
    SetChainID(101).
    SetTo("program_address").
    SetData([]byte("instruction_data")).
    Build(ctx)
```

## Helper Methods

The SDK provides convenience methods for common operations:

### EVM Helper Methods

```go
// Value conversion helpers
builder.SetValueEther(1.5)              // Converts ether to wei automatically
builder.SetValueWei(big.NewInt(1000))   // Sets wei value directly

// Chain configuration helpers  
builder.Ethereum()                      // Sets ChainID to 1 (Ethereum mainnet)
builder.Polygon()                       // Sets ChainID to 137 (Polygon mainnet)
```

### Solana Helper Methods

```go
// Value conversion helpers
builder.SetValueSOL(0.5)                        // Converts SOL to lamports automatically  
builder.SetValueLamports(big.NewInt(500000000)) // Sets lamports value directly
```

**Note**: Helper methods return the concrete builder type, so use them before calling interface methods for proper method chaining.

### Using Helper Methods

```go
// EVM with convenience methods (use helper methods first for proper chaining)
extTx, err := external.NewEVMTxBuilder().
    Ethereum().                                       // Sets ChainID to 1
    SetValueEther(1.5).                              // Automatically converts to wei
    SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
    Build(ctx)

// Polygon with convenience methods  
extTx, err := external.NewEVMTxBuilder().
    Polygon().                                        // Sets ChainID to 137
    SetValueEther(2.0).
    SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
    Build(ctx)

// Solana with convenience methods (use helper methods first)
extTx, err := external.NewSolanaTxBuilder().
    SetValueSOL(0.5).                                // Automatically converts to lamports
    SetChainID(101).                                  // Solana mainnet
    SetTo("11111111111111111111111111111112").
    Build(ctx)
```

### Chain Type Auto-Detection

```go
chainID := uint64(1) // Ethereum
chainType := external.GetChainType(chainID)
builder := external.NewExternalTxBuilder(chainType)

extTx, err := builder.
    SetChainID(chainID).
    SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
    SetValue(big.NewInt(1000000000000000000)).
    Build(ctx)
```

## Output Format

The builder creates `ExternalTransaction` structs with this format:

```go
type ExternalTransaction struct {
    ChainID uint64 `json:"chainId"`
    Tx      []byte `json:"tx"`      // JSON-encoded intent
}
```

### EVM Intent Format
```json
{
    "chainId": 1,
    "to": "0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264",
    "value": "1000000000000000000",
    "data": "0x"
}
```

### Solana Intent Format
```json
{
    "chainId": 101,
    "to": "11111111111111111111111111111112",
    "value": "1000000000",
    "data": "0x"
}
```

## How It Works

1. **User Creates Intent** - Express what transaction you want
2. **Builder Encodes Intent** - Convert to standardized JSON format
3. **SDK Emits** - ExternalTransaction is emitted by your application
4. **Pelagos TSS Processes** - TSS network handles gas, nonce, signing
5. **L1/L2 Execution** - Transaction is submitted to target chain

## Benefits

- **Simplified UX** - No gas tokens, private keys, or nonce management
- **Cross-Chain** - Same API for all supported blockchains
- **Sponsored Execution** - Pelagos network pays transaction fees
- **Account Abstraction** - Intent-based transactions like EIP-4337
- **Secure** - TSS handles signing, no private key exposure

## Examples

See `examples.go` for complete usage examples including:
- Basic transfers
- Contract calls
- Multi-chain transactions
- Error handling
