package evmtypes

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goccy/go-json"
)

// Body contains the body fields of a block (transactions, uncles).
type Body struct {
	Transactions []Transaction `json:"transactions"`
	Uncles       []common.Hash `json:"uncles,omitempty"`
}

// Block contains standard EVM block fields.
type Block[T CustomFields] struct {
	Header[T]
	Body

	Raw T `json:"-"` // Set by fetcher for GetCustomField()
}

// NewBlock creates a new Block with the given header and transactions.
func NewBlock[T CustomFields](header *Header[T], transactions []Transaction) *Block[T] {
	if transactions == nil {
		transactions = []Transaction{}
	}

	h := Header[T]{}
	if header != nil {
		h = *header
	}

	return &Block[T]{
		Header: h,
		Body:   Body{Transactions: transactions},
	}
}

func (b *Block[T]) GetCustom() T {
	return b.Raw
}

// GetCustomField extracts a chain-specific field from raw JSON.
func (b *Block[T]) GetCustomField(fieldName string) (any, error) {
	return GetCustomField[T](b, fieldName)
}

type RawField[T CustomFields] interface {
	GetCustom() T
}

func GetCustomField[T CustomFields](rawFieldGetter RawField[T], fieldName string) (any, error) {
	raw := rawFieldGetter.GetCustom()

	switch field := any(raw).(type) {
	case EthereumCustomFields:
		return field.GetField(fieldName)
	case json.RawMessage:
		return GetCustomFieldFromRaw(field, fieldName)
	default:
		return nil, fmt.Errorf("unknown custom field type: %T", field)
	}
}
