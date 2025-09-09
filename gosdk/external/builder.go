package external

import (
	"context"
	"errors"
	"math/big"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Static errors for validation.
var (
	ErrChainIDRequired = errors.New("chainID must be set")
	ErrAddressRequired = errors.New("to address must be set")
)

// ExTxBuilder provides a unified interface for building external transaction intents
type ExTxBuilder interface {
	// Chain configuration
	SetChainID(chainID uint64) ExTxBuilder

	// Transaction parameters (intent data)
	SetTo(address string) ExTxBuilder
	SetValue(value *big.Int) ExTxBuilder
	SetData(data []byte) ExTxBuilder

	// Build the external transaction intent (no signer needed - TSS handles it)
	Build(ctx context.Context) (*apptypes.ExternalTransaction, error)
}

// BaseTxIntent represents the common transaction intent structure for EVM and Solana
type BaseTxIntent struct {
	ChainID uint64 `json:"chainId"`
	To      string `json:"to"`    // Address/Public key
	Value   string `json:"value"` // Wei/Lamports as string
	Data    string `json:"data"`  // Hex encoded transaction data
}

// Factory function to create chain-specific builders
func NewExternalTxBuilder(chainType ChainType) ExTxBuilder {
	switch chainType {
	case ChainTypeEVM:
		return NewEVMTxBuilder()
	case ChainTypeSolana:
		return NewSolanaTxBuilder()
	default:
		return NewEVMTxBuilder() // Default to EVM
	}
}

type ChainType uint8

const (
	ChainTypeEVM ChainType = iota
	ChainTypeSolana
)

// Helper function to determine chain type from chainID
func GetChainType(chainID uint64) ChainType {
	switch chainID {
	case 101, 102, 103: // Solana mainnet, testnet, devnet
		return ChainTypeSolana
	default:
		return ChainTypeEVM // All other chains are EVM-compatible
	}
}
