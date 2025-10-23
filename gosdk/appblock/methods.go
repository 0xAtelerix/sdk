package appblock

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var (
	errTargetNil           = errors.New("target cannot be nil")
	errTargetNotPointer    = errors.New("target must be a pointer")
	errTargetNilPointer    = errors.New("target must be a non-nil pointer")
	errBlockPayloadEmpty   = errors.New("block payload is empty")
	errAppchainDatabase    = errors.New("appchain database cannot be nil")
	errAppBlockValueNil    = errors.New("block cannot be nil")
	errBlockNotFound       = errors.New("block not found")
	errTransactionsMissing = errors.New("block does not store transactions in payload")
)

// GetAppBlockByNumber decodes a CBOR-encoded payload into target and returns its fields and values.
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
	payload, err := fetchBucketValue(ctx, db, gosdk.BlocksBucket, blockNumber)
	if err != nil {
		return nil, err
	}

	return payload, nil
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

// GetTransactionsFromBlock hydrates the provided target template from the block
// payload stored for blockNumber and extracts transactions via the target's Txs
// field when present. It falls back to the block transactions bucket and
// distinguishes between missing data and an explicitly nil Txs field.
func GetTransactionsFromBlock[appTx apptypes.AppTransaction[R], R apptypes.Receipt, T any](
	ctx context.Context,
	db kv.RwDB,
	blockNumber uint64,
	rootTemplate T,
) ([]appTx, bool, error) {
	target, err := CloneTarget(rootTemplate)
	if err != nil {
		return nil, false, err
	}

	if err := decodeBlockIntoTarget(ctx, db, blockNumber, target); err != nil {
		return nil, false, err
	}

	if txs, ok, wasNil := ExtractTransactions[appTx](target); ok {
		if wasNil {
			return make([]appTx, 0), true, nil
		}

		return txs, true, nil
	}

	return nil, false, errTransactionsMissing
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

// fetchBucketValue reads the encoded value for the given block number from the
// requested bucket, copying the data to avoid holding references to the
// database buffer.
func fetchBucketValue(
	ctx context.Context,
	db kv.RwDB,
	bucket string,
	blockNumber uint64,
) ([]byte, error) {
	if db == nil {
		return nil, errAppchainDatabase
	}

	var encoded []byte

	err := db.View(ctx, func(tx kv.Tx) error {
		data, getErr := tx.GetOne(bucket, numberToBytes(blockNumber))
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
