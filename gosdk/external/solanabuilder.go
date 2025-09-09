package external

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// SolanaTxBuilder constructs Solana-compatible external transaction intents
type SolanaTxBuilder struct {
	chainID uint64
	to      string
	value   *big.Int
	data    []byte
}

func NewSolanaTxBuilder() *SolanaTxBuilder {
	return &SolanaTxBuilder{
		value: big.NewInt(0),
		data:  []byte{},
	}
}

func (b *SolanaTxBuilder) SetChainID(chainID uint64) ExternalTxBuilder {
	b.chainID = chainID
	return b
}

func (b *SolanaTxBuilder) SetTo(address string) ExternalTxBuilder {
	b.to = address
	return b
}

func (b *SolanaTxBuilder) SetValue(value *big.Int) ExternalTxBuilder {
	if value != nil {
		b.value = new(big.Int).Set(value)
	}
	return b
}

func (b *SolanaTxBuilder) SetData(data []byte) ExternalTxBuilder {
	b.data = make([]byte, len(data))
	copy(b.data, data)
	return b
}

func (b *SolanaTxBuilder) Build(ctx context.Context) (*apptypes.ExternalTransaction, error) {
	if b.chainID == 0 {
		return nil, fmt.Errorf("chainID must be set")
	}

	if b.to == "" {
		return nil, fmt.Errorf("to address must be set")
	}

	// Create intent structure
	intent := BaseTxIntent{
		ChainID: b.chainID,
		To:      b.to,
		Value:   b.value.String(), // Lamports
		Data:    fmt.Sprintf("0x%x", b.data),
	}

	// Encode intent to bytes
	intentBytes, err := json.Marshal(intent)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Solana intent: %w", err)
	}

	return &apptypes.ExternalTransaction{
		ChainID: b.chainID,
		Tx:      intentBytes,
	}, nil
}

// Helper functions for common Solana operations

// SetValueSOL sets value in SOL (converts to lamports)
func (b *SolanaTxBuilder) SetValueSOL(sol float64) *SolanaTxBuilder {
	lamports := new(big.Float).Mul(big.NewFloat(sol), big.NewFloat(1e9)) // 1 SOL = 1e9 lamports
	lamportsInt, _ := lamports.Int(nil)
	b.value = lamportsInt
	return b
}

// SetValueLamports sets value in lamports
func (b *SolanaTxBuilder) SetValueLamports(lamports *big.Int) *SolanaTxBuilder {
	if lamports != nil {
		b.value = new(big.Int).Set(lamports)
	}
	return b
}
