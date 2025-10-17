package chainblock

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// ErrNoBlocks signals that the requested bucket does not contain any chain blocks.
var ErrNoBlocks = errors.New("no chain blocks found")

// ErrChainTypeMismatch indicates stored payload has a different chain type than requested.
var ErrChainTypeMismatch = errors.New("stored chain type mismatch")

type storedChainBlock struct {
	ChainType apptypes.ChainType `cbor:"1,keyasint"`
	Block     []byte             `cbor:"2,keyasint"`
}

// GetBlock retrieves a chain block encoded under `key` in the provided bucket and converts it
// into a tabular FieldsValues representation. The shape mirrors GetBlock from the block package.
func GetChainBlock(
	tx kv.Tx,
	bucket string,
	chainType apptypes.ChainType,
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

	cb, err := decodeStoredChainBlock(value, chainType)
	if err != nil {
		return FieldsValues{}, err
	}

	return cb.convertToFieldsValues(), nil
}

// GetChainBlocks walks the specified bucket from newest to oldest (based on key ordering) and returns
// up to `count` chain blocks formatted as FieldsValues for the requested chain type. When count is
// zero it returns an empty slice.
func GetChainBlocks(
	tx kv.Tx,
	bucket string,
	chainType apptypes.ChainType,
	count uint64,
) ([]FieldsValues, error) {
	if count == 0 {
		return []FieldsValues{}, nil
	}

	cur, err := tx.Cursor(bucket)
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
	prefixBytes := prefix(chainType)

	for len(k) > 0 {
		if bytes.HasPrefix(k, prefixBytes) {
			cb, decErr := decodeStoredChainBlock(v, chainType)
			if decErr != nil {
				return nil, decErr
			}

			out = append(out, cb.convertToFieldsValues())
			if uint64(len(out)) == count {
				break
			}
		}

		k, v, err = cur.Prev()
		if err != nil {
			return nil, err
		}
	}

	if len(out) == 0 {
		return nil, ErrNoBlocks
	}

	return out, nil
}

func decodeStoredChainBlock(value []byte, chainType apptypes.ChainType) (*ChainBlock, error) {
	var stored storedChainBlock
	if err := cbor.Unmarshal(value, &stored); err != nil {
		return nil, err
	}

	if stored.ChainType != 0 && stored.ChainType != chainType {
		return nil, fmt.Errorf(
			"%w: got %d expected %d",
			ErrChainTypeMismatch,
			stored.ChainType,
			chainType,
		)
	}

	return newChainBlockFromPayload(chainType, stored.Block)
}

func newChainBlockFromPayload(
	chainType apptypes.ChainType,
	payload []byte,
) (cb *ChainBlock, err error) {
	cb, err = NewChainBlock(chainType, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode chain block: %w", err)
	}

	return cb, nil
}
