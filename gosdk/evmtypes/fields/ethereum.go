package fields

import (
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/common"

	"github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	// ErrFieldNotFound is returned when a custom field is not found in the raw JSON.
	ErrFieldNotFound    = errors.SDKError("field not found")
	ErrEmptyCustomField = errors.SDKError("raw custom field is empty")
)

type EthereumCustomFields struct {
	WithdrawalsRoot       *common.Hash `json:"withdrawalsRoot,omitempty" rlp:"optional"`
	BlobGasUsed           *uint64      `json:"blobGasUsed,omitempty" rlp:"optional"`
	ExcessBlobGas         *uint64      `json:"excessBlobGas,omitempty" rlp:"optional"`
	ParentBeaconBlockRoot *common.Hash `json:"parentBeaconBlockRoot,omitempty" rlp:"optional"`
	RequestsHash          *common.Hash `json:"requestsHash,omitempty" rlp:"optional"`
}

func (e *EthereumCustomFields) GetField(name string) (any, error) {
	if e == nil {
		return nil, fmt.Errorf("%w, field %q", ErrEmptyCustomField, name)
	}

	f := reflect.ValueOf(e).
		Elem().
		FieldByName(name)

	if !f.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrFieldNotFound, name)
	}

	if f.Kind() == reflect.Ptr && f.IsNil() {
		return nil, nil
	}

	return f.Interface(), nil
}
