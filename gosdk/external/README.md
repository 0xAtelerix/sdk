# External Transaction Builder

Unified Go library for creating cross-chain external transactions via TSS appchain architecture.

## Quick Start

```go
import "github.com/0xAtelerix/sdk/gosdk/external"

// Create external transaction
tx, err := external.NewExTxBuilder().
    Ethereum().
    SetPayload(yourEncodedData).
    Build(ctx)
```

## Installation

```bash
go get github.com/0xAtelerix/sdk/gosdk
```

**Note**: Solana support requires custom encoding/decoding (not Ethereum ABI). See examples for implementation patterns.

## TSS Flow

### EVM Chains (Ethereum, BSC, Polygon)
```
[Your App] → [External Tx Builder] → [TSS Appchain] → [Pelagos Contract] → [Your Contract]
     ↓                                                                          ↓
  ABI Encode                                                                ABI Decode
```

### Solana 
```
[Your App] → [External Tx Builder] → [TSS Appchain] → [Solana Program]
     ↓                                                       ↓
Custom Encode                                          Custom Decode
(Borsh/JSON/etc)                                      (Same format)
```

**Universal Process:**
1. **Encode** your data using appropriate format (ABI for EVM, Borsh for Solana, etc.)
2. **Create** external transaction with this builder  
3. **TSS appchain** validates and routes via threshold signatures
4. **Target chain** receives and forwards data to your contract/program
5. **Your code** decodes using the same format you encoded with

## Examples

### EVM Chains (ABI Encoding)
```go
// Ethereum ABI encoding
tx, err := external.NewExTxBuilder().
    Ethereum().
    SetPayload(abiEncodedData).
    Build(ctx)
```

### Solana (Custom Encoding)
```go
// Borsh encoding for Solana
type SolanaTransfer struct {
    To     [32]byte // Pubkey
    Amount uint64   // Lamports  
}
data := borsh.Serialize(SolanaTransfer{...})

tx, err := external.NewExTxBuilder().
    SolanaMainnet().
    SetPayload(data).
    Build(ctx)
```

### Complete Examples
See the [`example/`](./example/) folder for working implementations:

- **`example/main.go`** - EVM transfer and multi-chain swap examples
- **`example/appchain/encoder.go`** - ABI-based encoding package
- **`example/contract/AppchainReceiver.sol`** - Solidity decoding contract

```bash
cd example
go run main.go
```

## Key Features

- ✅ **Unified API** - Single builder for all chains
- ✅ **Type Safety** - Go types with proper validation  
- ✅ **Multi-Chain** - Deploy same payload to multiple chains
- ✅ **ABI Compatible** - Works with Ethereum ABI encoding
- ✅ **TSS Integration** - Seamless threshold signature coordination

## Developer Control

You control both encoding (appchain) and decoding (contract) sides:
- Define your data format (JSON, ABI, custom)
- Use same encoding/decoding logic on both sides
- Complete flexibility in payload design

---

For detailed examples and integration patterns, see the [`example/`](./example/) folder.
