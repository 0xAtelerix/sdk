# Atelerix Solana Programs

> **⚠️ IMPORTANT DISCLAIMER**  
> **These programs are for DEMONSTRATION and TESTING purposes only.**  
> They are NOT production-ready and should NOT be used in live environments without:
> - Comprehensive security audits
> - Proper input validation and account checks
> - Production-grade error handling
> - Thorough testing on devnet/testnet
> 
> Use at your own risk. The authors are not responsible for any losses or damages.

Sample Solana programs demonstrating cross-chain transaction routing via Pelagos router.

## 📖 Table of Contents

- [Overview](#overview)
- [🏗️ Flow Details](#flow-details)
- [📁 Programs](#-programs)
- [🚀 Quick Start](#-quick-start)
- [🔧 Integration Guide](#-integration-guide)
- [⚠️ Common Pitfalls](#️-common-pitfalls--extensions)
- [🎓 Advanced Topics](#-advanced-topics)
- [📚 Resources](#-resources)

## Overview

This directory contains working examples of the **Pelagos** router pattern for cross-chain transactions on Solana. Pelagos acts as a universal entry point that forwards transactions to specialized appchain programs via Cross-Program Invocation (CPI).

**Key Principle**: Pelagos program is **payload-agnostic**—it doesn't parse or validate transaction data, only routes it to the correct appchain program.

### 📁 Directory Structure

```
solana-programs/
├── pelagos/
│   └── pelagos.rs              # Main Pelagos router program
├── appchain-tokenmint/
│   └── appchain-tokenmint.rs   # Example appchain for token minting
└── README.md                   # This file
```

## Flow Details

1. **Appchain (Source Chain)**
   - Build external transaction using SDK
   - Payload includes: appchain program ID + specific accounts + instruction data
   - Emit as `ExternalTransaction`

2. **pelacli (Relayer)**
   - Decodes external transaction payload
   - Extracts accounts and payload
   - Builds Solana transaction and prepends payer to account list
   - Submits to Pelagos on Solana

3. **Pelagos (Router)**
   - Receives transaction with opaque payload
   - Validates payer signature
   - Forwards via CPI to target appchain program
   - Does NOT parse instruction data

4. **AppChain Program (Target)**
   - Receives CPI from Pelagos
   - Note: `accounts[0]` is the program itself (check programs for details)
   - Parses instruction data according to its format
   - Executes business logic (mint tokens, transfer, etc.)

## 📁 Programs

### 1. Pelagos (Router)
**File**: `pelagos/pelagos.rs`

Universal router that forwards transactions to any appchain program.

**Account Layout**:
- `[0]` Payer (signer, writable)
- `[1]` Target appchain program (executable)
- `[2..N]` Appchain-specific accounts

**Instruction Data**: Opaque bytes (forwarded as-is)

### 2. AppChain Token Mint
**File**: `appchain-tokenmint/appchain-tokenmint.rs`

Example appchain that mints SPL tokens using a PDA as mint authority.

**Account Layout** (in CPI context):
- `[0]` Program itself (auto-included)
- `[1]` Mint account (writable)
- `[2]` Recipient token account / ATA (writable)
- `[3]` Mint authority PDA
- `[4]` SPL Token program

**Instruction Data**: `[recipient:32][amount:8]` (40 bytes total)

**PDA Derivation**: `Pubkey::find_program_address(&[b"mint_authority"], program_id)`

**Why This Works**: Opaque payload (devs control format); accounts pre-derived off-chain (e.g., ATA); Pelagos stays dumb/simple.

## 🚀 Quick Start

### Prerequisites

- [Solana Playground](https://beta.solpg.io/) (easiest, no installation needed)
- Or [Solana CLI](https://docs.solana.com/cli/install-solana-cli-tools) for local development

### Deploy Programs

**Note:** These are example programs for learning and testing. The easiest way to deploy and test is using [Solana Playground](https://beta.solpg.io/).

**Using Solana Playground:**
1. Visit [beta.solpg.io](https://beta.solpg.io/)
2. Create a new project or import the program files
3. Copy `pelagos.rs` or `appchain-tokenmint.rs` code
4. Click "Build" to compile
5. Click "Deploy" to deploy to devnet
6. Copy the deployed Program ID for use in your appchain

**Note:** Unlike EVM contracts, Solana programs don't have a registration step. The appchain program ID is used directly in the external transaction payload from your appchain.

### Test Your Program

After deploying via Solana Playground, you can test directly in the browser or use the deployed Program ID in your appchain's external transactions.

From your appchain's Go code:

```go
import (
    "github.com/blocto/solana-go-sdk/common"
    "github.com/blocto/solana-go-sdk/types"
    "github.com/0xAtelerix/sdk/gosdk/external"
)

// In Transaction.Process() OR StateTransition.ProcessBlock()
func (tx *MyTx) Process(...) (receipt, []apptypes.ExternalTransaction, error) {
    // Build appchain accounts
    appchainProg := types.AccountMeta{
        PubKey:     common.PublicKeyFromString("<APPCHAIN_PROGRAM_ID>"),
        IsSigner:   false,
        IsWritable: false,
    }
    
    specifics := []types.AccountMeta{
        {PubKey: mintPubkey, IsSigner: false, IsWritable: true},
        {PubKey: recipientATA, IsSigner: false, IsWritable: true},
        {PubKey: mintAuthorityPDA, IsSigner: false, IsWritable: false},
        {PubKey: tokenProgramID, IsSigner: false, IsWritable: false},
    }
    
    // Build instruction data: [recipient:32][amount:8]
    data := make([]byte, 40)
    copy(data[0:32], recipientPubkey[:])
    binary.LittleEndian.PutUint64(data[32:40], amount)
    
    // Create Solana payload
    fullAccounts := append([]types.AccountMeta{appchainProg}, specifics...)
    etx, _ := external.NewExTxBuilder(data, gosdk.SolanaDevnetChainID).
        AddSolanaAccounts(fullAccounts).
        Build()
    
    return receipt, []apptypes.ExternalTransaction{etx}, nil
}
```

### Verify on Explorer

```bash
# After pelacli submits transaction
solana confirm -v <TRANSACTION_SIGNATURE>

# Check logs for:
# "🔄 Pelagos: Forwarding to appchain <APPCHAIN_PROGRAM_ID>"
# "✅ Pelagos: CPI completed"
# "✅ Appchain: Minted <AMOUNT> tokens to <RECIPIENT>"
```

## 🔧 Integration Guide

### For Appchain Developers

#### 1. Define Your Operation

Decide what your appchain program will do:
- Token mint (example provided)
- Token transfer
- Token burn
- Data logging
- NFT operations
- Custom logic

#### 2. Design Account Layout

Define which accounts your program needs:

```rust
// Example: Token transfer
// [0] = program (auto)
// [1] = source token account (writable)
// [2] = destination token account (writable)
// [3] = authority (signer)
// [4] = token program
```

#### 3. Define Instruction Data Format

Choose your payload structure:

```rust
// Example: Token transfer
// [recipient: 32 bytes][amount: 8 bytes] = 40 bytes total
```

**Pro Tip**: For complex data, use Borsh serialization:
```rust
#[derive(BorshSerialize, BorshDeserialize)]
struct TransferParams {
    recipient: Pubkey,
    amount: u64,
    memo: String,
}
```

#### 4. Implement Program Logic

```rust
pub fn process_instruction(
    program_id: &Pubkey,
    accounts: &[AccountInfo],
    instruction_data: &[u8],
) -> ProgramResult {
    // 1. Validate inputs
    // 2. Parse instruction data
    // 3. Get accounts (remember: [0] is program in CPI)
    // 4. Validate accounts
    // 5. Execute logic
    // 6. Log results
    Ok(())
}
```

#### 5. Build Go SDK Integration

In your appchain's `Transaction.Process()`:

```go
// Build accounts
accounts := []types.AccountMeta{
    {PubKey: yourAppchainProgram, ...},
    // ... your specific accounts
}

// Build data
data := buildYourPayload(...)

// Create payload
etx, _ := external.NewExTxBuilder(data, gosdk.SolanaDevnetChainID).
    AddSolanaAccounts(accounts).
    Build()

return receipt, []apptypes.ExternalTransaction{etx}, nil
```

## ⚠️ Common Pitfalls & Extensions

### Pitfalls
1. **Account Order**: Tx keys must match handler indices exactly—devs forget to prepend appchain_prog in SDK.
2. **PDA Mismatch**: Seeds must be identical (e.g., `b"mint_authority"`). Log PDAs in tests.
3. **ATA Non-Existent**: pelacli should pre-check/create via RPC (add in `ProcessExternalTransactions`).
4. **Account Indexing Off-By-One**: `accounts[0]` is program itself in CPI—always skip it in appchain handlers.
5. **Writable/Signer Flags**: Set correct flags in `AccountMeta`:
   - `IsWritable: true` for accounts that will be modified
   - `IsSigner: true` for accounts that must sign (usually just payer)

### Extensions
1. **Multi-Instruction**: Add 8-byte discriminator for multiple operations (prepend to data).
2. **Error Propagation**: Catch `invoke` errors and log for better debugging.
3. **Complex Data**: Use Borsh serialization for structured payloads.
4. **Testing**: Use `solana-test-validator` for local testing before devnet deployment.

## 🎓 Advanced Topics

### Multi-Instruction Support

Add an 8-byte discriminator for multiple operations:

```rust
const MINT_DISCRIMINATOR: [u8; 8] = [0x01, 0, 0, 0, 0, 0, 0, 0];
const BURN_DISCRIMINATOR: [u8; 8] = [0x02, 0, 0, 0, 0, 0, 0, 0];

pub fn process_instruction(...) -> ProgramResult {
    let discriminator = &instruction_data[0..8];
    match discriminator {
        MINT_DISCRIMINATOR => handle_mint(&instruction_data[8..], accounts),
        BURN_DISCRIMINATOR => handle_burn(&instruction_data[8..], accounts),
        _ => Err(ProgramError::InvalidInstructionData),
    }
}
```

In Go, prepend discriminator:
```go
data := make([]byte, 48) // 8 (disc) + 32 (recipient) + 8 (amount)
copy(data[0:8], MINT_DISCRIMINATOR)
copy(data[8:40], recipient[:])
binary.LittleEndian.PutUint64(data[40:48], amount)
```

### Error Handling

Log detailed errors for debugging:

```rust
if !payer.is_signer {
    msg!("❌ Error: Payer must be signer");
    msg!("   Payer: {}", payer.key);
    msg!("   Is signer: {}", payer.is_signer);
    return Err(ProgramError::MissingRequiredSignature);
}
```

### Testing

Use `solana-program-test` for integration tests:

```rust
#[cfg(test)]
mod tests {
    use solana_program_test::*;
    use solana_sdk::{signature::Signer, transaction::Transaction};

    #[tokio::test]
    async fn test_mint() {
        let program_id = Pubkey::new_unique();
        let mut program_test = ProgramTest::new(
            "appchain_tokenmint",
            program_id,
            processor!(process_instruction),
        );
        
        let (mut banks_client, payer, recent_blockhash) = program_test.start().await;
        
        // Build and send transaction
        let mut transaction = Transaction::new_with_payer(
            &[/* your instructions */],
            Some(&payer.pubkey()),
        );
        transaction.sign(&[&payer], recent_blockhash);
        banks_client.process_transaction(transaction).await.unwrap();
    }
}
```

## 🔒 Security Considerations

⚠️ **These are example programs for learning purposes. For production:**

1. **Input Validation**
   - Validate all payload sizes
   - Check account ownership
   - Verify writable/signer flags
   - Validate token program IDs

2. **Access Control**
   - Implement proper authority checks
   - Use PDAs for program-controlled accounts
   - Validate all signers

3. **Numeric Safety**
   - Check for overflows/underflows
   - Validate amount bounds
   - Use `checked_add`, `checked_sub`, etc.

4. **Audit**
   - Get professional security audit
   - Test extensively on devnet/testnet
   - Have emergency pause mechanism

## 📚 Resources

- **Solana Docs**: https://docs.solana.com/
- **SPL Token**: https://spl.solana.com/token
- **Solana Cookbook**: https://solanacookbook.com/
- **Atelerix SDK**: `../gosdk/` for Go integration

**Simple. Universal. Extensible.**

The Pelagos pattern enables any appchain to interact with Solana using CPI routing, without modifying the core router. Just deploy your appchain program and route through Pelagos!
