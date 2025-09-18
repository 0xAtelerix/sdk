package main

import (
	"encoding/json"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/0xAtelerix/sdk/gosdk/external"
	"github.com/0xAtelerix/sdk/gosdk/external/example/appchain"
)

// SolanaTransfer represents a Solana token transfer
type SolanaTransfer struct {
	To     string `json:"to"`     // Base58 Solana address
	Amount uint64 `json:"amount"` // Lamports
	Token  string `json:"token"`  // Token mint address
}

// SolanaExample demonstrates Solana program integration
func SolanaExample() {
	log.Println("=== Solana External Transaction Example ===")

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
		Build()
	if err != nil {
		log.Fatal("Failed to create Solana external transaction:", err)
	}

	log.Println("Solana External Transaction Created:")
	log.Printf("  Chain ID: %d (Solana Mainnet)\n", tx.ChainID)
	log.Printf("  Payload Size: %d bytes\n", len(tx.Tx))

	// 4. Simulate what happens in Solana program
	var decoded SolanaTransfer

	err = json.Unmarshal(tx.Tx, &decoded)
	if err != nil {
		log.Fatal("Failed to decode Solana transfer:", err)
	}

	log.Println("Decoded Solana Transfer:")
	log.Printf("  To: %s\n", decoded.To)
	log.Printf("  Amount: %d lamports (%.2f SOL)\n", decoded.Amount, float64(decoded.Amount)/1e9)
	log.Printf("  Token: %s\n", decoded.Token)

	log.Println("\n✓ TSS appchain will submit this to Solana program")
	log.Println("✓ Solana program decodes using same JSON format")
}

// MultiChainExample shows deploying to both EVM and Solana
func MultiChainExample() {
	log.Println("\n=== Multi-Chain: EVM + Solana Example ===")

	// 1. EVM chains - use ABI encoding (same as main.go)
	encoder := appchain.NewEncoder()

	evmTransferData := appchain.TransferData{
		To:     common.HexToAddress("0x742d35Cc6493C35b1234567890abcdef"),
		Amount: big.NewInt(1000000000000000000), // 1 ETH
		Token:  common.HexToAddress("0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef"),
	}

	evmData, err := encoder.EncodeTransfer(evmTransferData)
	if err != nil {
		log.Printf("Failed to encode EVM transfer: %v", err)

		return
	}

	// 2. Solana - use JSON encoding (program-specific)
	solanaPayload := SolanaTransfer{
		To:     "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
		Amount: 1000000000, // 1 SOL
		Token:  "So11111111111111111111111111111111111111112",
	}

	evmChains := []struct {
		name    string
		builder func() *external.ExTxBuilder
	}{
		{"Ethereum", external.NewExTxBuilder().Ethereum},
		{"BSC", external.NewExTxBuilder().BSC},
		{"Polygon", external.NewExTxBuilder().Polygon},
	}

	log.Println("EVM Deployments:")

	for _, chain := range evmChains {
		tx, chainErr := chain.builder().
			SetPayload(evmData).
			Build()
		if chainErr != nil {
			log.Printf("Failed %s: %v", chain.name, chainErr)

			continue
		}

		log.Printf("  ✓ %s (Chain %d) - %d bytes\n", chain.name, tx.ChainID, len(tx.Tx))
	}

	// 3. Deploy to Solana with JSON encoding
	solanaData, err := json.Marshal(solanaPayload)
	if err != nil {
		log.Printf("Failed to marshal Solana payload: %v", err)

		return
	}

	solanaTx, err := external.NewExTxBuilder().
		SolanaMainnet().
		SetPayload(solanaData).
		Build()
	if err != nil {
		log.Printf("Failed Solana: %v", err)
	} else {
		log.Printf("  ✓ Solana (Chain %d) - %d bytes\n", solanaTx.ChainID, len(solanaTx.Tx))
	}

	log.Println("\n✓ Same transfer logic deployed to EVM + Solana!")
	log.Println("✓ EVM chains use ABI encoding (contract-compatible)")
	log.Println("✓ Solana uses JSON encoding (program-specific)")
	log.Println("✓ TSS appchain handles all cross-chain coordination")
}
