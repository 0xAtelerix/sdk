package block

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
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
