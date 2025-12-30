package evmtypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/0xAtelerix/sdk/gosdk/evmtypes/fields"
)

// Transaction contains standard EVM transaction fields.
type Transaction[T fields.CustomFields] struct {
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

	Raw T `json:"-"` // Set by fetcher for GetCustomField()
}

// NewTransaction creates a new Transaction with basic fields.
func NewTransaction[T fields.CustomFields](hash common.Hash, from common.Address) *Transaction[T] {
	return &Transaction[T]{
		Hash: hash,
		From: from,
	}
}

func (t *Transaction[T]) GetCustom() T {
	return t.Raw
}

// GetCustomField extracts a chain-specific field from raw JSON.
func (t *Transaction[T]) GetCustomField(fieldName string) (any, error) {
	return GetCustomField[T](t, fieldName)
}
