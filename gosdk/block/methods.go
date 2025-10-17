package block

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// GetBlock retrieves a block by key from the specified bucket and decodes it into FieldsValues.
func GetBlock(
	tx kv.Tx,
	bucket string,
	key []byte,
	_ apptypes.AppchainBlock,
) (FieldsValues, error) {
	value, err := tx.GetOne(bucket, key)
	if err != nil {
		return FieldsValues{}, err
	}

	if len(value) == 0 {
		return FieldsValues{}, ErrNoBlocks
	}

	var b Block[apptypes.AppTransaction[apptypes.Receipt], apptypes.Receipt]
	if unmarshalErr := cbor.Unmarshal(value, &b); unmarshalErr != nil {
		return FieldsValues{}, unmarshalErr
	}

	fv, err := b.convertToFieldsValues()
	if err != nil {
		return FieldsValues{}, err
	}

	return fv, nil
}

// GetBlocks returns up to `count` most recent blocks from the BlockNumberBucket (newest first)
// and formats each block as FieldsValues (same shape as GetBlock).
// If count <= 0, it returns an empty slice. If the bucket is empty, returns ErrNoBlocks.
func GetBlocks(tx kv.Tx, count uint64) ([]FieldsValues, error) {
	if count == 0 {
		return []FieldsValues{}, nil
	}

	cur, err := tx.Cursor(BlockNumberBucket)
	if err != nil {
		return nil, err
	}
	defer cur.Close()

	k, v, err := cur.Last()
	if err != nil {
		return nil, err
	}

	if len(k) == 0 {
		return nil, ErrNoBlocks
	}

	out := make([]FieldsValues, 0, count)

	for i := uint64(0); i < count && len(k) > 0; i++ {
		var b Block[apptypes.AppTransaction[apptypes.Receipt], apptypes.Receipt]
		if unmarshalErr := cbor.Unmarshal(v, &b); unmarshalErr != nil {
			return nil, unmarshalErr
		}

		fv, err := b.convertToFieldsValues()
		if err != nil {
			return nil, err
		}

		out = append(out, fv)

		k, v, err = cur.Prev()
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// GetTransactionsByBlockNumber loads the block stored in BlockNumberBucket and returns
// a copy of its transactions decoded into the provided appTx type. When the block does
// not exist it returns ErrNoBlocks.
func GetTransactionsByBlockNumber[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	tx kv.Tx,
	number uint64,
	_ appTx,
) ([]appTx, error) {
	value, err := tx.GetOne(BlockNumberBucket, NumberToBytes(number))
	if err != nil {
		return nil, err
	}

	if len(value) == 0 {
		return nil, ErrNoBlocks
	}

	return extractTransactions[appTx](value)
}

// GetTransactionsByBlockHash retrieves the block stored in BlockHashBucket and returns a copy
// of its transactions decoded into the provided appTx type. When the block does not exist
// it returns ErrNoBlocks.
func GetTransactionsByBlockHash[appTx apptypes.AppTransaction[R], R apptypes.Receipt](
	tx kv.Tx,
	hash [32]byte,
	_ appTx,
) ([]appTx, error) {
	value, err := tx.GetOne(BlockHashBucket, hash[:])
	if err != nil {
		return nil, err
	}

	if len(value) == 0 {
		return nil, ErrNoBlocks
	}

	return extractTransactions[appTx](value)
}

// extractTransactions decodes the CBOR-encoded block payload and returns a defensive copy
// of its transactions so callers can safely mutate the slice. The helper is shared by the
// public getters and surfaces unmarshalling failures to the caller.
func extractTransactions[
	appTx apptypes.AppTransaction[R],
	R apptypes.Receipt,
](value []byte) ([]appTx, error) {
	var block Block[appTx, R]
	if err := cbor.Unmarshal(value, &block); err != nil {
		return nil, err
	}

	return append([]appTx(nil), block.Transactions...), nil
}
