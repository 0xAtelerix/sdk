package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/0xAtelerix/sdk/gosdk/external"
)

// SolanaTransfer represents a Solana token transfer
type SolanaTransfer struct {
	To     string `json:"to"`     // Base58 Solana address
	Amount uint64 `json:"amount"` // Lamports
	Token  string `json:"token"`  // Token mint address
}

// SolanaExample demonstrates Solana program integration
func SolanaExample() {
	ctx := context.Background()

	fmt.Println("=== Solana External Transaction Example ===")

	// 1. Create Solana transfer data
	transferData := SolanaTransfer{
		To:     "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM", // Example Solana address
		Amount: 1000000000,                                     // 1 SOL in lamports
		Token:  "So11111111111111111111111111111111111111112",  // Wrapped SOL
	}

	// 2. Encode for Solana (using JSON for simplicity, could use Borsh)
	payloadBytes, err := json.Marshal(transferData)
	if err != nil {
		log.Fatal("Failed to encode Solana transfer:", err)
	}

	// 3. Create external transaction for Solana
	tx, err := external.NewExTxBuilder().
		SolanaMainnet().
		SetPayload(payloadBytes).
		Build(ctx)
	if err != nil {
		log.Fatal("Failed to create Solana external transaction:", err)
	}

	fmt.Println("Solana External Transaction Created:")
	fmt.Printf("  Chain ID: %d (Solana Mainnet)\n", tx.ChainID)
	fmt.Printf("  Payload Size: %d bytes\n", len(tx.Tx))

	// 4. Simulate what happens in Solana program
	var decoded SolanaTransfer

	err = json.Unmarshal(tx.Tx, &decoded)
	if err != nil {
		log.Fatal("Failed to decode Solana transfer:", err)
	}

	fmt.Println("Decoded Solana Transfer:")
	fmt.Printf("  To: %s\n", decoded.To)
	fmt.Printf("  Amount: %d lamports (%.2f SOL)\n", decoded.Amount, float64(decoded.Amount)/1e9)
	fmt.Printf("  Token: %s\n", decoded.Token)

	fmt.Println("\n✓ TSS appchain will submit this to Solana program")
	fmt.Println("✓ Solana program decodes using same JSON format")
}

// MultiChainExample shows deploying to both EVM and Solana
func MultiChainExample() {
	ctx := context.Background()

	fmt.Println("\n=== Multi-Chain: EVM + Solana Example ===")

	// Same logical operation, different encoding formats
	evmPayload := map[string]any{
		"action": "transfer",
		"to":     "0x742d35Cc6493C35b1234567890abcdef",
		"amount": "1000000000000000000", // 1 ETH
	}

	solanaPayload := SolanaTransfer{
		To:     "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
		Amount: 1000000000, // 1 SOL
		Token:  "So11111111111111111111111111111111111111112",
	}

	// Deploy to EVM chains
	evmData, err := json.Marshal(evmPayload)
	if err != nil {
		log.Printf("Failed to marshal EVM payload: %v", err)

		return
	}

	evmChains := []struct {
		name    string
		builder func() *external.ExTxBuilder
	}{
		{"Ethereum", external.NewExTxBuilder().Ethereum},
		{"BSC", external.NewExTxBuilder().BSC},
		{"Polygon", external.NewExTxBuilder().Polygon},
	}

	fmt.Println("EVM Deployments:")

	for _, chain := range evmChains {
		tx, chainErr := chain.builder().
			SetPayload(evmData).
			Build(ctx)
		if chainErr != nil {
			log.Printf("Failed %s: %v", chain.name, chainErr)

			continue
		}

		fmt.Printf("  ✓ %s (Chain %d) - %d bytes\n", chain.name, tx.ChainID, len(tx.Tx))
	}

	// Deploy to Solana
	solanaData, err := json.Marshal(solanaPayload)
	if err != nil {
		log.Printf("Failed to marshal Solana payload: %v", err)

		return
	}

	solanaTx, err := external.NewExTxBuilder().
		SolanaMainnet().
		SetPayload(solanaData).
		Build(ctx)
	if err != nil {
		log.Printf("Failed Solana: %v", err)
	} else {
		fmt.Printf("  ✓ Solana (Chain %d) - %d bytes\n", solanaTx.ChainID, len(solanaTx.Tx))
	}

	fmt.Println("\n✓ Same transfer logic deployed to EVM + Solana!")
	fmt.Println("✓ Each chain uses its native encoding format")
	fmt.Println("✓ TSS appchain handles all cross-chain coordination")
}
