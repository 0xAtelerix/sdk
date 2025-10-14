package external

import (
	"errors"
	"fmt"

	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/types"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Static errors for validation.
var (
	ErrChainIDRequired = errors.New("chainID must be set")
	ErrTooManyAccounts = errors.New("too many accounts: maximum 64 allowed")
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

// Build constructs the final ExternalTransaction.
func (b *ExTxBuilder) Build() (apptypes.ExternalTransaction, error) {
	if b.chainID == 0 {
		return apptypes.ExternalTransaction{}, ErrChainIDRequired
	}

	if b.accounts != nil {
		return b.BuildSolanaPayload()
	}

	return apptypes.ExternalTransaction{
		ChainID: b.chainID,
		Tx:      b.payload,
	}, nil
}

// BuildSolanaPayload builds a Solana payload with account metadata prefix.
// Format: [u8 num_accounts][[32]pubkey + u8 flags][data...]
func (b *ExTxBuilder) BuildSolanaPayload() (apptypes.ExternalTransaction, error) {
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
