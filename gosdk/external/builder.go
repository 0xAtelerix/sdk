package external

import (
	"errors"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Static errors for validation.
var (
	ErrChainIDRequired = errors.New("chainID must be set")
)

// ExTxBuilder constructs external transaction intents for any chain
type ExTxBuilder struct {
	chainID apptypes.ChainType
	payload []byte
}

// NewExTxBuilder creates a new external transaction builder
func NewExTxBuilder() *ExTxBuilder {
	return &ExTxBuilder{
		payload: []byte{},
	}
}

// SetChainID sets the target chain ID
func (b *ExTxBuilder) SetChainID(chainID apptypes.ChainType) *ExTxBuilder {
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
func (b *ExTxBuilder) Build() (apptypes.ExternalTransaction, error) {
	if b.chainID == 0 {
		return apptypes.ExternalTransaction{}, ErrChainIDRequired
	}

	return apptypes.ExternalTransaction{
		ChainID: b.chainID,
		Tx:      b.payload,
	}, nil
}

// === HELPER METHODS FOR COMMON CHAINS ===

// Ethereum sets chainID to Ethereum mainnet (1)
func (b *ExTxBuilder) Ethereum() *ExTxBuilder {
	b.chainID = gosdk.EthereumChainID

	return b
}

// EthereumSepolia sets chainID to Ethereum Sepolia testnet (11155111)
func (b *ExTxBuilder) EthereumSepolia() *ExTxBuilder {
	b.chainID = gosdk.EthereumSepoliaChainID

	return b
}

// Polygon sets chainID to Polygon mainnet (137)
func (b *ExTxBuilder) Polygon() *ExTxBuilder {
	b.chainID = gosdk.PolygonChainID

	return b
}

// PolygonAmoy sets chainID to Polygon Amoy testnet (80002)
func (b *ExTxBuilder) PolygonAmoy() *ExTxBuilder {
	b.chainID = gosdk.PolygonAmoyChainID

	return b
}

// BSC sets chainID to Binance Smart Chain mainnet (56)
func (b *ExTxBuilder) BSC() *ExTxBuilder {
	b.chainID = gosdk.BNBChainID

	return b
}

// BSCTestnet sets chainID to Binance Smart Chain testnet (97)
func (b *ExTxBuilder) BSCTestnet() *ExTxBuilder {
	b.chainID = gosdk.BNBTestnetChainID

	return b
}

// SolanaMainnet sets chainID to Solana mainnet
func (b *ExTxBuilder) SolanaMainnet() *ExTxBuilder {
	b.chainID = gosdk.SolanaChainID

	return b
}

// SolanaDevnet sets chainID to Solana devnet
func (b *ExTxBuilder) SolanaDevnet() *ExTxBuilder {
	b.chainID = gosdk.SolanaDevnetChainID

	return b
}
