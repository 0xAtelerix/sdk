package appblock

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
)

var (
	errTargetNil         = errors.New("target cannot be nil")
	errTargetNotPointer  = errors.New("target must be a pointer")
	errTargetNilPointer  = errors.New("target must be a non-nil pointer")
	errBlockPayloadEmpty = errors.New("block payload is empty")
	errAppchainDatabase  = errors.New("appchain database cannot be nil")
	errAppBlockValueNil  = errors.New("block cannot be nil")
)

// GetAppBlockByNumber decodes a CBOR-encoded payload into target and returns its fields.
func GetAppBlockByNumber[T any](
	blockNumber uint64,
	payload []byte,
	target T,
) (FieldsValues, error) {
	if err := unmarshallIntoTarget(payload, target); err != nil {
		return FieldsValues{}, err
	}

	cb := NewAppBlock(blockNumber, target)

	return cb.ToFieldsAndValues(), nil
}

func unmarshallIntoTarget[T any](encoded []byte, target T) error {
	value := reflect.ValueOf(target)
	if !value.IsValid() {
		return errTargetNil
	}

	if value.Kind() != reflect.Pointer {
		return fmt.Errorf("%w: %T", errTargetNotPointer, target)
	}

	if value.IsNil() {
		return errTargetNilPointer
	}

	if len(encoded) == 0 {
		return errBlockPayloadEmpty
	}

	if err := cbor.Unmarshal(encoded, value.Interface()); err != nil {
		return fmt.Errorf("decode block payload: %w", err)
	}

	return nil
}

// StoreAppBlock encodes the provided block value and writes it to the appchain database.
// This helper is primarily intended for tests that need to seed the BlocksBucket.
func StoreAppBlock(ctx context.Context, db kv.RwDB, blockNumber uint64, block any) error {
	if db == nil {
		return errAppchainDatabase
	}

	if block == nil {
		return errAppBlockValueNil
	}

	encoded, err := cbor.Marshal(block)
	if err != nil {
		return fmt.Errorf("encode app block: %w", err)
	}

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNumber)

	return db.Update(ctx, func(tx kv.RwTx) error {
		return tx.Put(gosdk.BlocksBucket, key, encoded)
	})
}
