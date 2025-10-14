package block

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// GetBlock loads a block
// TODO consider to pass third argument of type apptypes.AppchainBlock to decode into concrete type
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
	if err := cbor.Unmarshal(value, &b); err != nil {
		return FieldsValues{}, err
	}

	return b.convertToFieldsValues(), nil
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

		out = append(out, b.convertToFieldsValues())

		k, v, err = cur.Prev()
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}
