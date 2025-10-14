package block

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/stretchr/testify/require"
)

func TestGetBlocks(t *testing.T) {
	t.Parallel()

	t.Run("zero count returns empty slice", func(tr *testing.T) {
		slice, err := GetBlocks(nil, 0)
		require.NoError(tr, err)
		require.Empty(tr, slice)
	})

	t.Run("empty bucket returns error", func(tr *testing.T) {
		db := createDB(tr, BlockNumberBucket)
		defer db.Close()

		require.NoError(tr, db.View(context.Background(), func(tx kv.Tx) error {
			_, err := GetBlocks(tx, 1)
			require.ErrorIs(tr, err, ErrNoBlocks)

			return nil
		}))
	})

	t.Run("returns newest first", func(tr *testing.T) {
		db := createDB(tr, BlockNumberBucket)
		defer db.Close()

		blocks := []*Block[testTx, testReceiptError]{
			buildBlock(1, filled(0x01), 100, nil),
			buildBlock(2, filled(0x02), 200, nil),
			buildBlock(5, filled(0x05), 500, nil),
		}

		require.NoError(tr, db.Update(context.Background(), func(tx kv.RwTx) error {
			for _, blk := range blocks {
				blk.Hash()

				payload := encodeBlock(blk)
				require.NotNil(tr, payload)

				if err := tx.Put(BlockNumberBucket, NumberToBytes(blk.BlockNumber), payload); err != nil {
					return err
				}
			}

			return nil
		}))

		require.NoError(tr, db.View(context.Background(), func(tx kv.Tx) error {
			fvSlice, err := GetBlocks(tx, 5)
			require.NoError(tr, err)
			require.Len(tr, fvSlice, len(blocks))

			expectedNumbers := []string{"5", "2", "1"}
			expectedFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}

			for i, fv := range fvSlice {
				require.Equal(tr, expectedNumbers[i], fv.Values[0])
				require.Equal(tr, expectedFields, fv.Fields)
			}

			return nil
		}))
	})
}

func TestGetBlock(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	t.Run("store and retrieve single block", func(tr *testing.T) {
		block := buildBlock(42, filled(0x42), 1_700_000_000, nil)
		hash := block.Hash()

		payload := encodeBlock(block)
		require.NotNil(tr, payload)

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
		blocks := []*Block[testTx, testReceiptError]{
			buildBlock(1, filled(0x01), 100, nil),
			buildBlock(2, filled(0x02), 200, nil),
			buildBlock(3, filled(0x03), 300, nil),
		}

		require.NoError(tr, db.Update(tr.Context(), func(tx kv.RwTx) error {
			for _, blk := range blocks {
				blk.Hash()

				payload := encodeBlock(blk)
				require.NotNil(tr, payload)

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
	blk *Block[testTx, testReceiptError],
	hash [32]byte,
) {
	tb.Helper()

	expectedFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}
	require.Equal(tb, fmt.Sprint(expectedFields), fmt.Sprint(fv.Fields))

	expectedValues := []string{
		strconv.FormatUint(blk.BlockNumber, 10),
		fmt.Sprintf("0x%x", hash),
		fmt.Sprintf("0x%x", expectedStateRoot(blk)),
		strconv.FormatUint(blk.Timestamp, 10),
		strconv.Itoa(len(blk.Transactions)),
	}
	require.Equal(tb, fmt.Sprint(expectedValues), fmt.Sprint(fv.Values))
}

func TestGetTransactionsForBlockNumber(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockNumberBucket)
	defer db.Close()

	block := buildBlock(77, filled(0x77), 1111, []testTx{newTestTx(0x01), newTestTx(0x02)})
	block.Hash()
	payload := encodeBlock(block)
	require.NotNil(t, payload)

	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.Put(BlockNumberBucket, NumberToBytes(block.BlockNumber), payload)
	}))

	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
		res, err := GetTransactionsForBlockNumber[testTx](tx, block.BlockNumber, testTx{})
		require.NoError(t, err)
		require.Len(t, res, len(block.Transactions))

		for i, tx := range res {
			require.Equal(t, block.Transactions[i].HashValue, tx.Hash())
		}

		return nil
	}))

	t.Run("non-existent block", func(tr *testing.T) {
		require.NoError(tr, db.View(context.Background(), func(tx kv.Tx) error {
			_, err := GetTransactionsForBlockNumber[testTx](tx, 999, testTx{})
			require.ErrorIs(tr, err, ErrNoBlocks)

			return nil
		}))
	})

	t.Run("empty transactions slice", func(tr *testing.T) {
		empty := buildBlock(88, filled(0x88), 2222, nil)
		empty.Hash()
		payloadEmpty := encodeBlock(empty)
		require.NotNil(tr, payloadEmpty)

		require.NoError(tr, db.Update(context.Background(), func(tx kv.RwTx) error {
			return tx.Put(BlockNumberBucket, NumberToBytes(empty.BlockNumber), payloadEmpty)
		}))

		require.NoError(tr, db.View(context.Background(), func(tx kv.Tx) error {
			res, err := GetTransactionsForBlockNumber[testTx](tx, empty.BlockNumber, testTx{})
			require.NoError(tr, err)
			require.Empty(tr, res)

			return nil
		}))
	})
}

func TestGetTransactionsForBlockHash(t *testing.T) {
	t.Parallel()

	db := createDB(t, BlockHashBucket)
	defer db.Close()

	block := buildBlock(91, filled(0x91), 3333, []testTx{newTestTx(0x09)})
	hash := block.Hash()
	payload := encodeBlock(block)
	require.NotNil(t, payload)

	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.Put(BlockHashBucket, hash[:], payload)
	}))

	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
		res, err := GetTransactionsForBlockHash[testTx](tx, hash, testTx{})
		require.NoError(t, err)
		require.Len(t, res, len(block.Transactions))

		for i, tx := range res {
			require.Equal(t, block.Transactions[i].HashValue, tx.Hash())
		}

		return nil
	}))

	t.Run("non-existent block hash", func(tr *testing.T) {
		require.NoError(tr, db.View(context.Background(), func(tx kv.Tx) error {
			_, err := GetTransactionsForBlockHash[testTx](tx, filled(0x00), testTx{})
			require.ErrorIs(tr, err, ErrNoBlocks)

			return nil
		}))
	})

	t.Run("empty transactions slice", func(tr *testing.T) {
		empty := buildBlock(92, filled(0x92), 4444, nil)
		emptyHash := empty.Hash()
		payloadEmpty := encodeBlock(empty)
		require.NotNil(t, payloadEmpty)

		require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
			return tx.Put(BlockHashBucket, emptyHash[:], payloadEmpty)
		}))

		require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
			res, err := GetTransactionsForBlockHash[testTx](tx, emptyHash, testTx{})
			require.NoError(tr, err)
			require.Empty(tr, res)

			return nil
		}))
	})
}
