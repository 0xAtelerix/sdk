# External Transaction Builder

Unified Go library for creating cross-chain external transactions via TSS appchain architecture.

## Quick Start

```go
import (
    "github.com/0xAtelerix/sdk/gosdk/external"
    gosdk "github.com/0xAtelerix/sdk/gosdk"
)

// EVM transaction
tx, err := external.NewExTxBuilder(yourEncodedData, gosdk.EthereumChainID).Build()

// Solana transaction with accounts
builder := external.NewExTxBuilder(jsonData, gosdk.SolanaChainID)
builder.AddSolanaAccounts(accounts)
solanaTx, err := builder.BuildSolanaPayload()
```

**Universal Process:**
1. **Encode** your data using appropriate format
2. **Create** external transaction with this builder
3. **TSS appchain** validates and routes via threshold signatures
4. **Target chain** receives and forwards data to your contract/program
5. **Your code** decodes using the same format you encoded with

## Examples

### EVM Chains (ABI Encoding)
```go
// Ethereum ABI encoding
tx, err := external.NewExTxBuilder(abiEncodedData, gosdk.EthereumChainID).Build()
if err != nil {
    log.Fatal(err)
}
```

### Solana (Account-Based Encoding)
```go
import (
    "github.com/blocto/solana-go-sdk/common"
    "github.com/blocto/solana-go-sdk/types"
)

// Solana with accounts and custom encoding
accounts := []types.AccountMeta{
    {PubKey: common.PublicKeyFromString("11111111111111111111111111111112"), IsSigner: false, IsWritable: false},
    {PubKey: common.PublicKeyFromString("recipient_address"), IsSigner: false, IsWritable: true},
    {PubKey: common.PublicKeyFromString("sender_address"), IsSigner: true, IsWritable: true},
}

builder := external.NewExTxBuilder(jsonData, gosdk.SolanaChainID)
builder.AddSolanaAccounts(accounts)
tx, err := builder.BuildSolanaPayload()
if err != nil {
    log.Fatal(err)
}
```

## Key Features

- ✅ **Unified API** - Single builder for all chains
- ✅ **Type Safety** - Go types with proper validation  
- ✅ **Multi-Chain** - Deploy same payload to multiple chains
- ✅ **ABI Compatible** - Works with Ethereum ABI encoding
- ✅ **Solana Accounts** - Full account metadata support for Solana transactions
- ✅ **TSS Integration** - Seamless threshold signature coordination

## Developer Control

You control both encoding (appchain) and decoding (contract) sides:
- Define your data format (JSON, ABI, custom)
- Use same encoding/decoding logic on both sides
- Complete flexibility in payload design
