package external

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// This file demonstrates comprehensive usage of the unified ExTxBuilder
// for various real-world scenarios across multiple blockchains.

// === BASIC EXAMPLES ===

// ExampleBasicEthereum shows the simplest way to create an Ethereum transaction
func ExampleBasicEthereum() {
	ctx := context.Background()

	// Simple transfer payload - appchain defines the format
	payload := map[string]interface{}{
		"to":    "0x742d35Cc6493C35b1234567890abcdef",
		"value": "1000000000000000000", // 1 ETH in wei
		"data":  "",
	}

	payloadBytes, _ := json.Marshal(payload)

	// Modern unified API with chain helper method
	tx, err := NewExTxBuilder().
		Ethereum(). // Sets chainID to 1
		SetPayload(payloadBytes).
		Build(ctx)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Ethereum Transaction Created:\n")
	fmt.Printf("ChainID: %d\n", tx.ChainID)
	fmt.Printf("Payload Size: %d bytes\n", len(tx.Tx))
	// Output:
	// Ethereum Transaction Created:
	// ChainID: 1
	// Payload Size: 56 bytes
}

// ExampleBasicSolana shows how to create a Solana transaction
func ExampleBasicSolana() {
	ctx := context.Background()

	payload := map[string]interface{}{
		"to":       "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
		"lamports": "1000000",                          // 0.001 SOL
		"program":  "11111111111111111111111111111112", // System Program
	}

	payloadBytes, _ := json.Marshal(payload)

	tx, err := NewExTxBuilder().
		SolanaMainnet(). // Sets chainID to 900
		SetPayload(payloadBytes).
		Build(ctx)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Solana Transaction Created:\n")
	fmt.Printf("ChainID: %d\n", tx.ChainID)
	fmt.Printf("Payload Size: %d bytes\n", len(tx.Tx))
	// Output:
	// Solana Transaction Created:
	// ChainID: 900
	// Payload Size: 98 bytes
}

// === MULTI-CHAIN EXAMPLES ===

// ExampleMultiChainDeFiSwap demonstrates creating the same DeFi operation across multiple chains
func ExampleMultiChainDeFiSwap() {
	ctx := context.Background()

	// DeFi swap payload - same format works on all EVM chains
	swapPayload := map[string]interface{}{
		"action":       "swap",
		"protocol":     "universal_router",
		"tokenIn":      "0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef",
		"tokenOut":     "0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
		"amountIn":     "1000000000000000000",                        // 1 token
		"minAmountOut": "990000000",                                  // 990 USDT (1% slippage)
		"deadline":     1699564800,
	}

	payloadBytes, _ := json.Marshal(swapPayload)

	// Create transactions on multiple chains with identical payload
	chains := []struct {
		name    string
		builder func() *ExTxBuilder
	}{
		{"Ethereum", func() *ExTxBuilder { return NewExTxBuilder().Ethereum() }},
		{"Polygon", func() *ExTxBuilder { return NewExTxBuilder().Polygon() }},
		{"BSC", func() *ExTxBuilder { return NewExTxBuilder().BSC() }},
	}

	fmt.Println("Multi-Chain DeFi Swap Transactions:")
	for _, chain := range chains {
		tx, err := chain.builder().
			SetPayload(payloadBytes).
			Build(ctx)

		if err != nil {
			log.Printf("Error creating %s transaction: %v", chain.name, err)
			continue
		}

		fmt.Printf("✓ %s (ChainID %d): Swap transaction ready\n", chain.name, tx.ChainID)
	}
	// Output:
	// Multi-Chain DeFi Swap Transactions:
	// ✓ Ethereum (ChainID 1): Swap transaction ready
	// ✓ Polygon (ChainID 137): Swap transaction ready
	// ✓ BSC (ChainID 56): Swap transaction ready
}

// === TESTNET DEVELOPMENT WORKFLOW ===

// ExampleTestnetDeployment shows development workflow using testnets
func ExampleTestnetDeployment() {
	ctx := context.Background()

	// Smart contract deployment payload
	deployPayload := map[string]interface{}{
		"action":   "deploy",
		"bytecode": "0x608060405234801561001057600080fd5b50...", // Contract bytecode
		"args":     []string{"Atelerix Token", "ATX", "18"},
		"gasLimit": "2000000",
		"metadata": map[string]interface{}{
			"compiler":     "solc-0.8.19",
			"optimization": true,
			"runs":         200,
		},
	}

	payloadBytes, _ := json.Marshal(deployPayload)

	// Deploy on testnets first for testing
	testnets := []struct {
		name    string
		builder func() *ExTxBuilder
	}{
		{"Ethereum Sepolia", func() *ExTxBuilder { return NewExTxBuilder().EthereumSepolia() }},
		{"Polygon Amoy", func() *ExTxBuilder { return NewExTxBuilder().PolygonAmoy() }},
		{"BSC Testnet", func() *ExTxBuilder { return NewExTxBuilder().BSCTestnet() }},
		{"Solana Devnet", func() *ExTxBuilder { return NewExTxBuilder().SolanaDevnet() }},
	}

	fmt.Println("Deploying to testnets:")
	for _, testnet := range testnets {
		tx, err := testnet.builder().
			SetPayload(payloadBytes).
			Build(ctx)

		if err != nil {
			log.Printf("Error deploying to %s: %v", testnet.name, err)
			continue
		}

		fmt.Printf("✓ %s (ChainID %d): Deployment ready\n", testnet.name, tx.ChainID)
	}
	// Output:
	// Deploying to testnets:
	// ✓ Ethereum Sepolia (ChainID 11155111): Deployment ready
	// ✓ Polygon Amoy (ChainID 80002): Deployment ready
	// ✓ BSC Testnet (ChainID 97): Deployment ready
	// ✓ Solana Devnet (ChainID 901): Deployment ready
}

// === ADVANCED USE CASES ===

// ExampleAdvancedDeFi shows complex DeFi operations with detailed parameters
func ExampleAdvancedDeFi() {
	ctx := context.Background()

	defiPayload := map[string]interface{}{
		"action":   "multi_hop_swap",
		"protocol": "uniswap_v3",
		"route": []map[string]interface{}{
			{
				"tokenIn":  "0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef",
				"tokenOut": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
				"fee":      "3000",
			},
			{
				"tokenIn":  "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
				"tokenOut": "0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
				"fee":      "500",
			},
		},
		"amountIn":     "5000000000000000000", // 5 tokens
		"minAmountOut": "4950000000",          // 4950 USDT
		"deadline":     1699564800,
		"slippage":     "100", // 1% (100 basis points)
		"metadata": map[string]interface{}{
			"price_impact":   "0.15",
			"gas_estimate":   "350000",
			"mev_protection": true,
		},
	}

	payloadBytes, _ := json.Marshal(defiPayload)

	tx, err := NewExTxBuilder().
		Ethereum().
		SetPayload(payloadBytes).
		Build(ctx)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Advanced DeFi Transaction:\n")
	fmt.Printf("ChainID: %d\n", tx.ChainID)
	fmt.Printf("Operation: Multi-hop swap with MEV protection\n")
	fmt.Printf("Payload Size: %d bytes\n", len(tx.Tx))
	// Output:
	// Advanced DeFi Transaction:
	// ChainID: 1
	// Operation: Multi-hop swap with MEV protection
	// Payload Size: 445 bytes
}

// ExampleGameNFT shows gaming and NFT operations
func ExampleGameNFT() {
	ctx := context.Background()

	gamePayload := map[string]interface{}{
		"action":   "mint_achievement_nft",
		"gameId":   "atelerix-rpg",
		"playerId": "player_12345",
		"achievement": map[string]interface{}{
			"id":          "legendary_sword_master",
			"name":        "Legendary Sword Master",
			"description": "Defeated the Ancient Dragon with a legendary sword",
			"rarity":      "legendary",
			"attributes": map[string]interface{}{
				"damage_dealt": "50000",
				"time_taken":   "1200", // 20 minutes
				"difficulty":   "nightmare",
				"witnesses":    3,
			},
			"metadata": map[string]interface{}{
				"image":        "ipfs://QmHash...",
				"animation":    "ipfs://QmHash...",
				"timestamp":    "2024-01-15T14:30:00Z",
				"game_version": "1.2.3",
			},
		},
		"recipient": "0x742d35Cc6493C35b1234567890abcdef",
	}

	payloadBytes, _ := json.Marshal(gamePayload)

	tx, err := NewExTxBuilder().
		Polygon(). // Gaming often uses Polygon for low fees and fast finality
		SetPayload(payloadBytes).
		Build(ctx)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Game Achievement NFT Transaction:\n")
	fmt.Printf("ChainID: %d (Polygon)\n", tx.ChainID)
	fmt.Printf("Achievement: Legendary Sword Master\n")
	fmt.Printf("Player: player_12345\n")
	// Output:
	// Game Achievement NFT Transaction:
	// ChainID: 137 (Polygon)
	// Achievement: Legendary Sword Master
	// Player: player_12345
}

// ExampleRealWorldAssets shows Real World Asset (RWA) tokenization
func ExampleRealWorldAssets() {
	ctx := context.Background()

	rwaPayload := map[string]interface{}{
		"action":    "tokenize_property",
		"assetType": "real_estate",
		"property": map[string]interface{}{
			"id":             "manhattan_apt_001",
			"address":        "123 Wall Street, New York, NY 10005",
			"type":           "condominium",
			"square_feet":    "1200",
			"bedrooms":       "2",
			"bathrooms":      "2",
			"year_built":     "2020",
			"amenities":      []string{"gym", "roof_deck", "concierge", "parking"},
			"valuation":      "2500000", // $2.5M
			"appraiser":      "XYZ Appraisal Company",
			"appraisal_date": "2024-01-01",
		},
		"tokenization": map[string]interface{}{
			"token_symbol":    "MAPT001",
			"total_supply":    "1000000", // 1M tokens representing the property
			"price_per_token": "2.50",    // $2.50 per token
			"min_investment":  "1000",    // $1000 minimum
		},
		"compliance": map[string]interface{}{
			"kyc_required":    true,
			"accredited_only": true,
			"jurisdiction":    "US",
			"securities_law":  "RegD_506c",
			"transfer_restrictions": map[string]interface{}{
				"lock_period": "12months",
				"whitelist":   true,
			},
		},
		"legal": map[string]interface{}{
			"custodian":     "ABC Trust Company",
			"legal_entity":  "Manhattan Property LLC",
			"documentation": []string{"deed", "insurance", "legal_opinion"},
		},
	}

	payloadBytes, _ := json.Marshal(rwaPayload)

	tx, err := NewExTxBuilder().
		Ethereum(). // RWA often on Ethereum for institutional trust and liquidity
		SetPayload(payloadBytes).
		Build(ctx)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Real World Asset Tokenization:\n")
	fmt.Printf("ChainID: %d (Ethereum)\n", tx.ChainID)
	fmt.Printf("Property: Manhattan Condominium\n")
	fmt.Printf("Token: MAPT001\n")
	fmt.Printf("Valuation: $2.5M\n")
	// Output:
	// Real World Asset Tokenization:
	// ChainID: 1 (Ethereum)
	// Property: Manhattan Condominium
	// Token: MAPT001
	// Valuation: $2.5M
}

// === CHAIN-SPECIFIC OPTIMIZATIONS ===

// ExampleChainOptimizedOperations demonstrates leveraging each chain's strengths
func ExampleChainOptimizedOperations() {
	ctx := context.Background()

	// Ethereum - High value, security-critical institutional transfers
	ethPayload := map[string]interface{}{
		"action": "institutional_transfer",
		"amount": "10000000000000000000000", // 10,000 ETH
		"to":     "0x742d35Cc6493C35b1234567890abcdef",
		"proof":  "zk_proof_hash_here",
		"compliance": map[string]interface{}{
			"aml_check":       true,
			"sanctions_check": true,
			"source_of_funds": "verified",
		},
	}
	ethBytes, _ := json.Marshal(ethPayload)

	ethTx, _ := NewExTxBuilder().
		Ethereum().
		SetPayload(ethBytes).
		Build(ctx)

	// Polygon - High frequency, low-cost microtransactions
	polygonPayload := map[string]interface{}{
		"action":   "batch_microtransaction",
		"batch_id": "batch_001",
		"transactions": []map[string]interface{}{
			{"to": "0xabc...", "amount": "100", "memo": "reward_1"},
			{"to": "0xdef...", "amount": "200", "memo": "reward_2"},
			{"to": "0x123...", "amount": "150", "memo": "reward_3"},
		},
		"total_amount": "450",
	}
	polygonBytes, _ := json.Marshal(polygonPayload)

	polygonTx, _ := NewExTxBuilder().
		Polygon().
		SetPayload(polygonBytes).
		Build(ctx)

	// BSC - DeFi yield farming and trading operations
	bscPayload := map[string]interface{}{
		"action":        "yield_farm",
		"protocol":      "pancakeswap",
		"pool":          "CAKE-BNB",
		"operation":     "stake",
		"amount":        "5000000000000000000", // 5 LP tokens
		"auto_compound": true,
		"lock_period":   "30days",
	}
	bscBytes, _ := json.Marshal(bscPayload)

	bscTx, _ := NewExTxBuilder().
		BSC().
		SetPayload(bscBytes).
		Build(ctx)

	// Solana - High-speed programmatic DEX operations
	solanaPayload := map[string]interface{}{
		"action":        "serum_dex_trade",
		"market":        "SOL/USDC",
		"side":          "buy",
		"order_type":    "limit",
		"price":         "125.50",
		"quantity":      "100",
		"time_in_force": "GTC", // Good Till Canceled
		"post_only":     true,
	}
	solanaBytes, _ := json.Marshal(solanaPayload)

	solanaTx, _ := NewExTxBuilder().
		SolanaMainnet().
		SetPayload(solanaBytes).
		Build(ctx)

	fmt.Printf("Chain-Optimized Transactions:\n")
	fmt.Printf("Ethereum (ChainID %d): $100M institutional transfer\n", ethTx.ChainID)
	fmt.Printf("Polygon (ChainID %d): Batch microtransactions (3 txs)\n", polygonTx.ChainID)
	fmt.Printf("BSC (ChainID %d): Yield farming with auto-compound\n", bscTx.ChainID)
	fmt.Printf("Solana (ChainID %d): High-frequency DEX trading\n", solanaTx.ChainID)
	// Output:
	// Chain-Optimized Transactions:
	// Ethereum (ChainID 1): $100M institutional transfer
	// Polygon (ChainID 137): Batch microtransactions (3 txs)
	// BSC (ChainID 56): Yield farming with auto-compound
	// Solana (ChainID 900): High-frequency DEX trading
}

// === ERROR HANDLING AND VALIDATION ===

// ExampleErrorHandling shows proper error handling patterns
func ExampleErrorHandling() {
	ctx := context.Background()

	// Example 1: Missing chainID validation
	fmt.Println("Testing validation errors:")

	_, err := NewExTxBuilder().
		SetPayload([]byte(`{"test": "payload"}`)).
		Build(ctx)

	if err != nil {
		fmt.Printf("✓ Caught expected error: %v\n", err)
	}

	// Example 2: Proper usage with validation
	payload := map[string]interface{}{
		"action": "valid_transaction",
		"amount": "1000",
		"to":     "0x742d35Cc6493C35b1234567890abcdef",
	}
	payloadBytes, _ := json.Marshal(payload)

	tx, err := NewExTxBuilder().
		SetChainID(1).
		SetPayload(payloadBytes).
		Build(ctx)

	if err != nil {
		log.Printf("Unexpected error: %v", err)
		return
	}

	fmt.Printf("✓ Valid transaction created: ChainID %d\n", tx.ChainID)
	// Output:
	// Testing validation errors:
	// ✓ Caught expected error: chainID must be set
	// ✓ Valid transaction created: ChainID 1
}

// === CUSTOM CHAIN INTEGRATION ===

// ExampleCustomAppchain shows how to integrate custom/private appchains
func ExampleCustomAppchain() {
	ctx := context.Background()

	// Custom appchain with domain-specific operations
	customPayload := map[string]interface{}{
		"action":  "medical_record_update",
		"version": "1.0",
		"patient": map[string]interface{}{
			"id":   "patient_12345",
			"hash": "sha256_hash_of_medical_data",
		},
		"provider": map[string]interface{}{
			"id":        "provider_67890",
			"license":   "MD123456",
			"signature": "digital_signature_here",
		},
		"record": map[string]interface{}{
			"type":      "diagnosis",
			"timestamp": "2024-01-15T10:30:00Z",
			"encrypted": true,
			"ipfs_hash": "QmCustomMedicalRecordHash...",
		},
		"compliance": map[string]interface{}{
			"hipaa":   true,
			"gdpr":    true,
			"consent": "explicit",
		},
	}

	payloadBytes, _ := json.Marshal(customPayload)

	// Custom medical blockchain with chainID 888888
	tx, err := NewExTxBuilder().
		SetChainID(888888).
		SetPayload(payloadBytes).
		Build(ctx)

	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Custom Medical Appchain Transaction:\n")
	fmt.Printf("ChainID: %d\n", tx.ChainID)
	fmt.Printf("Operation: Medical record update\n")
	fmt.Printf("Compliance: HIPAA + GDPR\n")
	// Output:
	// Custom Medical Appchain Transaction:
	// ChainID: 888888
	// Operation: Medical record update
	// Compliance: HIPAA + GDPR
}

// === PERFORMANCE AND OPTIMIZATION ===

// ExampleBatchOperations shows efficient batch transaction creation
func ExampleBatchOperations() {
	ctx := context.Background()

	// Create multiple transactions efficiently
	basePayload := map[string]interface{}{
		"action":  "user_reward",
		"program": "loyalty_points",
	}

	users := []struct {
		id     string
		points int
		wallet string
	}{
		{"user_001", 100, "0x742d35Cc6493C35b1234567890abcdef"},
		{"user_002", 250, "0x1234567890abcdef742d35Cc6493C35b"},
		{"user_003", 75, "0xabcdef1234567890742d35Cc6493C35b"},
	}

	fmt.Println("Batch reward distribution:")
	for _, user := range users {
		// Customize payload per user
		userPayload := make(map[string]interface{})
		for k, v := range basePayload {
			userPayload[k] = v
		}
		userPayload["user_id"] = user.id
		userPayload["points"] = user.points
		userPayload["wallet"] = user.wallet

		payloadBytes, _ := json.Marshal(userPayload)

		tx, err := NewExTxBuilder().
			Polygon(). // Use Polygon for low-cost batch operations
			SetPayload(payloadBytes).
			Build(ctx)

		if err != nil {
			log.Printf("Error creating reward for %s: %v", user.id, err)
			continue
		}

		fmt.Printf("✓ %s: %d points → ChainID %d\n", user.id, user.points, tx.ChainID)
	}
	// Output:
	// Batch reward distribution:
	// ✓ user_001: 100 points → ChainID 137
	// ✓ user_002: 250 points → ChainID 137
	// ✓ user_003: 75 points → ChainID 137
}
