package block

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"reflect"
	"strconv"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	BlockHashBucket         = "blockhash"   // block-hash -> block
	BlockNumberBucket       = "blocknumber" // block-number -> block
	BlockTransactionsBucket = "blocktxs"    // block-number -> txs
)

var _ apptypes.AppchainBlock = (*Block)(nil)

type Block struct {
	BlockNumber uint64   `json:"number"    cbor:"1,keyasint"`
	BlockHash   [32]byte `json:"hash"      cbor:"2,keyasint"`
	BlockRoot   [32]byte `json:"stateroot" cbor:"3,keyasint"`
	Timestamp   uint64   `json:"timestamp" cbor:"4,keyasint"`
}

// Number implements apptypes.AppchainBlock.
func (b Block) Number() uint64 { return b.BlockNumber }

// Hash implements apptypes.AppchainBlock.
func (b Block) Hash() [32]byte { return b.BlockHash }

// StateRoot implements apptypes.AppchainBlock.
func (b Block) StateRoot() [32]byte {
	return sha256.Sum256(b.Bytes())
}

// Bytes implements apptypes.AppchainBlock.
// We use CBOR to keep it consistent with storage/other hashes in this package.
func (b Block) Bytes() []byte {
	data, err := cbor.Marshal(b)
	if err != nil {
		return nil
	}

	return data
}

// TODO convertToFieldsValues converts Block to BlockFieldsValues.
func (b *Block) convertToFieldsValues() BlockFieldsValues {
	if b == nil {
		b = &Block{}
	}

	// Field names from `json` tags in declaration order
	var zero Block
	t := reflect.TypeOf(zero)

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
		strconv.FormatUint(b.Number(), 10),
		fmt.Sprintf("0x%x", b.Hash()),
		fmt.Sprintf("0x%x", b.StateRoot()),
		strconv.FormatUint(b.Timestamp, 10),
	}

	return BlockFieldsValues{Fields: fields, Values: values}
}

// var _ apptypes.AppchainBlock = (*Block)(nil)
// type Block struct {
// 	Number    uint64   `json:"number" cbor:"1,keyasint"`
// 	Hash      [32]byte `json:"hash" cbor:"2,keyasint"`
// 	StateRoot [32]byte `json:"stateroot" cbor:"3,keyasint"`
// 	Timestamp uint64   `json:"timestamp" cbor:"4,keyasint"`
// }

// // Number implements apptypes.AppchainBlock.
// func (b *Block) Number() uint64 { return b.Number }

// // Hash implements apptypes.AppchainBlock.
// func (b *Block) Hash() [32]byte { return b.hash }

// StateRoot implements apptypes.AppchainBlock.
// func (b *Block) ComputeStateRoot() [32]byte {
// 	data, _ := cbor.Marshal(b)
// 	return sha256.Sum256(data)
// }

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

type BlockFieldsValues struct {
	Fields []string
	Values []string
}

// GetBlock loads a block
// TODO consider to pass third argument of type apptypes.AppchainBlock to decode into concrete type
func GetBlock(
	tx kv.Tx,
	bucket string,
	key []byte,
	block apptypes.AppchainBlock,
) (BlockFieldsValues, error) {
	value, err := tx.GetOne(bucket, key)
	if err != nil {
		return BlockFieldsValues{}, err
	}

	if len(value) == 0 {
		return BlockFieldsValues{}, ErrNoBlocks
	}
	// Decode into the concrete Block to access fields deterministically
	b, ok := block.(*Block)
	if !ok {
		return BlockFieldsValues{}, ErrUnsupportedBlockType
	}

	if err := cbor.Unmarshal(value, &b); err != nil {
		return BlockFieldsValues{}, err
	}

	return b.convertToFieldsValues(), nil
}

// // GetBlocks returns up to `count` most recent blocks from the BlockNumberBucket (newest first)
// // and formats each block as BlockFieldsValues (same shape as GetBlock).
// // If count <= 0, it returns an empty slice. If the bucket is empty, returns ErrNoBlocks.
// func GetBlocks(tx kv.Tx, count uint64) (any, error) {
// 	if count == 0 {
// 		return []BlockFieldsValues{}, nil
// 	}

// 	cur, err := tx.Cursor(BlockNumberBucket)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Move to the last (highest-numbered) block.
// 	k, v, err := cur.Last()
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(k) == 0 {
// 		return nil, ErrNoBlocks
// 	}

// 	// Field names from `json` tags in declaration order
// 	// TODO declare standadlne function and remove code duplication with convertToFieldsValues
// 	var zero Block
// 	t := reflect.TypeOf(zero)
// 	fields := make([]string, 0, t.NumField())
// 	for i := 0; i < t.NumField(); i++ {
// 		f := t.Field(i)
// 		name := f.Tag.Get("json")
// 		if name == "" || name == "-" {
// 			name = f.Name
// 		}
// 		fields = append(fields, name)
// 	}

// 	out := make([]BlockFieldsValues, 0, count)

// 	appendOne := func(val []byte) error {
// 		var b Block
// 		if err = cbor.Unmarshal(val, &b); err != nil {
// 			return err
// 		}
// 		values := []string{
// 			fmt.Sprintf("%d", b.Number()),
// 			fmt.Sprintf("0x%x", b.Hash()),
// 			fmt.Sprintf("0x%x", b.StateRoot()),
// 			fmt.Sprintf("%d", b.Timestamp),
// 		}
// 		out = append(out, BlockFieldsValues{Fields: fields, Values: values})
// 		return nil
// 	}

// 	for i := uint64(0); i < count && len(k) > 0; i++ {
// 		if err = appendOne(v); err != nil {
// 			return nil, err
// 		}
// 		k, v, err = cur.Prev()
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	return out, nil
// }

// // Mirror of txpool_test.go CustomTransaction shape (same JSON and CBOR tags).
// // We cannot import the test type, but we guarantee identical (un)marshal shape.
// // TODO consider replace it with apptypes.AppTransaction
// type blockCustomTx struct {
// 	From  string `json:"from"  cbor:"1,keyasint"`
// 	To    string `json:"to"    cbor:"2,keyasint"`
// 	Value int    `json:"value" cbor:"3,keyasint"`
// }

// // getBlockbyNumber loads and decodes a Block by its number from BlockNumberBucket.
// func getBlockbyNumber(tx kv.Tx, number uint64) (Block, error) {
// 	key := NumberToBytes(number)

// 	value, err := tx.GetOne(BlockNumberBucket, key)
// 	if err != nil {
// 		return Block{}, err
// 	}
// 	if len(value) == 0 {
// 		return Block{}, ErrNoBlocks
// 	}

// 	var b Block
// 	if err := cbor.Unmarshal(value, &b); err != nil {
// 		return Block{}, err
// 	}
// 	return b, nil
// }

// // getTransactionforBlock retrieves transactions for the given concrete Block.
// // It supports two encodings under BlockTransactionsBucket:
// //  1. CBOR of []blockCustomTx
// //  2. CBOR of [][]byte, where each element is CBOR(blockCustomTx)
// //
// // If nothing is stored, it returns an empty slice instead of an error.
// func getTransactionsForBlock(tx kv.Tx, blockNum uint64) ([]blockCustomTx, error) {
// 	val, err := tx.GetOne(BlockTransactionsBucket, NumberToBytes(blockNum))
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(val) == 0 {
// 		return nil, nil
// 	}

// 	// Try direct CBOR([]blockCustomTx)
// 	var direct []blockCustomTx
// 	if e := cbor.Unmarshal(val, &direct); e == nil {
// 		return direct, nil
// 	}

// 	// Try CBOR([][]byte) with nested CBOR(blockCustomTx)
// 	var raw [][]byte
// 	if e := cbor.Unmarshal(val, &raw); e == nil {
// 		out := make([]blockCustomTx, 0, len(raw))
// 		for _, r := range raw {
// 			var t blockCustomTx
// 			if ue := cbor.Unmarshal(r, &t); ue != nil {
// 				return nil, ErrDecodeTransactionPayloadFailed
// 			}
// 			out = append(out, t)
// 		}
// 		return out, nil
// 	}

// 	return nil, ErrUnsupportedTransactionPayload
// }

// // GetTransactionsByBlockNumber is a convenience that chains getBlockbyNumber
// // and getTransactionforBlock, returning the tx list as `any`.

// func GetTransactionsByBlockNumber(tx kv.Tx, number uint64) (any, error) {
// 	// TODO consider to replace it with GetBlock implementation to return BlockFieldsValues
// 	// b, err := getBlockbyNumber(tx, number)
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	// TODO consider to implement storeTransactionForBlock
// 	txs, err := getTransactionsForBlock(tx, number)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return txs, nil
// }

func NumberToBytes(input uint64) []byte {
	// Create a byte slice of length 8, as uint64 occupies 8 bytes
	b := make([]byte, 8)

	// Encode the uint64 into the byte slice using Big Endian byte order
	binary.BigEndian.PutUint64(b, input)

	return b
}
