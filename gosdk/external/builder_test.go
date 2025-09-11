package external

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestBasicBuilder(t *testing.T) {
	payload := map[string]any{
		"to":    "0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264",
		"value": "1000000000000000000",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Test Ethereum transaction
	ethTx, err := NewExTxBuilder().
		Ethereum().
		SetPayload(payloadBytes).
		Build()
	if err != nil {
		t.Fatalf("Failed to build Ethereum transaction: %v", err)
	}

	if ethTx.ChainID != 1 {
		t.Errorf("Expected ChainID 1, got %d", ethTx.ChainID)
	}

	// Verify payload integrity
	var receivedPayload map[string]any

	err = json.Unmarshal(ethTx.Tx, &receivedPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal transaction payload: %v", err)
	}

	if receivedPayload["to"] != payload["to"] {
		t.Errorf("Expected to address %s, got %s", payload["to"], receivedPayload["to"])
	}
}

func TestChainHelperMethods(t *testing.T) {
	payload := []byte(`{"test": "payload"}`)

	tests := []struct {
		name        string
		builderFunc func() *ExTxBuilder
		expectedID  uint64
	}{
		{"Ethereum", func() *ExTxBuilder { return NewExTxBuilder().Ethereum() }, 1},
		{
			"EthereumSepolia",
			func() *ExTxBuilder { return NewExTxBuilder().EthereumSepolia() },
			11155111,
		},
		{"Polygon", func() *ExTxBuilder { return NewExTxBuilder().Polygon() }, 137},
		{"PolygonAmoy", func() *ExTxBuilder { return NewExTxBuilder().PolygonAmoy() }, 80002},
		{"BSC", func() *ExTxBuilder { return NewExTxBuilder().BSC() }, 56},
		{"BSCTestnet", func() *ExTxBuilder { return NewExTxBuilder().BSCTestnet() }, 97},
		{"SolanaMainnet", func() *ExTxBuilder { return NewExTxBuilder().SolanaMainnet() }, 900},
		{"SolanaDevnet", func() *ExTxBuilder { return NewExTxBuilder().SolanaDevnet() }, 901},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx, err := test.builderFunc().
				SetPayload(payload).
				Build()
			if err != nil {
				t.Fatalf("Failed to build %s transaction: %v", test.name, err)
			}

			if tx.ChainID != test.expectedID {
				t.Errorf(
					"Expected ChainID %d for %s, got %d",
					test.expectedID,
					test.name,
					tx.ChainID,
				)
			}
		})
	}
}

func TestSolanaTransactions(t *testing.T) {
	payload := map[string]any{
		"to":       "9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
		"lamports": "1000000000",
		"program":  "11111111111111111111111111111112",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Test Solana mainnet
	mainnetTx, err := NewExTxBuilder().
		SolanaMainnet().
		SetPayload(payloadBytes).
		Build()
	if err != nil {
		t.Fatalf("Failed to build Solana mainnet transaction: %v", err)
	}

	if mainnetTx.ChainID != 900 {
		t.Errorf("Expected ChainID 900 for Solana mainnet, got %d", mainnetTx.ChainID)
	}

	// Test Solana devnet
	devnetTx, err := NewExTxBuilder().
		SolanaDevnet().
		SetPayload(payloadBytes).
		Build()
	if err != nil {
		t.Fatalf("Failed to build Solana devnet transaction: %v", err)
	}

	if devnetTx.ChainID != 901 {
		t.Errorf("Expected ChainID 901 for Solana devnet, got %d", devnetTx.ChainID)
	}
}

func TestValidation(t *testing.T) {
	// Test missing chainID
	_, err := NewExTxBuilder().
		SetPayload([]byte(`{"test": "payload"}`)).
		Build()
	if err == nil {
		t.Error("Expected error for missing chainID, got nil")
	}

	if !errors.Is(err, ErrChainIDRequired) {
		t.Errorf("Expected ErrChainIDRequired, got %v", err)
	}

	// Test valid transaction
	tx, err := NewExTxBuilder().
		SetChainID(1).
		SetPayload([]byte(`{"test": "payload"}`)).
		Build()
	if err != nil {
		t.Errorf("Unexpected error for valid transaction: %v", err)
	}

	if tx.ChainID != 1 {
		t.Errorf("Expected ChainID 1, got %d", tx.ChainID)
	}
}

func TestAdvancedPayloads(t *testing.T) {
	// Test DeFi swap payload
	defiPayload := map[string]any{
		"action":       "swap",
		"protocol":     "uniswap_v3",
		"tokenIn":      "0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef",
		"tokenOut":     "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		"amountIn":     "5000000000000000000",
		"minAmountOut": "4950000000",
		"deadline":     1699564800,
		"slippage":     "100", // 1%
	}

	defiBytes, err := json.Marshal(defiPayload)
	if err != nil {
		t.Fatalf("Failed to marshal DeFi payload: %v", err)
	}

	defiTx, err := NewExTxBuilder().
		Ethereum().
		SetPayload(defiBytes).
		Build()
	if err != nil {
		t.Fatalf("Failed to build DeFi transaction: %v", err)
	}

	var receivedDefiPayload map[string]any

	err = json.Unmarshal(defiTx.Tx, &receivedDefiPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal DeFi payload: %v", err)
	}

	if receivedDefiPayload["action"] != defiPayload["action"] {
		t.Errorf("Expected action %s, got %s", defiPayload["action"], receivedDefiPayload["action"])
	}

	if receivedDefiPayload["protocol"] != defiPayload["protocol"] {
		t.Errorf(
			"Expected protocol %s, got %s",
			defiPayload["protocol"],
			receivedDefiPayload["protocol"],
		)
	}
}

func TestGameNFTPayload(t *testing.T) {
	gamePayload := map[string]any{
		"action":    "mint",
		"gameId":    "atelerix-rpg",
		"playerId":  "player_12345",
		"assetType": "nft",
		"assetId":   "legendary_sword_001",
		"attributes": map[string]any{
			"rarity":       "legendary",
			"damage":       "150",
			"durability":   "100",
			"enchantments": []string{"fire", "sharpness"},
			"level":        "50",
		},
		"recipient": "0x742d35Cc6493C35b1234567890abcdef",
	}

	gameBytes, err := json.Marshal(gamePayload)
	if err != nil {
		t.Fatalf("Failed to marshal game payload: %v", err)
	}

	gameTx, err := NewExTxBuilder().
		Polygon(). // Gaming often uses Polygon for low fees
		SetPayload(gameBytes).
		Build()
	if err != nil {
		t.Fatalf("Failed to build game transaction: %v", err)
	}

	var receivedGamePayload map[string]any

	err = json.Unmarshal(gameTx.Tx, &receivedGamePayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal game payload: %v", err)
	}

	if receivedGamePayload["gameId"] != gamePayload["gameId"] {
		t.Errorf("Expected gameId %s, got %s", gamePayload["gameId"], receivedGamePayload["gameId"])
	}

	if receivedGamePayload["assetId"] != gamePayload["assetId"] {
		t.Errorf(
			"Expected assetId %s, got %s",
			gamePayload["assetId"],
			receivedGamePayload["assetId"],
		)
	}
}

func TestMultiChainWorkflow(t *testing.T) {
	swapPayload := map[string]any{
		"action":       "swap",
		"tokenIn":      "0xA0b86a33E6441c9fa6e8Ee5B1234567890abcdef",
		"tokenOut":     "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		"amountIn":     "1000000000000000000",
		"minAmountOut": "990000000",
	}

	payloadBytes, err := json.Marshal(swapPayload)
	if err != nil {
		t.Fatalf("Failed to marshal swap payload: %v", err)
	}

	chains := []struct {
		name     string
		builder  func() *ExTxBuilder
		expected uint64
	}{
		{"Ethereum", func() *ExTxBuilder { return NewExTxBuilder().Ethereum() }, 1},
		{"Polygon", func() *ExTxBuilder { return NewExTxBuilder().Polygon() }, 137},
		{"BSC", func() *ExTxBuilder { return NewExTxBuilder().BSC() }, 56},
	}

	for _, chain := range chains {
		t.Run(chain.name, func(t *testing.T) {
			tx, err := chain.builder().
				SetPayload(payloadBytes).
				Build()
			if err != nil {
				t.Fatalf("Error creating %s transaction: %v", chain.name, err)
			}

			if tx.ChainID != chain.expected {
				t.Errorf(
					"Expected ChainID %d for %s, got %d",
					chain.expected,
					chain.name,
					tx.ChainID,
				)
			}

			// Verify payload integrity across chains
			var receivedPayload map[string]any

			err = json.Unmarshal(tx.Tx, &receivedPayload)
			if err != nil {
				t.Fatalf("Failed to unmarshal payload for %s: %v", chain.name, err)
			}

			if receivedPayload["action"] != swapPayload["action"] {
				t.Errorf("Payload corrupted for %s chain", chain.name)
			}
		})
	}
}

func TestCustomChain(t *testing.T) {
	customPayload := map[string]any{
		"action":        "custom_operation",
		"appchain_data": "custom_encoding_here",
		"version":       "1.0",
		"metadata": map[string]any{
			"timestamp":   "2024-01-01T00:00:00Z",
			"priority":    "high",
			"retry_count": 3,
		},
	}

	payloadBytes, err := json.Marshal(customPayload)
	if err != nil {
		t.Fatalf("Failed to marshal custom payload: %v", err)
	}

	// Test custom chain with explicit chainID
	tx, err := NewExTxBuilder().
		SetChainID(999999).
		SetPayload(payloadBytes).
		Build()
	if err != nil {
		t.Fatalf("Failed to create custom chain transaction: %v", err)
	}

	if tx.ChainID != 999999 {
		t.Errorf("Expected ChainID 999999, got %d", tx.ChainID)
	}

	var receivedPayload map[string]any

	err = json.Unmarshal(tx.Tx, &receivedPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal custom payload: %v", err)
	}

	if receivedPayload["action"] != customPayload["action"] {
		t.Error("Custom payload corrupted")
	}

	if receivedPayload["version"] != customPayload["version"] {
		t.Errorf(
			"Expected version %s, got %s",
			customPayload["version"],
			receivedPayload["version"],
		)
	}
}

func TestTestnetWorkflow(t *testing.T) {
	deployPayload := map[string]any{
		"action":   "deploy",
		"bytecode": "0x608060405234801561001057600080fd5b50...",
		"args":     []string{"Atelerix Token", "ATX", "18"},
		"gasLimit": "2000000",
	}

	payloadBytes, err := json.Marshal(deployPayload)
	if err != nil {
		t.Fatalf("Failed to marshal deploy payload: %v", err)
	}

	testnets := []struct {
		name     string
		builder  func() *ExTxBuilder
		expected uint64
	}{
		{
			"EthereumSepolia",
			func() *ExTxBuilder { return NewExTxBuilder().EthereumSepolia() },
			11155111,
		},
		{"PolygonAmoy", func() *ExTxBuilder { return NewExTxBuilder().PolygonAmoy() }, 80002},
		{"BSCTestnet", func() *ExTxBuilder { return NewExTxBuilder().BSCTestnet() }, 97},
		{"SolanaDevnet", func() *ExTxBuilder { return NewExTxBuilder().SolanaDevnet() }, 901},
	}

	for _, testnet := range testnets {
		t.Run(testnet.name, func(t *testing.T) {
			tx, err := testnet.builder().
				SetPayload(payloadBytes).
				Build()
			if err != nil {
				t.Fatalf("Error deploying to %s: %v", testnet.name, err)
			}

			if tx.ChainID != testnet.expected {
				t.Errorf(
					"Expected ChainID %d for %s, got %d",
					testnet.expected,
					testnet.name,
					tx.ChainID,
				)
			}
		})
	}
}

func TestPayloadCopying(t *testing.T) {
	originalPayload := []byte(`{"sensitive": "data"}`)

	tx, err := NewExTxBuilder().
		Ethereum().
		SetPayload(originalPayload).
		Build()
	if err != nil {
		t.Fatalf("Failed to build transaction: %v", err)
	}

	// Modify original payload to ensure it was copied
	originalPayload[0] = 'X'

	// Transaction should still have original data
	if string(tx.Tx) == string(originalPayload) {
		t.Error("Payload was not properly copied - modification affected transaction")
	}

	if string(tx.Tx) != `{"sensitive": "data"}` {
		t.Errorf("Expected transaction to preserve original payload, got %s", string(tx.Tx))
	}
}

func TestBuilderChaining(t *testing.T) {
	payload := []byte(`{"test": "chaining"}`)

	// Test that builder methods return the same instance for chaining
	builder := NewExTxBuilder()

	result1 := builder.Ethereum()
	result2 := result1.SetPayload(payload)

	// These should all be the same instance
	if result1 != builder {
		t.Error("Ethereum() should return the same builder instance")
	}

	if result2 != builder {
		t.Error("SetPayload() should return the same builder instance")
	}

	tx, err := result2.Build()
	if err != nil {
		t.Fatalf("Failed to build chained transaction: %v", err)
	}

	if tx.ChainID != 1 {
		t.Errorf("Expected ChainID 1 from chained calls, got %d", tx.ChainID)
	}
}
