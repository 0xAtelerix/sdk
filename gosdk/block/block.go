package block

import (
	"errors"
	"encoding/binary"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	BlockHashBucket = "blockhash" // block-hash -> block
    BlockNumberBucket = "blocknumber" // block-number -> block
)

var ErrNoBlocks = errors.New("no blocks found")

func StoreBlockbyHash(tx kv.RwTx, block apptypes.AppchainBlock) error {
	key := block.Hash()

	value, err := cbor.Marshal(block)
	if err != nil {
		return err
	}

	return tx.Put(BlockHashBucket, key[:], value)
}

func StoreBlockbyNumber(tx kv.RwTx, bucket string, block apptypes.AppchainBlock) error {
	key := NumberToBytes(block.Number())

	value, err := cbor.Marshal(block)
	if err != nil {
		return err
	}

	return tx.Put(bucket, key, value)
}

func GetBlock(tx kv.Tx, bucket string, key []byte, block apptypes.AppchainBlock) (apptypes.AppchainBlock, error) {
	value, err := tx.GetOne(bucket, key)
	if err != nil {
		return block, err
	}

	if len(value) == 0 {
		return block, ErrNoBlocks
	}

	if err := cbor.Unmarshal(value, &block); err != nil {
		return block, err
	}

	return block, nil
}

func NumberToBytes(input uint64) []byte {
	// Create a byte slice of length 8, as uint64 occupies 8 bytes
	b := make([]byte, 8)

	// Encode the uint64 into the byte slice using Big Endian byte order
	binary.BigEndian.PutUint64(b, input)

	return b
}

