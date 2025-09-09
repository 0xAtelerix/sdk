package external

import (
	"context"
	"fmt"
	"math/big"
)

// ExampleEVMTransfer demonstrates how to create a simple EVM transfer
func ExampleEVMTransfer() {
	ctx := context.Background()

	// Simple ETH transfer - use wei directly
	weiValue := new(big.Int).Mul(big.NewInt(15), big.NewInt(1e17)) // 1.5 ETH in wei

	extTx, err := NewEVMTxBuilder().
		SetChainID(1). // Ethereum mainnet
		SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
		SetValue(weiValue).
		Build(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Println("EVM Transaction Created:")
	fmt.Printf("ChainID: %d\n", extTx.ChainID)
	fmt.Printf("Tx Data: %s\n", string(extTx.Tx))
}

// ExampleEVMContractCall demonstrates how to create an EVM contract call
func ExampleEVMContractCall() {
	ctx := context.Background()

	// Contract call - developer must encode ABI data externally
	// Example using go-ethereum's ABI package:
	// import "github.com/ethereum/go-ethereum/accounts/abi"
	// contractABI, _ := abi.JSON(strings.NewReader(contractABIString))
	// callData, _ := contractABI.Pack("transfer", toAddress, amount)

	contractData := []byte("encoded_contract_call_data") // Pre-encoded ABI data

	extTx, err := NewEVMTxBuilder().
		SetChainID(137).                                     // Polygon
		SetTo("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"). // USDC contract
		SetValue(big.NewInt(0)).                             // No ETH value
		SetData(contractData).
		Build(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Println("EVM Contract Call Created:")
	fmt.Printf("ChainID: %d\n", extTx.ChainID)
	fmt.Printf("Tx Data: %s\n", string(extTx.Tx))
}

// ExampleSolanaTransfer demonstrates how to create a Solana transfer
func ExampleSolanaTransfer() {
	ctx := context.Background()

	// Simple SOL transfer - use lamports directly
	lamportsValue := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e8)) // 0.5 SOL in lamports

	extTx, err := NewSolanaTxBuilder().
		SetChainID(101).                           // Solana mainnet
		SetTo("11111111111111111111111111111112"). // System program
		SetValue(lamportsValue).
		Build(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Println("Solana Transaction Created:")
	fmt.Printf("ChainID: %d\n", extTx.ChainID)
	fmt.Printf("Tx Data: %s\n", string(extTx.Tx))
}

// ExampleChainTypeDetection demonstrates automatic chain type detection
func ExampleChainTypeDetection() {
	ctx := context.Background()

	// Auto-detect chain type and create appropriate builder
	chainID := uint64(1) // Ethereum
	chainType := GetChainType(chainID)

	builder := NewExternalTxBuilder(chainType)

	extTx, err := builder.
		SetChainID(chainID).
		SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
		SetValue(big.NewInt(1000000000000000000)). // 1 ETH in wei
		Build(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	fmt.Printf("Auto-detected Chain Type: %d\n", chainType)
	fmt.Println("Transaction Created:")
	fmt.Printf("ChainID: %d\n", extTx.ChainID)
	fmt.Printf("Tx Data: %s\n", string(extTx.Tx))
}

// ExampleMultipleChains demonstrates creating transactions for different chains
func ExampleMultipleChains() {
	ctx := context.Background()

	// Create transactions for multiple chains
	chains := []struct {
		name    string
		chainID uint64
		to      string
		value   *big.Int
	}{
		{
			"Ethereum", 1, "0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264",
			big.NewInt(1000000000000000000),
		},
		{
			"Polygon", 137, "0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264",
			big.NewInt(2000000000000000000),
		},
		{
			"Arbitrum", 42161, "0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264",
			big.NewInt(500000000000000000),
		},
		{
			"Solana", 101, "11111111111111111111111111111112",
			big.NewInt(1000000000),
		}, // 1 SOL in lamports
	}

	for _, chain := range chains {
		chainType := GetChainType(chain.chainID)
		builder := NewExternalTxBuilder(chainType)

		extTx, err := builder.
			SetChainID(chain.chainID).
			SetTo(chain.to).
			SetValue(chain.value).
			Build(ctx)
		if err != nil {
			fmt.Printf("Error creating %s transaction: %v\n", chain.name, err)

			continue
		}

		fmt.Printf("%s Transaction:\n", chain.name)
		fmt.Printf("  ChainID: %d\n", extTx.ChainID)
		fmt.Printf("  Tx Data: %s\n", string(extTx.Tx))
		fmt.Println()
	}
}

// ExampleFluentAPI demonstrates the fluent API style
func ExampleFluentAPI() {
	ctx := context.Background()

	// Fluent API examples using concrete types and helper methods

	// Ethereum with helper methods (use helper methods first, then interface methods)
	ethBuilder := NewEVMTxBuilder()
	ethTx, _ := ethBuilder.
		Ethereum().         // Sets ChainID to 1
		SetValueEther(2.0). // Converts to wei automatically
		SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
		Build(ctx)

	// Polygon with contract call
	polygonBuilder := NewEVMTxBuilder()
	polygonTx, _ := polygonBuilder.
		Polygon(). // Sets ChainID to 137
		SetTo("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174").
		SetData([]byte("encoded_contract_call_data")).
		Build(ctx)

	// Solana mainnet with helper methods (use helper methods first)
	solanaBuilder := NewSolanaTxBuilder()
	solanaTx, _ := solanaBuilder.
		SetValueSOL(1.0). // Converts to lamports automatically
		SetChainID(101).  // Solana mainnet (no helper method available)
		SetTo("11111111111111111111111111111112").
		Build(ctx)

	fmt.Printf("Ethereum: %s\n", string(ethTx.Tx))
	fmt.Printf("Polygon: %s\n", string(polygonTx.Tx))
	fmt.Printf("Solana: %s\n", string(solanaTx.Tx))
}
