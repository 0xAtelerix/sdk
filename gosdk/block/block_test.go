package block

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBlock is a lightweight test double that satisfies the AppchainBlock interface 
type TestBlock struct {
	H    [32]byte `json:"hash" cbor:"1,keyasint"`
	N    uint64   `json:"number" cbor:"2,keyasint"`
	Data string   `json:"data" cbor:"3,keyasint"`
}

func (b *TestBlock) Hash() [32]byte { return b.H }
func (b *TestBlock) Number() uint64 { return b.N }

func (b *TestBlock) StateRoot() [32]byte {
	// Derive a deterministic state root for testing by hashing the serialized bytes
	return sha256.Sum256(b.Bytes())
}

func (b *TestBlock) Bytes() []byte {
	// Compose a deterministic byte representation: hash (32) + number (8 BE) + data
	buf := make([]byte, 0, 32+8+len(b.Data))
	buf = append(buf, b.H[:]...)

	var n [8]byte
	binary.BigEndian.PutUint64(n[:], b.N)
	buf = append(buf, n[:]...)

	buf = append(buf, []byte(b.Data)...)
	return buf
}

func newTestBlock(num uint64, data string) *TestBlock {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", num, data)))
	return &TestBlock{H: h, N: num, Data: data}
}

// asTestBlock normalizes apptypes.AppchainBlock returned by GetBlock
// into *TestBlock regardless of whether the decoder produced a value
// or a pointer.
func asTestBlock(t *testing.T, v any) *TestBlock {
	switch b := v.(type) {
	case *TestBlock:
		return b
	case TestBlock:
		return &b
	default:
		require.Failf(t, "type assertion failed", "expected TestBlock, got %T", v)
		return nil
	}
}

func createDB(t testing.TB, buckets ...string) kv.RwDB {
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

func TestStoreAndGetBlockByHash(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockHashBucket)
	defer db.Close()

	blk := newTestBlock(1, "test-block-1")

	// store by hash
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return StoreBlockbyHash(tx, blk)
	})
	require.NoError(t, err)

	hash := blk.Hash()

	// get by hash
	err = db.View(context.Background(), func(tx kv.Tx) error {
		got, getErr := GetBlock(tx, BlockHashBucket, hash[:], &TestBlock{})
		if getErr != nil {
			return getErr
		}
		b := asTestBlock(t, got)

		assert.Equal(t, blk.H, b.H)
		assert.Equal(t, blk.N, b.N)
		assert.Equal(t, blk.Data, b.Data)
		return nil
	})
	require.NoError(t, err)
}

func TestStoreAndGetBlockByNumber(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	blk := newTestBlock(42, "answer")

	// store by number into the number bucket
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return StoreBlockbyNumber(tx, BlockNumberBucket, blk)
	})
	require.NoError(t, err)

	// get by number from the number bucket
	key := NumberToBytes(blk.Number())
	err = db.View(context.Background(), func(tx kv.Tx) error {
		got, getErr := GetBlock(tx, BlockNumberBucket, key, &TestBlock{})
		if getErr != nil {
			return getErr
		}
		b := asTestBlock(t, got)

		assert.Equal(t, blk.H, b.H)
		assert.Equal(t, blk.N, b.N)
		assert.Equal(t, blk.Data, b.Data)
		return nil
	})
	require.NoError(t, err)
}

func TestStoreMultipleBlocksAndRetrieve(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockHashBucket, BlockNumberBucket)
	defer db.Close()

	// prepare a batch of blocks
	blocks := make([]*TestBlock, 0, 100)
	for i := 0; i < 100; i++ {
		blocks = append(blocks, newTestBlock(uint64(i), fmt.Sprintf("blk-%d", i)))
	}

	// store all blocks by both hash and number
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		for _, b := range blocks {
			if e := StoreBlockbyHash(tx, b); e != nil {
				return e
			}
			if e := StoreBlockbyNumber(tx, BlockNumberBucket, b); e != nil {
				return e
			}
		}
		return nil
	})
	require.NoError(t, err)

	// verify retrieval by hash
	err = db.View(context.Background(), func(tx kv.Tx) error {
		for _, want := range blocks {
			hash := want.Hash()
			got, getErr := GetBlock(tx, BlockHashBucket, hash[:], &TestBlock{})
			if getErr != nil {
				return getErr
			}
			b := asTestBlock(t, got)
			assert.Equal(t, want.H, b.H)
			assert.Equal(t, want.N, b.N)
			assert.Equal(t, want.Data, b.Data)
		}
		return nil
	})
	require.NoError(t, err)

	// verify retrieval by number
	err = db.View(context.Background(), func(tx kv.Tx) error {
		for _, want := range blocks {
			key := NumberToBytes(want.Number())
			got, getErr := GetBlock(tx, BlockNumberBucket, key, &TestBlock{})
			if getErr != nil {
				return getErr
			}
			b := asTestBlock(t, got)
			assert.Equal(t, want.H, b.H)
			assert.Equal(t, want.N, b.N)
			assert.Equal(t, want.Data, b.Data)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGetBlockNonExistent(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockHashBucket, BlockNumberBucket)
	defer db.Close()

	nonExistentHash := sha256.Sum256([]byte("no-such-block"))

	// lookup by hash
	err := db.View(context.Background(), func(tx kv.Tx) error {
		_, getErr := GetBlock(tx, BlockHashBucket, nonExistentHash[:], &TestBlock{})
		assert.Error(t, getErr)
		return nil
	})
	require.NoError(t, err)

	// lookup by number
	key := NumberToBytes(999999)
	err = db.View(context.Background(), func(tx kv.Tx) error {
		_, getErr := GetBlock(tx, BlockNumberBucket, key, &TestBlock{})
		assert.Error(t, getErr)
		return nil
	})
	require.NoError(t, err)
}

func TestNumberToBytesBigEndian(t *testing.T) {
	cases := []struct {
		name string
		in   uint64
	}{
		{"zero", 0},
		{"one", 1},
		{"pattern", 0x0102030405060708},
		{"max", math.MaxUint64},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(tr *testing.T) {
			got := NumberToBytes(tc.in)

			// expected big endian encoding
			var want [8]byte
			binary.BigEndian.PutUint64(want[:], tc.in)

			assert.Equal(tr, want[:], got)
		})
	}
}

