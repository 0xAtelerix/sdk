# Atelerix External Transaction Builder

A unified Go library for creating cross-chain external transactions. Simple API for Ethereum, Polygon, BSC, Solana, and custom appchains.

## 🚀 Quick Start

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    
    "github.com/atelerix/sdk/gosdk/external"
)

func main() {
    ctx := context.Background()
    
    // Create transaction payload (your appchain defines the format)
    payload := map[string]interface{}{
        "to":    "0x742d35Cc6493C35b1234567890abcdef",
        "value": "1000000000000000000", // 1 ETH
    }
    
    payloadBytes, _ := json.Marshal(payload)
    
    // Build external transaction
    tx, err := external.NewExTxBuilder().
        Ethereum().                    // Sets chainID to 1
        SetPayload(payloadBytes).
        Build(ctx)
    
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("External transaction created for ChainID: %d", tx.ChainID)
    
    // TSS Appchain Flow:
    // 1. TSS appchain receives this external transaction
    // 2. TSS appchain validates and processes the transaction  
    // 3. TSS appchain submits to Pelagos contract on Ethereum
    // 4. Pelagos contract forwards data to your appchain contract
    // 5. Your appchain contract decodes and processes the transfer
}
```

## � Installation

```bash
go get github.com/atelerix/sdk/gosdk
```

## 🔧 API Overview

### Basic Usage
```go
// Single builder for all chains
builder := external.NewExTxBuilder()

// Chain helpers (recommended)
tx, err := builder.Ethereum().SetPayload(payload).Build(ctx)        // ChainID: 1
tx, err := builder.Polygon().SetPayload(payload).Build(ctx)          // ChainID: 137
tx, err := builder.SolanaMainnet().SetPayload(payload).Build(ctx)    // ChainID: 900

// Custom chains
tx, err := builder.SetChainID(999999).SetPayload(payload).Build(ctx)
```

## 🔄 TSS Appchain Transaction Flow

This builder creates external transactions that flow through a TSS (Threshold Signature Scheme) appchain architecture:

```go
// 1. Create external transaction with this builder
tx, err := external.NewExTxBuilder().
    Ethereum().
    SetPayload(yourPayload).
    Build(ctx)

// 2. TSS Appchain receives the external transaction
// 3. TSS Appchain validates and processes the transaction
// 4. TSS Appchain submits transaction to Pelagos contract on target chain
// 5. Pelagos contract decodes the payload and forwards data to your appchain contract
// 6. Your appchain contract processes the data using the same encoding format
```

### Architecture Overview

```
[Your App] → [External Tx Builder] → [TSS Appchain] → [Pelagos Contract] → [Appchain Contract]
```

### Detailed Flow

1. **External Transaction Creation**: Your application uses this builder to create standardized external transactions with chain-specific payloads

2. **TSS Appchain Processing**: The TSS appchain receives external transactions, validates them, and coordinates cross-chain operations using threshold signatures

3. **Pelagos Contract Interaction**: TSS appchain submits transactions to the Pelagos contract deployed on the target chain (Ethereum, BSC, etc.)

4. **Payload Decoding & Execution**: Pelagos contract decodes the payload and forwards the data to your appchain contract

5. **Appchain Contract Processing**: Your appchain contract receives the decoded data and executes the intended logic (this is why you encode the payload - so your contract can decode and process it)

### Benefits of TSS Architecture

- **Security**: Threshold signatures provide robust multi-party security
- **Interoperability**: Seamless cross-chain operations through Pelagos contracts
- **Flexibility**: Payload-based approach allows arbitrary operation encoding
- **Reliability**: TSS consensus ensures transaction integrity and finality

### What This Builder Does
- ✅ Creates standardized external transactions
- ✅ Manages chain IDs for TSS routing
- ✅ Transports your encoded payloads across chains
- ✅ Provides type safety and validation

### What TSS Appchain Does
- 🔧 Validates external transactions
- 🔧 Coordinates threshold signatures
- 🔧 Submits transactions to Pelagos contracts

### What Pelagos Contract Does
- ⚙️ Receives transactions from TSS appchain
- ⚙️ Forwards encoded data to your appchain contract
- ⚙️ Handles the cross-chain communication protocol

### What Your Appchain Contract Does
- 🏗️ Receives encoded data from Pelagos contract
- 🏗️ Decodes data using the same ABI you used to encode it
- 🏗️ Executes your custom business logic
- 🏗️ Updates contract state based on the operation

### Developer Control & Flexibility
Since you control **both the appchain side** (where you encode) and **the contract side** (where you decode), you have complete flexibility:
- Define your own data encoding scheme
- Use the same ABI on both appchain and contract sides  
- Structure data flow according to your specific needs
- Maintain consistency between encoding and decoding logic

## 💡 Examples

### Multi-Chain DeFi via TSS
```go
// Same DeFi operation across multiple chains via TSS appchain
swapPayload := map[string]interface{}{
    "action": "swap",
    "tokenIn": "0xTokenA...",
    "tokenOut": "0xTokenB...", 
    "amount": "1000000000000000000", // 1 token
    "slippage": "50", // 0.5%
}
payloadBytes, _ := json.Marshal(swapPayload)

// TSS appchain will route to Pelagos contracts on each chain
chains := []func() *external.ExTxBuilder{
    external.NewExTxBuilder().Ethereum,   // → Pelagos on Ethereum
    external.NewExTxBuilder().BSC,        // → Pelagos on BSC  
    external.NewExTxBuilder().Polygon,    // → Pelagos on Polygon
}

for _, chain := range chains {
    tx, _ := chain().SetPayload(payloadBytes).Build(ctx)
    // TSS appchain processes → Pelagos contract executes swap
}
```

## ⚠️ Error Handling

```go
tx, err := external.NewExTxBuilder().
    SetPayload(payload).  // Missing chain!
    Build(ctx)

if err != nil {
    log.Fatal("Error:", err)  // "chainID must be set"
}
```
