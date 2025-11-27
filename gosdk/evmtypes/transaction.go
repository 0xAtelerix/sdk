package evmtypes

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Transaction contains standard EVM transaction fields.
type Transaction struct {
	Hash     common.Hash     `json:"hash"`
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Value    *hexutil.Big    `json:"value"`
	Input    hexutil.Bytes   `json:"input"`
	Nonce    hexutil.Uint64  `json:"nonce"`
	Gas      hexutil.Uint64  `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice,omitempty"`

	Type                 *hexutil.Uint64 `json:"type,omitempty"`                 // EIP-1559
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas,omitempty"`         // EIP-1559
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas,omitempty"` // EIP-1559

	Raw json.RawMessage `json:"-"` // Set by fetcher for GetCustomField()
}

// NewTransaction creates a new Transaction with basic fields.
func NewTransaction(hash common.Hash, from common.Address) *Transaction {
	return &Transaction{
		Hash: hash,
		From: from,
	}
}

// GetCustomField extracts a chain-specific field from raw JSON.
func (t *Transaction) GetCustomField(fieldName string) (any, error) {
	if t.Raw == nil {
		return nil, ErrRawJSONNotAvailable
	}

	var data map[string]any
	if err := json.Unmarshal(t.Raw, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw transaction: %w", err)
	}

	value, exists := data[fieldName]
	if !exists {
		return nil, fmt.Errorf("%w: %s in transaction", ErrFieldNotFound, fieldName)
	}

	return value, nil
}
