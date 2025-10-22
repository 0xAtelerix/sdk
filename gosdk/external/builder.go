package external

import (
	"errors"
	"fmt"

	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/types"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
)

// Static errors for validation.
var (
	ErrChainIDRequired = errors.New("chainID must be set")
	ErrTooManyAccounts = errors.New("too many accounts: maximum 64 allowed")
	ErrNoAccountsGiven = errors.New("no accounts given")
	ErrPayloadTooShort = errors.New("payload too short")
)

// ExTxBuilder constructs external transaction intents for any chain
type ExTxBuilder struct {
	chainID  apptypes.ChainType
	payload  []byte
	accounts []types.AccountMeta // For Solana payloads
}

// NewExTxBuilder creates a new external transaction builder
func NewExTxBuilder(data []byte, chainID apptypes.ChainType) *ExTxBuilder {
	payloadCopy := make([]byte, len(data))
	copy(payloadCopy, data)

	return &ExTxBuilder{
		payload: payloadCopy,
		chainID: chainID,
	}
}

// AddSolanaAccounts adds Solana accounts to the payload builder.
// This is used for building Solana transaction payloads.
func (b *ExTxBuilder) AddSolanaAccounts(accounts []types.AccountMeta) *ExTxBuilder {
	b.accounts = accounts

	return b
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

// Build constructs the final ExternalTransaction.
func (b *ExTxBuilder) Build() (apptypes.ExternalTransaction, error) {
	if library.IsEvmChain(b.chainID) {
		return apptypes.ExternalTransaction{
			ChainID: b.chainID,
			Tx:      b.payload,
		}, nil
	}

	if library.IsSolanaChain(b.chainID) {
		return b.BuildSolanaPayload()
	}

	return apptypes.ExternalTransaction{}, ErrChainIDRequired
}

// BuildSolanaPayload builds a Solana payload with account metadata prefix.
// Format: [u8 num_accounts][[32]pubkey + u8 flags][data...]
func (b *ExTxBuilder) BuildSolanaPayload() (apptypes.ExternalTransaction, error) {
	if len(b.accounts) == 0 {
		return apptypes.ExternalTransaction{}, ErrNoAccountsGiven
	}

	if len(b.accounts) > 64 {
		return apptypes.ExternalTransaction{}, ErrTooManyAccounts
	}

	// Calculate total size: 1 (num_accounts) + len(accounts)*(32+1) + len(data)
	totalSize := 1 + len(b.accounts)*(32+1) + len(b.payload)
	payload := make([]byte, totalSize)

	offset := 0
	payload[offset] = byte(len(b.accounts))
	offset++

	// Encode each account: [32]pubkey + u8 flags
	for _, account := range b.accounts {
		copy(payload[offset:], account.PubKey[:])
		offset += 32

		var flags byte
		if account.IsSigner {
			flags |= 1
		}

		if account.IsWritable {
			flags |= 2
		}

		payload[offset] = flags
		offset++
	}

	// Append the data
	copy(payload[offset:], b.payload)

	return apptypes.ExternalTransaction{
		ChainID: b.chainID,
		Tx:      payload,
	}, nil
}

// Ethereum sets chainID to Ethereum mainnet (1)
func (b *ExTxBuilder) Ethereum() *ExTxBuilder {
	b.chainID = library.EthereumChainID

	return b
}

// EthereumSepolia sets chainID to Ethereum Sepolia testnet (11155111)
func (b *ExTxBuilder) EthereumSepolia() *ExTxBuilder {
	b.chainID = library.EthereumSepoliaChainID

	return b
}

// Polygon sets chainID to Polygon mainnet (137)
func (b *ExTxBuilder) Polygon() *ExTxBuilder {
	b.chainID = library.PolygonChainID

	return b
}

// PolygonAmoy sets chainID to Polygon Amoy testnet (80002)
func (b *ExTxBuilder) PolygonAmoy() *ExTxBuilder {
	b.chainID = library.PolygonAmoyChainID

	return b
}

// BSC sets chainID to Binance Smart Chain mainnet (56)
func (b *ExTxBuilder) BSC() *ExTxBuilder {
	b.chainID = library.BNBChainID

	return b
}

// BSCTestnet sets chainID to Binance Smart Chain testnet (97)
func (b *ExTxBuilder) BSCTestnet() *ExTxBuilder {
	b.chainID = library.BNBTestnetChainID

	return b
}

// SolanaMainnet sets chainID to Solana mainnet
func (b *ExTxBuilder) SolanaMainnet() *ExTxBuilder {
	b.chainID = library.SolanaChainID

	return b
}

// SolanaDevnet sets chainID to Solana devnet
func (b *ExTxBuilder) SolanaDevnet() *ExTxBuilder {
	b.chainID = library.SolanaDevnetChainID

	return b
}

// DecodeSolanaPayload decodes a Solana payload with account metadata prefix.
// Format: [u8 num_accounts][[32]pubkey + u8 flags][data...]
// Returns the decoded accounts and data.
func DecodeSolanaPayload(payload []byte) ([]types.AccountMeta, []byte, error) {
	if len(payload) < 1 {
		return nil, nil, fmt.Errorf("%w: missing num_accounts", ErrPayloadTooShort)
	}

	numAccounts := int(payload[0])
	offset := 1

	// Validate we have enough data for all accounts
	minAccountSize := 32 + 1 // pubkey + flags
	if len(payload) < offset+numAccounts*minAccountSize {
		return nil, nil, fmt.Errorf("%w: insufficient account data", ErrPayloadTooShort)
	}

	accounts := make([]types.AccountMeta, numAccounts)

	for i := range numAccounts {
		if offset+32+1 > len(payload) {
			return nil, nil, fmt.Errorf("%w: incomplete account data", ErrPayloadTooShort)
		}

		// Read pubkey
		var pubkey common.PublicKey
		copy(pubkey[:], payload[offset:offset+32])
		offset += 32

		// Read flags
		flags := payload[offset]
		offset++

		accounts[i] = types.AccountMeta{
			PubKey:     pubkey,
			IsSigner:   (flags & 1) != 0,
			IsWritable: (flags & 2) != 0,
		}
	}

	// Remaining data
	data := payload[offset:]

	return accounts, data, nil
}
