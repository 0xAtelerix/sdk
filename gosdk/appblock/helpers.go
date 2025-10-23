package appblock

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
)

// CloneTarget returns a new zero-valued instance of the provided pointer
// template, ensuring the original template remains untouched.
func CloneTarget[T any](tpl T) (T, error) {
	var zero T

	val := reflect.ValueOf(tpl)
	if !val.IsValid() {
		return zero, ErrMissingBlockTemplate
	}

	typ := val.Type()
	if typ.Kind() != reflect.Pointer {
		return zero, fmt.Errorf("%w: %T", ErrUnsupportedPayload, tpl)
	}

	newVal := reflect.New(typ.Elem())

	result, ok := newVal.Interface().(T)
	if !ok {
		return zero, fmt.Errorf("%w: %T", ErrUnsupportedPayload, tpl)
	}

	return result, nil
}

// structValueFrom unwraps pointer chains on target and returns the underlying
// struct value if present, signalling success via the second return value.
func structValueFrom(target any) (reflect.Value, bool) {
	value := reflect.ValueOf(target)
	if !value.IsValid() {
		return reflect.Value{}, false
	}

	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}

		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	return value, true
}

// txsField returns the exported `Txs` slice field from the supplied struct value,
// if it exists and is a slice; otherwise it reports false.
func txsField(value reflect.Value) (reflect.Value, bool) {
	field := value.FieldByName("Txs")
	if !field.IsValid() || field.Kind() != reflect.Slice {
		return reflect.Value{}, false
	}

	return field, true
}

// extractTransactions returns a copy of the target's exported Txs field if it is
// a slice compatible with AppTx. The returned bool indicates whether the field
// existed and matched the expected type, and the second bool reports whether the
// field was nil.
func extractTransactions[AppTx any](target any) (txs []AppTx, hasField bool, fieldNil bool) {
	structValue, ok := structValueFrom(target)
	if !ok {
		return nil, false, false
	}

	field, ok := txsField(structValue)
	if !ok || !field.CanInterface() {
		return nil, false, false
	}

	var zero []AppTx

	expectedType := reflect.TypeOf(zero)
	if expectedType == nil || !field.Type().AssignableTo(expectedType) {
		return nil, false, false
	}

	slice, ok := field.Interface().([]AppTx)
	if !ok {
		return nil, false, false
	}

	return append([]AppTx(nil), slice...), true, field.IsNil()
}

// unmarshallIntoTarget decodes the CBOR payload into the provided pointer
// target, validating that the target is a non-nil pointer before populating it.
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

// decodeBlockIntoTarget loads the block payload from storage and decodes it into the
// supplied target, preserving the same validation guarantees as unmarshallIntoTarget.
func decodeBlockIntoTarget[T any](
	ctx context.Context,
	db kv.RwDB,
	blockNumber uint64,
	target T,
) error {
	payload, err := LoadBlockPayload(ctx, db, blockNumber)
	if err != nil {
		return err
	}

	return unmarshallIntoTarget(payload, target)
}

// storeCBORValue encodes the provided value with CBOR and stores it in the
// designated bucket under the block number key.
func storeCBORValue(
	ctx context.Context,
	db kv.RwDB,
	bucket string,
	blockNumber uint64,
	value any,
	encodeErrPrefix string,
) error {
	encoded, err := cbor.Marshal(value)
	if err != nil {
		return fmt.Errorf("%s: %w", encodeErrPrefix, err)
	}

	return db.Update(ctx, func(tx kv.RwTx) error {
		return tx.Put(bucket, numberToBytes(blockNumber), encoded)
	})
}

// fetchBucketValue reads the encoded value for the given block number from gosdk.BlocksBucket, copying the data to avoid holding references to the
// database buffer.
func fetchBucketValue(
	ctx context.Context,
	db kv.RwDB,
	blockNumber uint64,
) ([]byte, error) {
	if db == nil {
		return nil, errAppchainDatabase
	}

	var encoded []byte

	err := db.View(ctx, func(tx kv.Tx) error {
		data, getErr := tx.GetOne(gosdk.BlocksBucket, numberToBytes(blockNumber))
		if getErr != nil {
			return getErr
		}

		if len(data) == 0 {
			return errBlockNotFound
		}

		encoded = append([]byte(nil), data...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return encoded, nil
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

	return storeCBORValue(ctx, db, gosdk.BlocksBucket, blockNumber, block, "encode app block")
}

// LoadBlockPayload fetches the raw encoded block payload for the supplied block
// number from the appchain database.
func LoadBlockPayload(
	ctx context.Context,
	db kv.RwDB,
	blockNumber uint64,
) ([]byte, error) {
	payload, err := fetchBucketValue(ctx, db, blockNumber)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

// numberToBytes converts a uint64 block number into its big-endian byte slice
// representation for use as a storage key.
func numberToBytes(input uint64) []byte {
	// Create a byte slice of length 8, as uint64 occupies 8 bytes
	b := make([]byte, 8)

	// Encode the uint64 into the byte slice using Big Endian byte order
	binary.BigEndian.PutUint64(b, input)

	return b
}
