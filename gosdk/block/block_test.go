package block

// import (
// 	"context"
// 	"crypto/sha256"

// 	//"encoding/binary"
// 	"fmt"
// 	//"math"
// 	"strings"
// 	"testing"

// 	"github.com/fxamacker/cbor/v2"
// 	"github.com/ledgerwatch/erigon-lib/kv"
// 	"github.com/ledgerwatch/erigon-lib/kv/memdb"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )

// // -------------------- Shared helpers --------------------

// func createDB(t testing.TB, buckets ...string) kv.RwDB {
// 	t.Helper()
// 	db := memdb.NewTestDB(t)
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		for _, b := range buckets {
// 			if e := tx.CreateBucket(b); e != nil {
// 				return e
// 			}
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)
// 	return db
// }

// // testBlock mirrors the on-disk CBOR layout of Block using the same numeric keys.
// type testBlock struct {
// 	Number    uint64   `json:"number"    cbor:"1,keyasint"`
// 	Hash      [32]byte `json:"hash"      cbor:"2,keyasint"`
// 	StateRoot [32]byte `json:"stateroot" cbor:"3,keyasint"`
// 	Timestamp uint64   `json:"timestamp" cbor:"4,keyasint"`
// }

// func newTestBlock(num uint64, note string) ([]byte, testBlock) {
// 	// Derive a deterministic hash
// 	h := sha256.Sum256([]byte(fmt.Sprintf("block-%d-%s", num, note)))
// 	blk := testBlock{
// 		Number:    num,
// 		Hash:      h,
// 		Timestamp: 1630000000 + num*10,
// 	}
// 	// Compute a synthetic state root similar to production code
// 	tmp, _ := cbor.Marshal(blk)
// 	blk.StateRoot = sha256.Sum256(tmp)
// 	enc, _ := cbor.Marshal(blk)
// 	return enc, blk
// }

// func expectedFieldOrder() []string { return []string{"number", "hash", "stateroot", "timestamp"} }

// // adaptResult normalizes GetBlock(any,error) into (fields,values)
// func adaptResult(t *testing.T, result any) (fields, values []string) {
// 	switch v := result.(type) {
// 	case BlockFieldsValues: // struct with Fields/Values
// 		return v.Fields, v.Values
// 	case *BlockFieldsValues:
// 		return v.Fields, v.Values
// 	default:
// 		require.Failf(t, "unexpected result type", "got %T", result)
// 		return nil, nil
// 	}
// }

// // adaptBlocksResult normalizes GetBlocks(any,error) to []BlockFieldsValues
// // TODO remove duplicated code
// func adaptBlocksResult(t *testing.T, result any) []BlockFieldsValues {
// 	switch v := result.(type) {
// 	case []BlockFieldsValues:
// 		return v
// 	case *[]BlockFieldsValues:
// 		return *v
// 	default:
// 		require.Failf(t, "unexpected result type from GetBlocks", "got %T", result)
// 		return nil
// 	}
// }

// // -------------------- GetBlock(any,error) tests --------------------

// func TestStoreAndGetBlockByHash(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockHashBucket)
// 	defer db.Close()

// 	enc, want := newTestBlock(1, "by-hash")

// 	// Store by hash
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(BlockHashBucket, want.Hash[:], enc)
// 	})
// 	require.NoError(t, err)

// 	// Retrieve
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		result, getErr := GetBlock(tx, BlockHashBucket, want.Hash[:])
// 		if getErr != nil {
// 			return getErr
// 		}
// 		fields, values := adaptResult(t, result)

// 		assert.Equal(t, expectedFieldOrder(), fields)
// 		require.Len(t, values, 4)
// 		assert.Equal(t, "1", values[0])                               // number (decimal)
// 		assert.True(t, strings.HasPrefix(values[1], "0x"))            // hash (hex)
// 		assert.True(t, strings.HasPrefix(values[2], "0x"))            // stateroot (hex)
// 		assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3]) // timestamp
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestStoreAndgetBlockbyNumber(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	enc, want := newTestBlock(42, "by-number")
// 	key := NumberToBytes(want.Number)

// 	// Store by number
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(BlockNumberBucket, key, enc)
// 	})
// 	require.NoError(t, err)

// 	// Retrieve
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		result, getErr := GetBlock(tx, BlockNumberBucket, key)
// 		if getErr != nil {
// 			return getErr
// 		}
// 		fields, values := adaptResult(t, result)

// 		assert.Equal(t, expectedFieldOrder(), fields)
// 		require.Len(t, values, 4)
// 		assert.Equal(t, "42", values[0])                              // number
// 		assert.True(t, strings.HasPrefix(values[1], "0x"))            // hash
// 		assert.True(t, strings.HasPrefix(values[2], "0x"))            // stateroot
// 		assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3]) // timestamp
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestStoreMultipleBlocksAndRetrieve_GetBlockShape(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockHashBucket, BlockNumberBucket)
// 	defer db.Close()

// 	encs := make([][]byte, 0, 10)
// 	blks := make([]testBlock, 0, 10)
// 	for i := range 10 {
// 		enc, b := newTestBlock(uint64(i), fmt.Sprintf("blk-%d", i))
// 		encs = append(encs, enc)
// 		blks = append(blks, b)
// 	}

// 	// Store both by hash and by number
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		for i, b := range blks {
// 			if e := tx.Put(BlockHashBucket, b.Hash[:], encs[i]); e != nil {
// 				return e
// 			}
// 			if e := tx.Put(BlockNumberBucket, NumberToBytes(b.Number), encs[i]); e != nil {
// 				return e
// 			}
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)

// 	// Verify retrieval by hash
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		for _, want := range blks {
// 			res, e := GetBlock(tx, BlockHashBucket, want.Hash[:])
// 			if e != nil {
// 				return e
// 			}
// 			fields, values := adaptResult(t, res)
// 			assert.Equal(t, expectedFieldOrder(), fields)
// 			require.Len(t, values, 4)
// 			assert.Equal(t, fmt.Sprintf("%d", want.Number), values[0])
// 			assert.True(t, strings.HasPrefix(values[1], "0x"))
// 			assert.True(t, strings.HasPrefix(values[2], "0x"))
// 			assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3])
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)

// 	// Verify retrieval by number
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		for _, want := range blks {
// 			key := NumberToBytes(want.Number)
// 			res, e := GetBlock(tx, BlockNumberBucket, key)
// 			if e != nil {
// 				return e
// 			}
// 			fields, values := adaptResult(t, res)
// 			assert.Equal(t, expectedFieldOrder(), fields)
// 			require.Len(t, values, 4)
// 			assert.Equal(t, fmt.Sprintf("%d", want.Number), values[0])
// 			assert.True(t, strings.HasPrefix(values[1], "0x"))
// 			assert.True(t, strings.HasPrefix(values[2], "0x"))
// 			assert.Equal(t, fmt.Sprintf("%d", want.Timestamp), values[3])
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestGetBlockNonExistent(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockHashBucket, BlockNumberBucket)
// 	defer db.Close()

// 	missing := sha256.Sum256([]byte("no-such-block"))

// 	err := db.View(context.Background(), func(tx kv.Tx) error {
// 		_, e := GetBlock(tx, BlockHashBucket, missing[:])
// 		assert.Error(t, e)
// 		return nil
// 	})
// 	require.NoError(t, err)

// 	key := NumberToBytes(999_999)
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		_, e := GetBlock(tx, BlockNumberBucket, key)
// 		assert.Error(t, e)
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// // -------------------- GetBlocks(count) []BlockFieldsValues tests --------------------

// func TestGetBlocks_OrderAndCount(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	// Seed 10 blocks numbered 0..9 (as raw CBOR of the on-disk Block layout)
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		for i := range 10 {
// 			enc, b := newTestBlock(uint64(i), fmt.Sprintf("blk-%d", i))
// 			if e := tx.Put(BlockNumberBucket, NumberToBytes(b.Number), enc); e != nil {
// 				return e
// 			}
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)

// 	// Retrieve last 5 blocks → expect 9,8,7,6,5 (newest first)
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		resAny, e := GetBlocks(tx, 5)
// 		if e != nil {
// 			return e
// 		}
// 		res := adaptBlocksResult(t, resAny)
// 		require.Len(t, res, 5)

// 		wantNums := []uint64{9, 8, 7, 6, 5}
// 		assert.Equal(t, expectedFieldOrder(), res[0].Fields) // same for all entries

// 		for i, fv := range res {
// 			require.Len(t, fv.Values, 4)
// 			// number
// 			assert.Equal(t, fmt.Sprintf("%d", wantNums[i]), fv.Values[0])
// 			// hash, stateroot as hex
// 			assert.True(t, strings.HasPrefix(fv.Values[1], "0x"))
// 			assert.True(t, strings.HasPrefix(fv.Values[2], "0x"))
// 			// timestamp matches the deterministic pattern used by newTestBlock
// 			wantTs := 1630000000 + wantNums[i]*10
// 			assert.Equal(t, fmt.Sprintf("%d", wantTs), fv.Values[3])
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestGetBlocks_CountExceedsAvailable(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	// Seed 3 blocks (0,1,2)
// 	err := db.Update(context.Background(), func(tx kv.RwTx) error {
// 		for i := 0; i < 3; i++ {
// 			enc, b := newTestBlock(uint64(i), fmt.Sprintf("n=%d", i))
// 			if e := tx.Put(BlockNumberBucket, NumberToBytes(b.Number), enc); e != nil {
// 				return e
// 			}
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)

// 	// Request 10 → should return all 3: 2,1,0
// 	err = db.View(context.Background(), func(tx kv.Tx) error {
// 		resAny, e := GetBlocks(tx, 10)
// 		if e != nil {
// 			return e
// 		}
// 		res := adaptBlocksResult(t, resAny)
// 		require.Len(t, res, 3)
// 		wantNums := []uint64{2, 1, 0}
// 		for i, fv := range res {
// 			assert.Equal(t, fmt.Sprintf("%d", wantNums[i]), fv.Values[0])
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestGetBlocks_ZeroCount(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	err := db.View(context.Background(), func(tx kv.Tx) error {
// 		resAny, e := GetBlocks(tx, 0)
// 		res := adaptBlocksResult(t, resAny)
// 		require.NoError(t, e)
// 		assert.Empty(t, res)
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// func TestGetBlocks_EmptyDB(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	err := db.View(context.Background(), func(tx kv.Tx) error {
// 		_, e := GetBlocks(tx, 5)
// 		assert.ErrorIs(t, e, ErrNoBlocks)
// 		return nil
// 	})
// 	require.NoError(t, err)
// }

// // -------------------- getBlockbyNumber / getTransactionforBlock / GetTransactionsByBlockNumber tests --------------------

// // testTx mirrors the CBOR/JSON shape used by block.go's tx struct (from, to, value)
// type testTx struct {
// 	From  string `json:"from"  cbor:"1,keyasint"`
// 	To    string `json:"to"    cbor:"2,keyasint"`
// 	Value int    `json:"value" cbor:"3,keyasint"`
// }

// // normalizeTxs marshals the returned (any or slice) into CBOR and unmarshals
// // into []testTx so we can compare values without referencing the internal type name.
// func normalizeTxs(t *testing.T, v any) []testTx {
// 	t.Helper()
// 	raw, err := cbor.Marshal(v)
// 	require.NoError(t, err)
// 	var out []testTx
// 	require.NoError(t, cbor.Unmarshal(raw, &out))
// 	return out
// }

// func Test_getBlockbyNumber_Found(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	enc, want := newTestBlock(123, "gbn-found")
// 	key := NumberToBytes(want.Number)

// 	// Seed block by number
// 	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(BlockNumberBucket, key, enc)
// 	}))

// 	// Retrieve and verify
// 	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
// 		b, err := getBlockbyNumber(tx, want.Number)
// 		require.NoError(t, err)

// 		assert.Equal(t, want.Number, b.Number)
// 		assert.Equal(t, want.Hash, b.Hash)
// 		assert.Equal(t, want.StateRoot, b.StateRoot)
// 		assert.Equal(t, want.Timestamp, b.Timestamp)
// 		return nil
// 	}))
// }

// func Test_getBlockbyNumber_NotFound(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket)
// 	defer db.Close()

// 	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
// 		_, err := getBlockbyNumber(tx, 999_999)
// 		assert.ErrorIs(t, err, ErrNoBlocks)
// 		return nil
// 	}))
// }

// func Test_getTransactionforBlock_Empty(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockTransactionsBucket)
// 	defer db.Close()

// 	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
// 		// No entry for number 7 → expect empty slice, no error
// 		var blockNum uint64
// 		blockNum = 7
// 		got, err := getTransactionsForBlock(tx, blockNum)
// 		require.NoError(t, err)
// 		assert.Empty(t, got)
// 		return nil
// 	}))
// }

// func Test_getTransactionforBlock_DirectEncoding(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockTransactionsBucket)
// 	defer db.Close()

// 	blockNum := uint64(5)
// 	want := []blockCustomTx{
// 		{From: "alice", To: "bob", Value: 1},
// 		{From: "carol", To: "dave", Value: 2},
// 	}
// 	enc, err := cbor.Marshal(want)
// 	require.NoError(t, err)

// 	// Seed direct slice encoding: CBOR([]testTx)
// 	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(BlockTransactionsBucket, NumberToBytes(blockNum), enc)
// 	}))

// 	// Retrieve via getTransactionforBlock
// 	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
// 		var blockNum uint64
// 		blockNum = 7
// 		got, err := getTransactionsForBlock(tx, blockNum)
// 		require.NoError(t, err)

// 		//gotNorm := normalizeTxs(t, got)
// 		assert.Equal(t, want, got)
// 		return nil
// 	}))
// }

// func Test_getTransactionforBlock_NestedEncoding(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockTransactionsBucket)
// 	defer db.Close()

// 	blockNum := uint64(6)
// 	want := []testTx{
// 		{From: "eve", To: "frank", Value: 7},
// 		{From: "grace", To: "heidi", Value: 8},
// 	}

// 	// Build nested encoding: CBOR([][]byte) where each []byte is CBOR(testTx)
// 	raw := make([][]byte, 0, len(want))
// 	for _, tx := range want {
// 		b, err := cbor.Marshal(tx)
// 		require.NoError(t, err)
// 		raw = append(raw, b)
// 	}
// 	enc, err := cbor.Marshal(raw)
// 	require.NoError(t, err)

// 	// Seed nested payload
// 	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(BlockTransactionsBucket, NumberToBytes(blockNum), enc)
// 	}))

// 	// Retrieve via getTransactionforBlock
// 	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
// 		var blockNum uint64
// 		blockNum = 7
// 		got, err := getTransactionsForBlock(tx, blockNum)
// 		require.NoError(t, err)

// 		gotNorm := normalizeTxs(t, got)
// 		assert.Equal(t, want, gotNorm)
// 		return nil
// 	}))
// }

// func Test_GetTransactionsByBlockNumber_DirectEncoding(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket, BlockTransactionsBucket)
// 	defer db.Close()

// 	// Seed the block header (so lookup by number succeeds)
// 	blkEnc, blk := newTestBlock(77, "txs-direct")
// 	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(BlockNumberBucket, NumberToBytes(blk.Number), blkEnc)
// 	}))

// 	// Seed txs (direct slice)
// 	want := []testTx{
// 		{From: "isaac", To: "judy", Value: 10},
// 		{From: "kate", To: "liam", Value: 11},
// 	}
// 	txsEnc, err := cbor.Marshal(want)
// 	require.NoError(t, err)

// 	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
// 		return tx.Put(BlockTransactionsBucket, NumberToBytes(blk.Number), txsEnc)
// 	}))

// 	// Call GetTransactionsByBlockNumber
// 	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
// 		resAny, err := GetTransactionsByBlockNumber(tx, blk.Number)
// 		require.NoError(t, err)

// 		gotNorm := normalizeTxs(t, resAny)
// 		assert.Equal(t, want, gotNorm)
// 		return nil
// 	}))
// }

// func Test_GetTransactionsByBlockNumber_BlockMissing(t *testing.T) {
// 	t.Parallel()

// 	db := createDB(t, BlockNumberBucket, BlockTransactionsBucket)
// 	defer db.Close()

// 	// No block with this number exists → expect ErrNoBlocks
// 	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
// 		_, err := GetTransactionsByBlockNumber(tx, 999)
// 		assert.ErrorIs(t, err, ErrNoBlocks)
// 		return nil
// 	}))
// }
