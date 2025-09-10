package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/0xAtelerix/sdk/gosdk/external"
	"github.com/0xAtelerix/sdk/gosdk/external/example/appchain"
)

// TransferExample demonstrates a realistic token transfer flow
func TransferExample() {
	ctx := context.Background()

	// 1. Appchain side - encode actual transfer data
	encoder := appchain.NewEncoder()

	transferData := appchain.TransferData{
		To:     common.HexToAddress("0x742d35Cc6493C35b1234567890abcdef"),
		Amount: big.NewInt(1000000000000000000), // 1 ETH
		Token:  common.HexToAddress("0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef"),
	}

	payloadBytes, err := encoder.EncodeTransfer(transferData)
	if err != nil {
		log.Fatal("Failed to encode transfer:", err)
	}

	// 2. Create external transaction
	tx, err := external.NewExTxBuilder().
		Ethereum().
		SetPayload(payloadBytes).
		Build(ctx)
	if err != nil {
		log.Fatal("Failed to create external transaction:", err)
	}

	fmt.Printf("External Transaction Created: ChainID=%d, Size=%d bytes\n", tx.ChainID, len(tx.Tx))

	// 3. Simulate contract-side decoding (what happens in AppchainReceiver.sol)
	decodedData, err := encoder.DecodeTransfer(tx.Tx)
	if err != nil {
		log.Fatal("Failed to decode transfer:", err)
	}

	fmt.Printf("Decoded Transfer: %s → %s (%s wei)\n",
		decodedData.Token.Hex(),
		decodedData.To.Hex(),
		decodedData.Amount.String())
}

// Multi-chain example showing same payload on different chains
func SwapExample() {
	ctx := context.Background()

	encoder := appchain.NewEncoder()

	swapData := appchain.SwapData{
		TokenIn:      common.HexToAddress("0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef"),
		TokenOut:     common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7"),
		AmountIn:     big.NewInt(5000000000000000000), // 5 tokens
		MinAmountOut: big.NewInt(4950000000),          // 4950 USDT
		Deadline:     big.NewInt(1699564800),
		Slippage:     50, // 0.5%
	}

	payloadBytes, err := encoder.EncodeSwap(swapData)
	if err != nil {
		log.Fatal("Failed to encode swap:", err)
	}

	// Deploy to multiple chains
	chains := []struct {
		name    string
		builder func() *external.ExTxBuilder
	}{
		{"Ethereum", external.NewExTxBuilder().Ethereum},
		{"BSC", external.NewExTxBuilder().BSC},
		{"Polygon", external.NewExTxBuilder().Polygon},
	}

	fmt.Printf("Deploying swap to %d chains:\n", len(chains))

	for _, chain := range chains {
		tx, err := chain.builder().
			SetPayload(payloadBytes).
			Build(ctx)
		if err != nil {
			log.Printf("Failed %s: %v", chain.name, err)

			continue
		}

		fmt.Printf("✓ %s (Chain %d) - %d bytes\n", chain.name, tx.ChainID, len(tx.Tx))
	}
}

func main() {
	fmt.Println("=== External Transaction Examples ===")

	// Run EVM examples
	fmt.Println("\n1. Token Transfer:")
	TransferExample()

	fmt.Println("\n2. Multi-Chain Swap:")
	SwapExample()

	// Run Solana examples
	fmt.Println("\n3. Solana Integration:")
	SolanaExample()

	fmt.Println("\n4. Multi-Chain: EVM + Solana:")
	MultiChainExample()

	fmt.Println("\n✓ All examples completed successfully")
	fmt.Println("=== Summary ===")
	fmt.Println("• External transaction builder supports all chains")
	fmt.Println("• Use appropriate encoding for each chain (ABI vs Borsh vs JSON)")
	fmt.Println("• TSS appchain transports data regardless of encoding format")
	fmt.Println("• Your contracts/programs decode using same format you encoded with")
}
