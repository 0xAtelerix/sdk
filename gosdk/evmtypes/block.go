package evmtypes

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goccy/go-json"

	"github.com/0xAtelerix/sdk/gosdk/evmtypes/fields"
)

// Body contains the body fields of a block (transactions, uncles).
type Body[T fields.CustomFields] struct {
	Transactions []Transaction[T] `json:"transactions"`
	Uncles       []common.Hash    `json:"uncles,omitempty"`
}

// Block contains standard EVM block fields.
type Block[T fields.CustomFields] struct {
	Header[T]
	Body[T]

	Raw T `json:"-"` // Set by fetcher for GetCustomField()
}

// NewBlock creates a new Block with the given header and transactions.
func NewBlock[T fields.CustomFields](header *Header[T], transactions []Transaction[T]) *Block[T] {
	if transactions == nil {
		transactions = []Transaction[T]{}
	}

	h := Header[T]{}
	if header != nil {
		h = *header
	}

	return &Block[T]{
		Header: h,
		Body:   Body[T]{Transactions: transactions},
	}
}

func (b *Block[T]) GetCustom() T {
	return b.Raw
}

// GetCustomField extracts a chain-specific field from raw JSON.
func (b *Block[T]) GetCustomField(fieldName string) (any, error) {
	return GetCustomField[T](b, fieldName)
}

type RawField[T fields.CustomFields] interface {
	GetCustom() T
}

func GetCustomField[T fields.CustomFields](rawFieldGetter RawField[T], fieldName string) (any, error) {
	raw := rawFieldGetter.GetCustom()

	switch field := any(raw).(type) {
	case fields.EthereumCustomFields:
		return field.GetField(fieldName)
	case json.RawMessage:
		return GetCustomFieldFromRaw(field, fieldName)
	default:
		return nil, fmt.Errorf("unknown custom field type: %T", field)
	}
}
