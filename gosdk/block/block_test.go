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

// adaptBlocksResult normalizes GetBlocks(any,error) to []FieldsValues
func adaptBlocksResult(t *testing.T, result any) []FieldsValues {
	switch v := result.(type) {
	case []FieldsValues:
		return v
	case *[]FieldsValues:
		return *v
	default:
		require.Failf(t, "unexpected result type from GetBlocks", "got %T", result)
		return nil
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

// -------------------- GetBlocks(count) []FieldsValues tests --------------------

func TestGetBlocks_OrderAndCount(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	// Seed 10 blocks numbered 0..9 (as raw CBOR of the on-disk Block layout)
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		for i := 0; i < 10; i++ {
			enc, b := makeCBORBlock(uint64(i), fmt.Sprintf("blk-%d", i))
			if e := tx.Put(BlockNumberBucket, NumberToBytes(b.Number), enc); e != nil {
				return e
			}
		}
		return nil
	})
	require.NoError(t, err)

	// Retrieve last 5 blocks → expect 9,8,7,6,5 (newest first)
	err = db.View(context.Background(), func(tx kv.Tx) error {
		resAny, e := GetBlocks(tx, 5)
		if e != nil {
			return e
		}
		res := adaptBlocksResult(t, resAny)
		require.Len(t, res, 5)      


		wantNums := []uint64{9, 8, 7, 6, 5}
		assert.Equal(t, expectedFieldOrder(), res[0].Fields) // same for all entries

		for i, fv := range res {
			require.Len(t, fv.Values, 4)
			// number
			assert.Equal(t, fmt.Sprintf("%d", wantNums[i]), fv.Values[0])
			// hash, stateroot as hex
			assert.True(t, strings.HasPrefix(fv.Values[1], "0x"))
			assert.True(t, strings.HasPrefix(fv.Values[2], "0x"))
			// timestamp matches the deterministic pattern used by makeCBORBlock
			wantTs := 1630000000 + wantNums[i]*10
			assert.Equal(t, fmt.Sprintf("%d", wantTs), fv.Values[3])
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGetBlocks_CountExceedsAvailable(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	// Seed 3 blocks (0,1,2)
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		for i := 0; i < 3; i++ {
			enc, b := makeCBORBlock(uint64(i), fmt.Sprintf("n=%d", i))
			if e := tx.Put(BlockNumberBucket, NumberToBytes(b.Number), enc); e != nil {
				return e
			}
		}
		return nil
	})
	require.NoError(t, err)

	// Request 10 → should return all 3: 2,1,0
	err = db.View(context.Background(), func(tx kv.Tx) error {
		resAny, e := GetBlocks(tx, 10)
		if e != nil {
			return e
		}
		res := adaptBlocksResult(t, resAny)
		require.Len(t, res, 3)
		wantNums := []uint64{2, 1, 0}
		for i, fv := range res {
			assert.Equal(t, fmt.Sprintf("%d", wantNums[i]), fv.Values[0])
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGetBlocks_ZeroCount(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	err := db.View(context.Background(), func(tx kv.Tx) error {
		resAny, e := GetBlocks(tx, 0)
		res := adaptBlocksResult(t, resAny)
		require.NoError(t, e)
		assert.Empty(t, res)
		return nil
	})
	require.NoError(t, err)
}

func TestGetBlocks_EmptyDB(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	err := db.View(context.Background(), func(tx kv.Tx) error {
		_, e := GetBlocks(tx, 5)
		assert.ErrorIs(t, e, ErrNoBlocks)
		return nil
	})
	require.NoError(t, err)
}
