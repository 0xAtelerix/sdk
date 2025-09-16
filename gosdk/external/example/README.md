# External Transaction Example

This example demonstrates realistic external transaction usage with actual ABI encoding/decoding.

## Overview

```
[Your Appchain] → [External Tx Builder] → [TSS Appchain] → [Pelagos Contract] → [Your Contract]
     ↓                                                                               ↓
   Encode                                                                          Decode
```

## Files

### `main.go` - EVM Examples
- **TransferExample**: Token transfer with ABI encoding
- **SwapExample**: Multi-chain DeFi swap deployment
- Uses actual contract-compatible encoding

### `solana_main.go` - Solana Examples
- **SolanaExample**: Solana program integration with JSON encoding
- **MultiChainExample**: Deploy to both EVM and Solana chains
- Shows different encoding formats per chain

### `appchain/encoder.go` - ABI Encoder Package
- Type-safe ABI encoding/decoding
- Matches `AppchainReceiver.sol` contract
- Production-ready implementation

### `contract/AppchainReceiver.sol` - Contract Side
- Solidity contract that receives data from Pelagos
- Decodes using same ABI format as encoder
- Handles transfer and swap operations

## Running the Example

**EVM Examples:**
```bash
cd example
go run main.go
```

**Solana Examples:**
```bash
cd example  
go run solana_main.go
```

**Output Examples:**
```
=== External Transaction Examples ===

1. Token Transfer:
External Transaction Created: ChainID=1, Size=100 bytes
Decoded Transfer: 0xa0b86A33e6441c9fa6e8EE5B1234567890aBCdef → 0x00000000742d35Cc6493c35b1234567890aBcDef (1000000000000000000 wei)

2. Multi-Chain Swap:
Deploying swap to 3 chains:
✓ Ethereum (Chain 1) - 196 bytes
✓ BSC (Chain 56) - 196 bytes  
✓ Polygon (Chain 137) - 196 bytes
```

**Solana Output:**
```
=== Solana External Transaction Example ===
Solana External Transaction Created:
  Chain ID: 900 (Solana Mainnet)
  Payload Size: 127 bytes
Decoded Solana Transfer:
  To: 9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM
  Amount: 1000000000 lamports (1.00 SOL)
```

## Key Features

### 1. Multi-Chain Support
Works with both EVM and non-EVM chains:

**EVM Chains (ABI Encoding):**
```go
// Ethereum ABI encoding
transferData := appchain.TransferData{
    To:     common.HexToAddress("0x742d35Cc6493C35b1234567890abcdef"),
    Amount: big.NewInt(1000000000000000000), // 1 ETH
    Token:  common.HexToAddress("0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef"),
}
payloadBytes, _ := encoder.EncodeTransfer(transferData)
```

**Solana (JSON/Borsh Encoding):**
```go
// Solana JSON encoding
transferData := SolanaTransfer{
    To:     "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM", // Base58 address
    Amount: 1000000000,                                      // Lamports
    Token:  "So11111111111111111111111111111111111111112",   // Token mint
}
payloadBytes, _ := json.Marshal(transferData)
```

### 2. Same External Transaction Builder
```go
// EVM chains
tx, _ := external.NewExTxBuilder().Ethereum().SetPayload(abiData).Build(ctx)

// Solana  
tx, _ := external.NewExTxBuilder().SolanaMainnet().SetPayload(jsonData).Build(ctx)
```

### 3. TSS Transport Layer
- **TSS appchain** handles routing to all chains
- **Same security model** for EVM and Solana
- **Format agnostic** - transports any byte payload

## Integration Guide

1. **Define Your Operations**: Create structs for your data types
2. **Implement ABI**: Use same function signatures in Go encoder and Solidity contract  
3. **Test Encoding**: Verify your payload can be decoded correctly
4. **Deploy**: Use external transaction builder for cross-chain deployment

## What This Demonstrates

- ✅ **Realistic Usage**: Production-style code without tutorial verbosity
- ✅ **ABI Consistency**: Same encoding format on both sides  
- ✅ **Multi-Chain**: Deploy once, execute on multiple chains
- ✅ **Type Safety**: Go types map directly to Solidity types
- ✅ **Minimal Output**: Shows only essential information

This example represents how you'd actually use the external transaction builder in a real appchain project.
