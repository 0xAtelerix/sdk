package block

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleBlock() *Block[testTx, testReceipt] {
	return buildBlock(123, filled(0xAA), 456789, []testTx{newTestTx(1), newTestTx(2)})
}

func TestBlockNumber(t *testing.T) {
	t.Parallel()

	var zero *Block[testTx, testReceipt]
	require.Equal(t, uint64(0), zero.Number())

	b := sampleBlock()
	require.Equal(t, b.BlockNumber, b.Number())
}

func TestBlockHashIncludesTransactions(t *testing.T) {
	t.Parallel()

	var zero *Block[testTx, testReceipt]
	require.Equal(t, [32]byte{}, zero.Hash())

	b := sampleBlock()
	want := expectedHash(b)

	got := b.Hash()
	require.Equal(t, want, got)
	require.Equal(t, want, b.BlockHash)
	require.Equal(t, want, b.Hash())
}

func TestBlockStateRoot(t *testing.T) {
	t.Parallel()

	var zero *Block[testTx, testReceipt]
	require.Equal(t, [32]byte{}, zero.StateRoot())

	b := sampleBlock()
	b.Hash()

	require.Equal(t, expectedStateRoot(b), b.StateRoot())
}

func TestBlockBytes(t *testing.T) {
	t.Parallel()

	var zero *Block[testTx, testReceipt]
	require.Nil(t, zero.Bytes())

	b := sampleBlock()
	b.Hash()
	want := encodeBlock(b)
	require.Equal(t, string(want), string(b.Bytes()))
}

func TestBlockConvertToFieldsValues(t *testing.T) {
	t.Parallel()

	b := sampleBlock()
	b.Hash()

	got := b.convertToFieldsValues()
	wantFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}
	require.Equal(t, fmt.Sprint(wantFields), fmt.Sprint(got.Fields))

	want := []string{
		strconv.FormatUint(b.BlockNumber, 10),
		fmt.Sprintf("0x%x", expectedHash(b)),
		fmt.Sprintf("0x%x", expectedStateRoot(b)),
		strconv.FormatUint(b.Timestamp, 10),
		strconv.Itoa(len(b.Transactions)),
	}
	require.Equal(t, fmt.Sprint(want), fmt.Sprint(got.Values))

	zeroTemplate := buildBlock(0, [32]byte{}, 0, nil)
	zeroTemplate.Hash()

	var zero *Block[testTx, testReceipt]

	gotZero := zero.convertToFieldsValues()
	require.Equal(t, fmt.Sprint(wantFields), fmt.Sprint(gotZero.Fields))

	wantZero := []string{
		"0",
		fmt.Sprintf("0x%x", expectedHash(zeroTemplate)),
		fmt.Sprintf("0x%x", expectedStateRoot(zeroTemplate)),
		"0",
		"0",
	}
	require.Equal(t, fmt.Sprint(wantZero), fmt.Sprint(gotZero.Values))
}

func createDB(tb testing.TB, buckets ...string) kv.RwDB {
	tb.Helper()
	db := memdb.NewTestDB(tb)
	require.NoError(tb, db.Update(context.Background(), func(tx kv.RwTx) error {
		for _, bucket := range buckets {
			if err := tx.CreateBucket(bucket); err != nil {
				return err
			}
		}

		return nil
	}))

	return db
}

func TestGetBlock(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	t.Run("store and retrieve single block", func(tr *testing.T) {
		block := buildBlock(42, filled(0x42), 1_700_000_000, nil)
		hash := block.Hash()
		payload := encodeBlock(block)
		key := NumberToBytes(block.BlockNumber)

		require.NoError(tr, db.Update(tr.Context(), func(tx kv.RwTx) error {
			return tx.Put(BlockNumberBucket, key, payload)
		}))

		require.NoError(tr, db.View(tr.Context(), func(tx kv.Tx) error {
			fv, err := GetBlock(tx, BlockNumberBucket, key, nil)
			if err != nil {
				return err
			}

			requireFieldsValues(tr, fv, block, hash)

			return nil
		}))
	})

	t.Run("store multiple blocks", func(tr *testing.T) {
		blocks := []*Block[testTx, testReceipt]{
			buildBlock(1, filled(0x01), 100, nil),
			buildBlock(2, filled(0x02), 200, nil),
			buildBlock(3, filled(0x03), 300, nil),
		}

		require.NoError(tr, db.Update(tr.Context(), func(tx kv.RwTx) error {
			for _, blk := range blocks {
				blk.Hash()

				payload := encodeBlock(blk)
				if err := tx.Put(BlockNumberBucket, NumberToBytes(blk.BlockNumber), payload); err != nil {
					return err
				}
			}

			return nil
		}))

		require.NoError(tr, db.View(tr.Context(), func(tx kv.Tx) error {
			for _, blk := range blocks {
				fv, err := GetBlock(tx, BlockNumberBucket, NumberToBytes(blk.BlockNumber), nil)
				if err != nil {
					return err
				}

				requireFieldsValues(tr, fv, blk, blk.BlockHash)
			}

			return nil
		}))
	})

	t.Run("non-existent block", func(tr *testing.T) {
		require.NoError(tr, db.View(tr.Context(), func(tx kv.Tx) error {
			_, err := GetBlock(tx, BlockNumberBucket, NumberToBytes(999), nil)
			require.ErrorIs(tr, err, ErrNoBlocks)

			return nil
		}))
	})
}

func requireFieldsValues(
	tb testing.TB,
	fv FieldsValues,
	blk *Block[testTx, testReceipt],
	hash [32]byte,
) {
	tb.Helper()

	wantFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}
	assert.Equal(tb, fmt.Sprint(wantFields), fmt.Sprint(fv.Fields))

	want := []string{
		strconv.FormatUint(blk.BlockNumber, 10),
		fmt.Sprintf("0x%x", hash),
		fmt.Sprintf("0x%x", expectedStateRoot(blk)),
		strconv.FormatUint(blk.Timestamp, 10),
		strconv.Itoa(len(blk.Transactions)),
	}
	assert.Equal(tb, fmt.Sprint(want), fmt.Sprint(fv.Values))
}
