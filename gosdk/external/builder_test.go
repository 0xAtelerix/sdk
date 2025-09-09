package external

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
)

func TestEVMTxBuilder(t *testing.T) {
	ctx := context.Background()

	// Test basic EVM transaction
	extTx, err := NewEVMTxBuilder().
		SetChainID(1).
		SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
		SetValue(big.NewInt(1000000000000000000)). // 1 ETH
		Build(ctx)
	if err != nil {
		t.Fatalf("Failed to build EVM transaction: %v", err)
	}

	if extTx.ChainID != 1 {
		t.Errorf("Expected ChainID 1, got %d", extTx.ChainID)
	}

	// Verify the transaction data can be unmarshaled
	var intent BaseTxIntent

	err = json.Unmarshal(extTx.Tx, &intent)
	if err != nil {
		t.Fatalf("Failed to unmarshal transaction data: %v", err)
	}

	if intent.ChainID != 1 {
		t.Errorf("Expected intent ChainID 1, got %d", intent.ChainID)
	}

	if intent.To != "0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264" {
		t.Errorf(
			"Expected to address 0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264, got %s",
			intent.To,
		)
	}

	if intent.Value != "1000000000000000000" {
		t.Errorf("Expected value 1000000000000000000, got %s", intent.Value)
	}
}

func TestSolanaTxBuilder(t *testing.T) {
	ctx := context.Background()

	// Test basic Solana transaction
	extTx, err := NewSolanaTxBuilder().
		SetChainID(101).
		SetTo("11111111111111111111111111111112").
		SetValue(big.NewInt(1000000000)). // 1 SOL
		Build(ctx)
	if err != nil {
		t.Fatalf("Failed to build Solana transaction: %v", err)
	}

	if extTx.ChainID != 101 {
		t.Errorf("Expected ChainID 101, got %d", extTx.ChainID)
	}

	// Verify the transaction data can be unmarshaled
	var intent BaseTxIntent

	err = json.Unmarshal(extTx.Tx, &intent)
	if err != nil {
		t.Fatalf("Failed to unmarshal transaction data: %v", err)
	}

	if intent.ChainID != 101 {
		t.Errorf("Expected intent ChainID 101, got %d", intent.ChainID)
	}

	if intent.To != "11111111111111111111111111111112" {
		t.Errorf("Expected to address 11111111111111111111111111111112, got %s", intent.To)
	}

	if intent.Value != "1000000000" {
		t.Errorf("Expected value 1000000000, got %s", intent.Value)
	}
}

func TestEVMHelperMethods(t *testing.T) {
	builder := NewEVMTxBuilder()

	// Test SetValueEther
	builder.SetValueEther(1.5)

	expected := new(big.Int).Mul(big.NewInt(15), big.NewInt(1e17)) // 1.5 ETH in wei
	if builder.value.Cmp(expected) != 0 {
		t.Errorf("SetValueEther: expected %s, got %s", expected.String(), builder.value.String())
	}

	// Test chain configurations
	ethBuilder := builder.Ethereum()

	if ethBuilder.chainID != 1 {
		t.Errorf("Ethereum(): expected chainID 1, got %d", ethBuilder.chainID)
	}

	polygonBuilder := NewEVMTxBuilder().Polygon()

	if polygonBuilder.chainID != 137 {
		t.Errorf("Polygon(): expected chainID 137, got %d", polygonBuilder.chainID)
	}
}

func TestSolanaHelperMethods(t *testing.T) {
	builder := NewSolanaTxBuilder()

	// Test SetValueSOL
	builder.SetValueSOL(1.5)

	expected := new(big.Int).Mul(big.NewInt(15), big.NewInt(1e8)) // 1.5 SOL in lamports
	if builder.value.Cmp(expected) != 0 {
		t.Errorf("SetValueSOL: expected %s, got %s", expected.String(), builder.value.String())
	}

	// Test manual chain ID setting (no more network helper methods)
	builder.SetChainID(101) // Mainnet

	if builder.chainID != 101 {
		t.Errorf("SetChainID(101): expected chainID 101, got %d", builder.chainID)
	}

	testnetBuilder := NewSolanaTxBuilder()
	testnetBuilder.SetChainID(102) // Testnet

	if testnetBuilder.chainID != 102 {
		t.Errorf("SetChainID(102): expected chainID 102, got %d", testnetBuilder.chainID)
	}
}

func TestFactoryFunction(t *testing.T) {
	ctx := context.Background()

	// Test EVM builder creation
	evmBuilder := NewExternalTxBuilder(ChainTypeEVM)

	extTx, err := evmBuilder.
		SetChainID(1).
		SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
		SetValue(big.NewInt(1000000000000000000)).
		Build(ctx)
	if err != nil {
		t.Fatalf("Failed to build EVM transaction via factory: %v", err)
	}

	if extTx.ChainID != 1 {
		t.Errorf("Expected ChainID 1, got %d", extTx.ChainID)
	}

	// Test Solana builder creation
	solanaBuilder := NewExternalTxBuilder(ChainTypeSolana)

	extTx2, err := solanaBuilder.
		SetChainID(101).
		SetTo("11111111111111111111111111111112").
		SetValue(big.NewInt(1000000000)).
		Build(ctx)
	if err != nil {
		t.Fatalf("Failed to build Solana transaction via factory: %v", err)
	}

	if extTx2.ChainID != 101 {
		t.Errorf("Expected ChainID 101, got %d", extTx2.ChainID)
	}
}

func TestValidationErrors(t *testing.T) {
	ctx := context.Background()

	// Test missing chainID
	_, err := NewEVMTxBuilder().
		SetTo("0x742d35Cc8AAbc38b9b5d1c16e785b2Ce6b8E7264").
		SetValue(big.NewInt(1000000000000000000)).
		Build(ctx)
	if err == nil {
		t.Error("Expected error for missing chainID")
	}

	// Test that empty 'to' address is allowed for contract deployment
	extTx, err := NewEVMTxBuilder().
		SetChainID(1).
		SetValue(big.NewInt(0)).
		SetData([]byte{0x60, 0x80, 0x60, 0x40}). // Contract bytecode
		Build(ctx)
	if err != nil {
		t.Errorf("Expected contract deployment to work with empty 'to', got error: %v", err)
	}

	if extTx == nil {
		t.Error("Expected successful contract deployment transaction")
	}
}

func TestChainTypeDetection(t *testing.T) {
	tests := []struct {
		chainID  uint64
		expected ChainType
		desc     string
	}{
		{1, ChainTypeEVM, "Ethereum"},
		{137, ChainTypeEVM, "Polygon"},
		{101, ChainTypeSolana, "Solana mainnet"},
		{102, ChainTypeSolana, "Solana testnet"},
		{103, ChainTypeSolana, "Solana devnet"},
		{42161, ChainTypeEVM, "Arbitrum"},
		{10, ChainTypeEVM, "Optimism"},
		{56, ChainTypeEVM, "BSC"},
	}

	for _, test := range tests {
		result := GetChainType(test.chainID)
		if result != test.expected {
			t.Errorf(
				"%s (ChainID %d): expected %d, got %d",
				test.desc, test.chainID, test.expected, result,
			)
		}
	}
}
