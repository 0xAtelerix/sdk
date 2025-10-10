package block

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	BlockHashBucket   = "blockhash"   // block-hash -> block
	BlockNumberBucket = "blocknumber" // block-number -> block
)

var ErrNoBlocks = errors.New("no blocks found")

type Block struct {
	Number    uint64   `json:"number" cbor:"1,keyasint"`
	Hash      [32]byte `json:"hash" cbor:"2,keyasint"`
	StateRoot [32]byte `json:"stateroot" cbor:"3,keyasint"`
	Timestamp uint64   `json:"timestamp" cbor:"4,keyasint"`
}

// // Number implements apptypes.AppchainBlock.
// func (b *Block) Number() uint64 { return b.Number }

// // Hash implements apptypes.AppchainBlock.
// func (b *Block) Hash() [32]byte { return b.hash }

// StateRoot implements apptypes.AppchainBlock.
func (b *Block) ComputeStateRoot() [32]byte {
	data, _ := cbor.Marshal(b)
	return sha256.Sum256(data)
}

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

type FieldsValues struct {
	Fields []string
	Values []string
}

// func GetBlock(tx kv.Tx, bucket string, key []byte, block apptypes.AppchainBlock) (apptypes.AppchainBlock, error) {
// 	value, err := tx.GetOne(bucket, key)
// 	if err != nil {
// 		return block, err
// 	}

// 	if len(value) == 0 {
// 		return block, ErrNoBlocks
// 	}

// 	if err := cbor.Unmarshal(value, &block); err != nil {
// 		return block, err
// 	}

// 	return block, nil
// }

// GetBlock loads a block and returns two slices of strings:
//  1. the field names of Block (in declaration order)
//  2. the stringified values of those fields
//
// Stringification:
//   - uint64    → decimal string
//   - [32]byte  → 0x-prefixed lowercase hex
func GetBlock(tx kv.Tx, bucket string, key []byte) (any, error) {
	value, err := tx.GetOne(bucket, key)
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return nil, ErrNoBlocks
	}
	// Decode into the concrete Block to access fields deterministically
	var b Block
	if err := cbor.Unmarshal(value, &b); err != nil {
		return nil, err
	}
	// Field names from `json` tags in declaration order
	t := reflect.TypeOf(b)
	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := f.Tag.Get("json")
		if name == "" || name == "-" {
			name = f.Name
		}
		fields = append(fields, name)
	}
	// Values aligned with fields order
	values := []string{
		fmt.Sprintf("%d", b.Number),
		fmt.Sprintf("0x%x", b.Hash),
		fmt.Sprintf("0x%x", b.StateRoot),
		fmt.Sprintf("%d", b.Timestamp),
	}

	return FieldsValues{
		Fields: fields,
		Values: values,
	}, nil
}

// // GetBlocks returns up to `count` most recent blocks from the BlockNumberBucket (newest first).
// // It decodes each block into the concrete type of the provided `prototype`.
// // If `count` is 0, it returns an empty slice. If the bucket is empty, returns ErrNoBlocks.
// func GetBlocks(tx kv.Tx, count int, prototype apptypes.AppchainBlock) ([]apptypes.AppchainBlock, error) {
// 	if count <= 0 {
// 		return []apptypes.AppchainBlock{}, nil
// 	}
// 	if prototype == nil {
// 		return nil, fmt.Errorf("nil block prototype")
// 	}
// 	cur, err := tx.Cursor(BlockNumberBucket)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// Move to the last (highest-numbered) block
// 	k, v, err := cur.Last()
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(k) == 0 {
// 		return nil, ErrNoBlocks
// 	}
// 	blocks := make([]apptypes.AppchainBlock, 0, count)

// 	// helper to allocate a fresh instance of the prototype's concrete type
// 	newInstance := func() any {
// 		typ := reflect.TypeOf(prototype)
// 		if typ.Kind() == reflect.Ptr {
// 			return reflect.New(typ.Elem()).Interface()
// 		}
// 		return reflect.New(typ).Interface()
// 	}

// 	appendDecoded := func(val []byte) error {
// 		inst := newInstance()
// 		if err := cbor.Unmarshal(val, inst); err != nil {
// 			return err
// 		}
// 		// Prefer pointer assertion; fall back to element if needed
// 		if b, ok := inst.(apptypes.AppchainBlock); ok {
// 			blocks = append(blocks, b)
// 			return nil
// 		}
// 		iv := reflect.ValueOf(inst)
// 		if iv.Kind() == reflect.Ptr {
// 			iv = iv.Elem()
// 		}
// 		if b2, ok := iv.Interface().(apptypes.AppchainBlock); ok {
// 			blocks = append(blocks, b2)
// 			return nil
// 		}
// 		return fmt.Errorf("decoded value does not implement AppchainBlock: %T", inst)
// 	}

// 	for i := 0; i < count && len(k) > 0; i++ {
// 		if err := appendDecoded(v); err != nil {
// 			return nil, err
// 		}
// 		k, v, err = cur.Prev()
// 		if err != nil {
// 			return nil, err
// 		}
// 	}
// 	return blocks, nil
// }

func NumberToBytes(input uint64) []byte {
	// Create a byte slice of length 8, as uint64 occupies 8 bytes
	b := make([]byte, 8)

	// Encode the uint64 into the byte slice using Big Endian byte order
	binary.BigEndian.PutUint64(b, input)

	return b
}
