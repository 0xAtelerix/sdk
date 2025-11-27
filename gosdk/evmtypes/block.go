package evmtypes

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// Body contains the body fields of a block (transactions, uncles).
type Body struct {
	Transactions []Transaction `json:"transactions"`
	Uncles       []common.Hash `json:"uncles,omitempty"`
}

// Block contains standard EVM block fields.
type Block struct {
	Header
	Body

	Raw json.RawMessage `json:"-"` // Set by fetcher for GetCustomField()
}

// NewBlock creates a new Block with the given header and transactions.
func NewBlock(header *Header, transactions []Transaction) *Block {
	if transactions == nil {
		transactions = []Transaction{}
	}

	h := Header{}
	if header != nil {
		h = *header
	}

	return &Block{
		Header: h,
		Body:   Body{Transactions: transactions},
	}
}

// GetCustomField extracts a chain-specific field from raw JSON.
func (b *Block) GetCustomField(fieldName string) (any, error) {
	if b.Raw == nil {
		return nil, ErrRawJSONNotAvailable
	}

	var data map[string]any
	if err := json.Unmarshal(b.Raw, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw block: %w", err)
	}

	value, exists := data[fieldName]
	if !exists {
		return nil, fmt.Errorf("%w: %s in block", ErrFieldNotFound, fieldName)
	}

	return value, nil
}
