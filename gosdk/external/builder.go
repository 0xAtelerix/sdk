package external

import (
	"context"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// ExTxBuilder constructs external transaction intents for any chain
type ExTxBuilder struct {
	chainID uint64
	payload []byte
}

// NewExTxBuilder creates a new external transaction builder
func NewExTxBuilder() *ExTxBuilder {
	return &ExTxBuilder{
		payload: []byte{},
	}
}

// SetChainID sets the target chain ID
func (b *ExTxBuilder) SetChainID(chainID uint64) *ExTxBuilder {
	b.chainID = chainID
	return b
}

// SetPayload sets the payload data (appchain-controlled encoding)
func (b *ExTxBuilder) SetPayload(payload []byte) *ExTxBuilder {
	b.payload = make([]byte, len(payload))
	copy(b.payload, payload)
	return b
}

// Build creates the external transaction
func (b *ExTxBuilder) Build(_ context.Context) (*apptypes.ExternalTransaction, error) {
	if b.chainID == 0 {
		return nil, ErrChainIDRequired
	}

	return &apptypes.ExternalTransaction{
		ChainID: b.chainID,
		Tx:      b.payload,
	}, nil
}

// === HELPER METHODS FOR COMMON CHAINS ===

// Ethereum sets chainID to Ethereum mainnet (1)
func (b *ExTxBuilder) Ethereum() *ExTxBuilder {
	b.chainID = 1
	return b
}

// EthereumSepolia sets chainID to Ethereum Sepolia testnet (11155111)
func (b *ExTxBuilder) EthereumSepolia() *ExTxBuilder {
	b.chainID = 11155111
	return b
}

// Polygon sets chainID to Polygon mainnet (137)
func (b *ExTxBuilder) Polygon() *ExTxBuilder {
	b.chainID = 137
	return b
}

// PolygonAmoy sets chainID to Polygon Amoy testnet (80002)
func (b *ExTxBuilder) PolygonAmoy() *ExTxBuilder {
	b.chainID = 80002
	return b
}

// BSC sets chainID to Binance Smart Chain mainnet (56)
func (b *ExTxBuilder) BSC() *ExTxBuilder {
	b.chainID = 56
	return b
}

// BSCTestnet sets chainID to Binance Smart Chain testnet (97)
func (b *ExTxBuilder) BSCTestnet() *ExTxBuilder {
	b.chainID = 97
	return b
}

// SolanaMainnet sets chainID to Solana mainnet (900)
func (b *ExTxBuilder) SolanaMainnet() *ExTxBuilder {
	b.chainID = 900
	return b
}

// SolanaDevnet sets chainID to Solana devnet (901)
func (b *ExTxBuilder) SolanaDevnet() *ExTxBuilder {
	b.chainID = 901
	return b
}
