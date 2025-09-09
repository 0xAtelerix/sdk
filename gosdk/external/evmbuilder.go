package external

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// EVMTxBuilder constructs EVM-compatible external transaction intents
type EVMTxBuilder struct {
	chainID uint64
	to      string
	value   *big.Int
	data    []byte
}

func NewEVMTxBuilder() *EVMTxBuilder {
	return &EVMTxBuilder{
		value: big.NewInt(0),
		data:  []byte{},
	}
}

func (b *EVMTxBuilder) SetChainID(chainID uint64) ExTxBuilder {
	b.chainID = chainID

	return b
}

func (b *EVMTxBuilder) SetTo(address string) ExTxBuilder {
	b.to = address

	return b
}

func (b *EVMTxBuilder) SetValue(value *big.Int) ExTxBuilder {
	if value != nil {
		b.value = new(big.Int).Set(value)
	}

	return b
}

func (b *EVMTxBuilder) SetData(data []byte) ExTxBuilder {
	b.data = make([]byte, len(data))
	copy(b.data, data)

	return b
}

func (b *EVMTxBuilder) Build(_ context.Context) (*apptypes.ExternalTransaction, error) {
	if b.chainID == 0 {
		return nil, ErrChainIDRequired
	}

	// Create intent structure
	intent := BaseTxIntent{
		ChainID: b.chainID,
		To:      b.to,
		Value:   b.value.String(),
		Data:    fmt.Sprintf("0x%x", b.data),
	}

	intentBytes, err := json.Marshal(intent)
	if err != nil {
		return nil, fmt.Errorf("failed to encode EVM intent: %w", err)
	}

	return &apptypes.ExternalTransaction{
		ChainID: b.chainID,
		Tx:      intentBytes,
	}, nil
}

// Helper functions for common EVM operations

// SetValueEther sets value in ether (converts to wei)
func (b *EVMTxBuilder) SetValueEther(ether float64) *EVMTxBuilder {
	wei := new(big.Float).Mul(big.NewFloat(ether), big.NewFloat(1e18))
	weiInt, _ := wei.Int(nil)
	b.value = weiInt

	return b
}

// SetValueWei sets value in wei
func (b *EVMTxBuilder) SetValueWei(wei *big.Int) *EVMTxBuilder {
	if wei != nil {
		b.value = new(big.Int).Set(wei)
	}

	return b
}

// Common EVM chain configurations
// TODO : Will update with more chains
func (b *EVMTxBuilder) Ethereum() *EVMTxBuilder {
	b.SetChainID(1)

	return b
}

func (b *EVMTxBuilder) Polygon() *EVMTxBuilder {
	b.SetChainID(137)

	return b
}
