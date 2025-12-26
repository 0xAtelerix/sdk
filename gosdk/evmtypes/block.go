package evmtypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/goccy/go-json"
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
	return GetCustomFieldFromRaw(b.Raw, fieldName)
}
