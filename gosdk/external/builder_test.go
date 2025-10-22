package external

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
)

func TestBasicBuilder(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"to":    "0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264",
		"value": "1000000000000000000",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Test Ethereum transaction
	ethTx, err := NewExTxBuilder(payloadBytes, library.EthereumChainID).Build()
	if err != nil {
		t.Fatalf("Failed to build Ethereum transaction: %v", err)
	}

	if ethTx.ChainID != library.EthereumChainID {
		t.Errorf("Expected ChainID %d, got %d", library.EthereumChainID, ethTx.ChainID)
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
	t.Parallel()

	payload := []byte(`{"test": "payload"}`)

	tests := []struct {
		name       string
		chainID    apptypes.ChainType
		expectedID apptypes.ChainType
	}{
		{
			"Ethereum",
			library.EthereumChainID,
			library.EthereumChainID,
		},
		{
			"EthereumSepolia",
			library.EthereumSepoliaChainID,
			library.EthereumSepoliaChainID,
		},
		{
			"Polygon",
			library.PolygonChainID,
			library.PolygonChainID,
		},
		{
			"PolygonAmoy",
			library.PolygonAmoyChainID,
			library.PolygonAmoyChainID,
		},
		{
			"BSC",
			library.BNBChainID,
			library.BNBChainID,
		},
		{
			"BSCTestnet",
			library.BNBTestnetChainID,
			library.BNBTestnetChainID,
		},
		{
			"SolanaMainnet",
			library.SolanaChainID,
			library.SolanaChainID,
		},
		{
			"SolanaDevnet",
			library.SolanaDevnetChainID,
			library.SolanaDevnetChainID,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			tt.Parallel()

			b := NewExTxBuilder(payload, test.chainID)

			if library.IsSolanaChain(test.chainID) {
				b.AddSolanaAccounts([]types.AccountMeta{{}})
			}

			tx, err := b.Build()
			if err != nil {
				tt.Fatalf("Failed to build %s transaction: %v", test.name, err)
			}

			if tx.ChainID != test.expectedID {
				tt.Errorf(
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
	t.Parallel()

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
	mainnetTx, err := NewExTxBuilder(payloadBytes, library.SolanaChainID).
		AddSolanaAccounts([]types.AccountMeta{{}}).
		Build()
	require.NoError(t, err)

	require.Equal(t, mainnetTx.ChainID, library.SolanaChainID, "Unexpected ChainID for Solana mainnet")
	if mainnetTx.ChainID != library.SolanaChainID {
		t.Errorf(
			"Expected ChainID %d for Solana mainnet, got %d",
			library.SolanaChainID,
			mainnetTx.ChainID,
		)
	}

	// Test Solana devnet
	devnetTx, err := NewExTxBuilder(payloadBytes, library.SolanaDevnetChainID).AddSolanaAccounts([]types.AccountMeta{{}}).Build()
	require.NoError(t, err)

	if devnetTx.ChainID != library.SolanaDevnetChainID {
		t.Errorf(
			"Expected ChainID %d for Solana devnet, got %d",
			library.SolanaDevnetChainID,
			devnetTx.ChainID,
		)
	}
}

func TestSolanaPayload_BuildAndDecode(t *testing.T) {
	t.Parallel()

	// Test data
	payload := map[string]any{
		"instruction": "transfer",
		"amount":      "1000000",
		"memo":        "test transfer",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Create test accounts
	accounts := []types.AccountMeta{
		{
			PubKey: common.PublicKeyFromString(
				"11111111111111111111111111111112",
			), // System program
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PubKey: common.PublicKeyFromString(
				"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
			), // From account
			IsSigner:   true,
			IsWritable: true,
		},
		{
			PubKey: common.PublicKeyFromString(
				"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			), // To account
			IsSigner:   false,
			IsWritable: true,
		},
	}

	// Build Solana payload
	tx, err := NewExTxBuilder(payloadBytes, library.SolanaChainID).
		AddSolanaAccounts(accounts).
		Build()
	if err != nil {
		t.Fatalf("Failed to build Solana payload: %v", err)
	}

	if tx.ChainID != library.SolanaChainID {
		t.Errorf("Expected ChainID %d, got %d", library.SolanaChainID, tx.ChainID)
	}

	// Decode the payload
	decodedAccounts, decodedData, err := DecodeSolanaPayload(tx.Tx)
	if err != nil {
		t.Fatalf("Failed to decode Solana payload: %v", err)
	}

	// Verify accounts
	if len(decodedAccounts) != len(accounts) {
		t.Errorf("Expected %d accounts, got %d", len(accounts), len(decodedAccounts))
	}

	for i, expected := range accounts {
		if i >= len(decodedAccounts) {
			t.Errorf("Missing account at index %d", i)

			continue
		}

		actual := decodedAccounts[i]
		if actual.PubKey != expected.PubKey {
			t.Errorf("Account %d: expected PubKey %s, got %s", i, expected.PubKey, actual.PubKey)
		}

		if actual.IsSigner != expected.IsSigner {
			t.Errorf(
				"Account %d: expected IsSigner %v, got %v",
				i,
				expected.IsSigner,
				actual.IsSigner,
			)
		}

		if actual.IsWritable != expected.IsWritable {
			t.Errorf(
				"Account %d: expected IsWritable %v, got %v",
				i,
				expected.IsWritable,
				actual.IsWritable,
			)
		}
	}

	// Verify data
	var decodedPayload map[string]any

	err = json.Unmarshal(decodedData, &decodedPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal decoded data: %v", err)
	}

	if decodedPayload["instruction"] != payload["instruction"] {
		t.Errorf(
			"Expected instruction %s, got %s",
			payload["instruction"],
			decodedPayload["instruction"],
		)
	}

	if decodedPayload["amount"] != payload["amount"] {
		t.Errorf("Expected amount %s, got %s", payload["amount"], decodedPayload["amount"])
	}
}

func TestSolanaPayload_EmptyAccounts(t *testing.T) {
	t.Parallel()

	payloadBytes := []byte(`{"test": "empty accounts"}`)

	// Build with empty accounts
	tx, err := NewExTxBuilder(payloadBytes, library.SolanaChainID).
		AddSolanaAccounts([]types.AccountMeta{{}}).
		Build()
	if err != nil {
		t.Fatalf("Failed to build Solana payload with empty accounts: %v", err)
	}

	// Decode
	decodedAccounts, decodedData, err := DecodeSolanaPayload(tx.Tx)
	if err != nil {
		t.Fatalf("Failed to decode Solana payload: %v", err)
	}

	if len(decodedAccounts) != 1 {
		t.Errorf("Expected 1 accounts, got %d", len(decodedAccounts))
	}

	if string(decodedData) != string(payloadBytes) {
		t.Errorf("Expected data %s, got %s", string(payloadBytes), string(decodedData))
	}
}

func TestSolanaPayload_SingleAccount(t *testing.T) {
	t.Parallel()

	payloadBytes := []byte(`{"action": "create_account"}`)

	account := types.AccountMeta{
		PubKey:     common.PublicKeyFromString("11111111111111111111111111111112"),
		IsSigner:   true,
		IsWritable: false,
	}

	tx, err := NewExTxBuilder(payloadBytes, library.SolanaChainID).
		AddSolanaAccounts([]types.AccountMeta{account}).
		Build()
	if err != nil {
		t.Fatalf("Failed to build Solana payload: %v", err)
	}

	decodedAccounts, decodedData, err := DecodeSolanaPayload(tx.Tx)
	if err != nil {
		t.Fatalf("Failed to decode Solana payload: %v", err)
	}

	if len(decodedAccounts) != 1 {
		t.Errorf("Expected 1 account, got %d", len(decodedAccounts))
	}

	if decodedAccounts[0].PubKey != account.PubKey {
		t.Errorf("Expected PubKey %s, got %s", account.PubKey, decodedAccounts[0].PubKey)
	}

	if decodedAccounts[0].IsSigner != account.IsSigner {
		t.Errorf("Expected IsSigner %v, got %v", account.IsSigner, decodedAccounts[0].IsSigner)
	}

	if decodedAccounts[0].IsWritable != account.IsWritable {
		t.Errorf(
			"Expected IsWritable %v, got %v",
			account.IsWritable,
			decodedAccounts[0].IsWritable,
		)
	}

	if string(decodedData) != string(payloadBytes) {
		t.Errorf("Expected data %s, got %s", string(payloadBytes), string(decodedData))
	}
}

func TestSolanaPayload_MaxAccounts(t *testing.T) {
	t.Parallel()

	payloadBytes := []byte(`{"test": "max accounts"}`)

	// Create 64 accounts (maximum allowed)
	accounts := make([]types.AccountMeta, 64)
	for i := range accounts {
		accounts[i] = types.AccountMeta{
			PubKey:     common.PublicKeyFromString("11111111111111111111111111111112"),
			IsSigner:   i%2 == 0, // Alternate signers
			IsWritable: i%3 == 0, // Every third account writable
		}
	}

	tx, err := NewExTxBuilder(payloadBytes, library.SolanaChainID).
		AddSolanaAccounts(accounts).
		Build()
	if err != nil {
		t.Fatalf("Failed to build Solana payload with max accounts: %v", err)
	}

	decodedAccounts, decodedData, err := DecodeSolanaPayload(tx.Tx)
	if err != nil {
		t.Fatalf("Failed to decode Solana payload: %v", err)
	}

	if len(decodedAccounts) != 64 {
		t.Errorf("Expected 64 accounts, got %d", len(decodedAccounts))
	}

	// Verify first few accounts
	for i := 0; i < 5 && i < len(decodedAccounts); i++ {
		if decodedAccounts[i].PubKey != accounts[i].PubKey {
			t.Errorf("Account %d: PubKey mismatch", i)
		}

		if decodedAccounts[i].IsSigner != accounts[i].IsSigner {
			t.Errorf("Account %d: IsSigner mismatch", i)
		}

		if decodedAccounts[i].IsWritable != accounts[i].IsWritable {
			t.Errorf("Account %d: IsWritable mismatch", i)
		}
	}

	if string(decodedData) != string(payloadBytes) {
		t.Error("Data mismatch")
	}
}

func TestSolanaPayload_TooManyAccounts(t *testing.T) {
	t.Parallel()

	payloadBytes := []byte(`{"test": "too many"}`)

	// Create 65 accounts (exceeds maximum)
	accounts := make([]types.AccountMeta, 65)
	for i := range accounts {
		accounts[i] = types.AccountMeta{
			PubKey:     common.PublicKeyFromString("11111111111111111111111111111112"),
			IsSigner:   false,
			IsWritable: false,
		}
	}

	_, err := NewExTxBuilder(payloadBytes, library.SolanaChainID).
		AddSolanaAccounts(accounts).
		Build()
	if err == nil {
		t.Error("Expected error for too many accounts, got nil")
	}

	if !errors.Is(err, ErrTooManyAccounts) {
		t.Errorf("Expected ErrTooManyAccounts, got %v", err)
	}
}

func TestDecodeSolanaPayload_InvalidData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
		wantErr bool
	}{
		{
			name:    "empty payload",
			payload: []byte{},
			wantErr: true,
		},
		{
			name:    "incomplete account data",
			payload: []byte{1}, // num_accounts=1, but no account data
			wantErr: true,
		},
		{
			name:    "truncated pubkey",
			payload: []byte{1, 1, 2, 3}, // num_accounts=1, incomplete pubkey
			wantErr: true,
		},
		{
			name: "missing flags",
			payload: []byte{
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
				1,
			}, // pubkey but no flags
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(tr *testing.T) {
			tr.Parallel()

			_, _, err := DecodeSolanaPayload(tt.payload)
			if (err != nil) != tt.wantErr {
				tr.Errorf("DecodeSolanaPayload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSolanaPayload_RoundTrip(t *testing.T) {
	t.Parallel()

	// Test various combinations of accounts and data
	testCases := []struct {
		name     string
		payload  []byte
		accounts []types.AccountMeta
		err      error
	}{
		{
			name:    "simple transfer",
			payload: []byte(`{"instruction":"transfer","amount":"1000000"}`),
			accounts: []types.AccountMeta{
				{
					PubKey:     common.PublicKeyFromString("11111111111111111111111111111112"),
					IsSigner:   false,
					IsWritable: false,
				},
				{
					PubKey: common.PublicKeyFromString(
						"9WzDXwBbmkg8ZTbNMqUxvQRAyrZzDsGYdLVL9zYtAWWM",
					),
					IsSigner:   true,
					IsWritable: true,
				},
				{
					PubKey: common.PublicKeyFromString(
						"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
					),
					IsSigner:   false,
					IsWritable: true,
				},
			},
		},
		{
			name:     "no accounts",
			payload:  []byte(`{"instruction":"memo","message":"hello world"}`),
			accounts: []types.AccountMeta{},
			err:      ErrNoAccountsGiven,
		},
		{
			name:    "program only",
			payload: []byte(`{"instruction":"create","size":100}`),
			accounts: []types.AccountMeta{
				{
					PubKey: common.PublicKeyFromString(
						"BPFLoaderUpgradeab1e11111111111111111111111",
					),
					IsSigner:   false,
					IsWritable: false,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tr *testing.T) {
			tr.Parallel()

			// Build
			tx, err := NewExTxBuilder(tc.payload, library.SolanaChainID).
				AddSolanaAccounts(tc.accounts).
				Build()
			if tc.err != nil {
				require.ErrorIs(tr, err, tc.err)
				return
			}
			require.NoError(tr, err)

			// Decode
			decodedAccounts, decodedData, err := DecodeSolanaPayload(tx.Tx)
			if err != nil {
				tr.Fatalf("Failed to decode: %v", err)
			}

			// Verify
			if len(decodedAccounts) != len(tc.accounts) {
				tr.Errorf(
					"Account count mismatch: expected %d, got %d",
					len(tc.accounts),
					len(decodedAccounts),
				)
			}

			for i, expected := range tc.accounts {
				if i >= len(decodedAccounts) {
					continue
				}

				actual := decodedAccounts[i]
				if actual.PubKey != expected.PubKey ||
					actual.IsSigner != expected.IsSigner ||
					actual.IsWritable != expected.IsWritable {
					tr.Errorf("Account %d mismatch", i)
				}
			}

			if string(decodedData) != string(tc.payload) {
				tr.Errorf(
					"Data mismatch: expected %s, got %s",
					string(tc.payload),
					string(decodedData),
				)
			}
		})
	}
}

func TestValidation(t *testing.T) {
	t.Parallel()

	// Test missing chainID
	_, err := NewExTxBuilder([]byte(`{"test": "payload"}`), apptypes.ChainType(0)).Build()
	if err == nil {
		t.Error("Expected error for missing chainID, got nil")
	}

	if !errors.Is(err, ErrChainIDRequired) {
		t.Errorf("Expected ErrChainIDRequired, got %v", err)
	}

	// Test valid transaction
	tx, err := NewExTxBuilder([]byte(`{"test": "payload"}`), library.EthereumChainID).Build()
	if err != nil {
		t.Errorf("Unexpected error for valid transaction: %v", err)
	}

	if tx.ChainID != library.EthereumChainID {
		t.Errorf("Expected ChainID %d, got %d", library.EthereumChainID, tx.ChainID)
	}
}

func TestAdvancedPayloads(t *testing.T) {
	t.Parallel()

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

	defiTx, err := NewExTxBuilder(defiBytes, library.EthereumChainID).Build()
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
	t.Parallel()

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

	gameTx, err := NewExTxBuilder(gameBytes, library.PolygonChainID).Build()
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
	t.Parallel()

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
		expected apptypes.ChainType
	}{
		{
			"Ethereum",
			func() *ExTxBuilder { return NewExTxBuilder(payloadBytes, library.EthereumChainID) },
			library.EthereumChainID,
		},
		{
			"Polygon",
			func() *ExTxBuilder { return NewExTxBuilder(payloadBytes, library.PolygonChainID) },
			library.PolygonChainID,
		},
		{
			"BSC",
			func() *ExTxBuilder { return NewExTxBuilder(payloadBytes, library.BNBChainID) },
			library.BNBChainID,
		},
	}

	for _, chain := range chains {
		t.Run(chain.name, func(tt *testing.T) {
			tt.Parallel()

			tx, err := chain.builder().
				SetPayload(payloadBytes).
				Build()
			if err != nil {
				tt.Fatalf("Error creating %s transaction: %v", chain.name, err)
			}

			if tx.ChainID != chain.expected {
				tt.Errorf(
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
				tt.Fatalf("Failed to unmarshal payload for %s: %v", chain.name, err)
			}

			if receivedPayload["action"] != swapPayload["action"] {
				tt.Errorf("Payload corrupted for %s chain", chain.name)
			}
		})
	}
}

func TestCustomChain(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err)

	// Test custom chain with explicit chainID
	_, err = NewExTxBuilder(payloadBytes, apptypes.ChainType(999999)).Build()
	require.ErrorIs(t, err, ErrChainIDRequired)
}

func TestTestnetWorkflow(t *testing.T) {
	t.Parallel()

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
		expected apptypes.ChainType
	}{
		{
			"EthereumSepolia",
			func() *ExTxBuilder { return NewExTxBuilder(payloadBytes, library.EthereumSepoliaChainID) },
			library.EthereumSepoliaChainID,
		},
		{
			"PolygonAmoy",
			func() *ExTxBuilder { return NewExTxBuilder(payloadBytes, library.PolygonAmoyChainID) },
			library.PolygonAmoyChainID,
		},
		{
			"BSCTestnet",
			func() *ExTxBuilder { return NewExTxBuilder(payloadBytes, library.BNBTestnetChainID) },
			library.BNBTestnetChainID,
		},
		{
			"SolanaDevnet",
			func() *ExTxBuilder {
				return NewExTxBuilder(payloadBytes, library.SolanaDevnetChainID).AddSolanaAccounts([]types.AccountMeta{{}})
			},
			library.SolanaDevnetChainID,
		},
	}

	for _, testnet := range testnets {
		t.Run(testnet.name, func(tt *testing.T) {
			tt.Parallel()

			tx, err := testnet.builder().
				SetPayload(payloadBytes).
				Build()
			if err != nil {
				tt.Fatalf("Error deploying to %s: %v", testnet.name, err)
			}

			if tx.ChainID != testnet.expected {
				tt.Errorf(
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
	t.Parallel()

	originalPayload := []byte(`{"sensitive": "data"}`)

	tx, err := NewExTxBuilder(originalPayload, library.EthereumChainID).Build()
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
	t.Parallel()

	payload := []byte(`{"test": "chaining"}`)

	// Test that builder methods return the same instance for chaining
	builder := NewExTxBuilder(payload, library.EthereumChainID)

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

	if tx.ChainID != library.EthereumChainID {
		t.Errorf(
			"Expected ChainID %d from chained calls, got %d",
			library.EthereumChainID,
			tx.ChainID,
		)
	}
}
