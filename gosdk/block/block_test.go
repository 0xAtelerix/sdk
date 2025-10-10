package block

import (
	"context"
	"crypto/sha256"

	//"encoding/binary"
	"fmt"
	//"math"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -------------------- Shared helpers --------------------

func createDB(t testing.TB, buckets ...string) kv.RwDB {
	t.Helper()
	db := memdb.NewTestDB(t)
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		for _, b := range buckets {
			if e := tx.CreateBucket(b); e != nil {
				return e
			}
		}
		return nil
	})
	require.NoError(t, err)
	return db
}

// testBlock mirrors the on-disk CBOR layout of Block using the same numeric keys.
type testBlock struct {
	Number    uint64   `json:"number"    cbor:"1,keyasint"`
	Hash      [32]byte `json:"hash"      cbor:"2,keyasint"`
	StateRoot [32]byte `json:"stateroot" cbor:"3,keyasint"`
	Timestamp uint64   `json:"timestamp" cbor:"4,keyasint"`
}

func makeCBORBlock(num uint64, note string) ([]byte, testBlock) {
	// Derive a deterministic hash
	h := sha256.Sum256([]byte(fmt.Sprintf("block-%d-%s", num, note)))
	blk := testBlock{
		Number:    num,
		Hash:      h,
		Timestamp: 1630000000 + num*10,
	}
	// Compute a synthetic state root similar to production code
	tmp, _ := cbor.Marshal(blk)
	blk.StateRoot = sha256.Sum256(tmp)
	enc, _ := cbor.Marshal(blk)
	return enc, blk
}

func expectedFieldOrder() []string { return []string{"number", "hash", "stateroot", "timestamp"} }

// adaptResult normalizes GetBlock(any,error) into (fields,values)
func adaptResult(t *testing.T, result any) (fields, values []string) {
	switch v := result.(type) {
	case FieldsValues: // struct with Fields/Values
		return v.Fields, v.Values
	case *FieldsValues:
		return v.Fields, v.Values
	default:
		require.Failf(t, "unexpected result type", "got %T", result)
		return nil, nil
	}
}

// -------------------- GetBlock(any,error) tests --------------------

func TestStoreAndGetBlockByHash(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockHashBucket)
	defer db.Close()

	enc, want := makeCBORBlock(1, "by-hash")

	// Store by hash
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.Put(BlockHashBucket, want.Hash[:], enc)
	})
	require.NoError(t, err)

	// Retrieve
	err = db.View(context.Background(), func(tx kv.Tx) error {
		result, getErr := GetBlock(tx, BlockHashBucket, want.Hash[:])
		if getErr != nil {
			return getErr
		}
		fields, values := adaptResult(t, result)

		assert.Equal(t, expectedFieldOrder(), fields)
		require.Len(t, values, 4)
		assert.Equal(t, "1", values[0])                               // number (decimal)
		assert.True(t, strings.HasPrefix(values[1], "0x"))            // hash (hex)
		assert.True(t, strings.HasPrefix(values[2], "0x"))            // stateroot (hex)
		assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3]) // timestamp
		return nil
	})
	require.NoError(t, err)
}

func TestStoreAndGetBlockByNumber(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	enc, want := makeCBORBlock(42, "by-number")
	key := NumberToBytes(want.Number)

	// Store by number
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.Put(BlockNumberBucket, key, enc)
	})
	require.NoError(t, err)

	// Retrieve
	err = db.View(context.Background(), func(tx kv.Tx) error {
		result, getErr := GetBlock(tx, BlockNumberBucket, key)
		if getErr != nil {
			return getErr
		}
		fields, values := adaptResult(t, result)

		assert.Equal(t, expectedFieldOrder(), fields)
		require.Len(t, values, 4)
		assert.Equal(t, "42", values[0])                              // number
		assert.True(t, strings.HasPrefix(values[1], "0x"))            // hash
		assert.True(t, strings.HasPrefix(values[2], "0x"))            // stateroot
		assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3]) // timestamp
		return nil
	})
	require.NoError(t, err)
}

func TestStoreMultipleBlocksAndRetrieve_GetBlockShape(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockHashBucket, BlockNumberBucket)
	defer db.Close()

	encs := make([][]byte, 0, 10)
	blks := make([]testBlock, 0, 10)
	for i := 0; i < 10; i++ {
		enc, b := makeCBORBlock(uint64(i), fmt.Sprintf("blk-%d", i))
		encs = append(encs, enc)
		blks = append(blks, b)
	}

	// Store both by hash and by number
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		for i, b := range blks {
			if e := tx.Put(BlockHashBucket, b.Hash[:], encs[i]); e != nil {
				return e
			}
			if e := tx.Put(BlockNumberBucket, NumberToBytes(b.Number), encs[i]); e != nil {
				return e
			}
		}
		return nil
	})
	require.NoError(t, err)

	// Verify retrieval by hash
	err = db.View(context.Background(), func(tx kv.Tx) error {
		for _, want := range blks {
			res, e := GetBlock(tx, BlockHashBucket, want.Hash[:])
			if e != nil {
				return e
			}
			fields, values := adaptResult(t, res)
			assert.Equal(t, expectedFieldOrder(), fields)
			require.Len(t, values, 4)
			assert.Equal(t, fmt.Sprintf("%d", want.Number), values[0])
			assert.True(t, strings.HasPrefix(values[1], "0x"))
			assert.True(t, strings.HasPrefix(values[2], "0x"))
			assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3])
		}
		return nil
	})
	require.NoError(t, err)

	// Verify retrieval by number
	err = db.View(context.Background(), func(tx kv.Tx) error {
		for _, want := range blks {
			key := NumberToBytes(want.Number)
			res, e := GetBlock(tx, BlockNumberBucket, key)
			if e != nil {
				return e
			}
			fields, values := adaptResult(t, res)
			assert.Equal(t, expectedFieldOrder(), fields)
			require.Len(t, values, 4)
			assert.Equal(t, fmt.Sprintf("%d", want.Number), values[0])
			assert.True(t, strings.HasPrefix(values[1], "0x"))
			assert.True(t, strings.HasPrefix(values[2], "0x"))
			assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3])
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGetBlockNonExistent(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockHashBucket, BlockNumberBucket)
	defer db.Close()

	missing := sha256.Sum256([]byte("no-such-block"))

	err := db.View(context.Background(), func(tx kv.Tx) error {
		_, e := GetBlock(tx, BlockHashBucket, missing[:])
		assert.Error(t, e)
		return nil
	})
	require.NoError(t, err)

	key := NumberToBytes(999_999)
	err = db.View(context.Background(), func(tx kv.Tx) error {
		_, e := GetBlock(tx, BlockNumberBucket, key)
		assert.Error(t, e)
		return nil
	})
	require.NoError(t, err)
}

// // -------------------- Latest blocks (GetBlocks) tests --------------------

// type TestBlock struct {
// 	H    [32]byte `json:"hash" cbor:"1,keyasint"`
// 	N    uint64   `json:"number" cbor:"2,keyasint"`
// 	Data string   `json:"data" cbor:"3,keyasint"`
// }

// func (b *TestBlock) Hash() [32]byte { return b.H }
// func (b *TestBlock) Number() uint64 { return b.N }

// func (b *TestBlock) StateRoot() [32]byte {
// 	return sha256.Sum256(b.Bytes())
// }

// func (b *TestBlock) Bytes() []byte {
// 	buf := make([]byte, 0, 32+8+len(b.Data))
// 	buf = append(buf, b.H[:]...)
// 	var n [8]byte
// 	binary.BigEndian.PutUint64(n[:], b.N)
// 	buf = append(buf, n[:]...)
// 	buf = append(buf, []byte(b.Data)...)
// 	return buf
// }

// func newTestBlock(num uint64, data string) *TestBlock {
// 	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", num, data)))
// 	return &TestBlock{H: h, N: num, Data: data}
// }

// func asTestBlock(t *testing.T, v any) *TestBlock {
// 	switch b := v.(type) {
// 	case *TestBlock:
// 		return b
// 	case TestBlock:
// 		return &b
// 	default:
// 		require.Failf(t, "type assertion failed", "expected TestBlock, got %T", v)
// 		return nil
// 	}
// }

// func TestGetLatestBlocks_OrderAndCount(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	// store 10 blocks numbered 0..9
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		for i := 0; i < 10; i++ {
// 			if e := StoreBlockbyNumber(tx, BlockNumberBucket, newTestBlock(uint64(i), fmt.Sprintf("blk-%d", i))); e != nil {
// 				return e
// 			}
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)

// 	// retrieve last 5 blocks, expect 9,8,7,6,5
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		res, e := GetBlocks(tx, 5, &TestBlock{})
// 		if e != nil {
// 			return e
// 		}
// 		require.Len(t, res, 5)
// 		want := []uint64{9, 8, 7, 6, 5}
// 		for i, blkAny := range res {
// 			blk := asTestBlock(t, blkAny)
// 			assert.Equal(t, want[i], blk.Number())
// 			assert.Equal(t, fmt.Sprintf("blk-%d", want[i]), blk.Data)
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestGetLatestBlocks_CountExceedsAvailable(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	// store 3 blocks
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		for i := 0; i < 3; i++ {
// 			if e := StoreBlockbyNumber(tx, BlockNumberBucket, newTestBlock(uint64(i), fmt.Sprintf("n=%d", i))); e != nil {
// 				return e
// 			}
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)

// 	// request more than available; should return all (newest first)
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		res, e := GetBlocks(tx, 10, &TestBlock{})
// 		if e != nil {
// 			return e
// 		}
// 		require.Len(t, res, 3)
// 		want := []uint64{2, 1, 0}
// 		for i, blkAny := range res {
// 			blk := asTestBlock(t, blkAny)
// 			assert.Equal(t, want[i], blk.Number())
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestGetLatestBlocks_ZeroCount(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	err := db.View(context.Background(), func(tx kv.Tx) error {
// 		res, e := GetBlocks(tx, 0, &TestBlock{})
// 		require.NoError(t, e)
// 		assert.Empty(t, res)
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestGetLatestBlocks_EmptyDB(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	err := db.View(context.Background(), func(tx kv.Tx) error {
// 		_, e := GetBlocks(tx, 5, &TestBlock{})
// 		assert.ErrorIs(t, e, ErrNoBlocks)
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// // -------------------- NumberToBytes test --------------------

// func TestNumberToBytesBigEndian(t *testing.T) {
// 	cases := []struct {
// 		name string
// 		in   uint64
// 	}{
// 		{"zero", 0},
// 		{"one", 1},
// 		{"pattern", 0x0102030405060708},
// 		{"max", math.MaxUint64},
// 	}

// 	for _, tc := range cases {
// 		t.Run(tc.name, func(tr *testing.T) {
// 			got := NumberToBytes(tc.in)

// 			// expected big endian encoding
// 			var want [8]byte
// 			binary.BigEndian.PutUint64(want[:], tc.in)

// 			assert.Equal(tr, want[:], got)
// 		})
// 	}
// }
