package block

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockConvertToFieldsValues_HappyPath(t *testing.T) {
	t.Helper()

	b := &Block[testTx, testReceiptError]{
		BlockNumber:  777,
		BlockHash:    filled(0x11),
		BlockRoot:    filled(0x22),
		Timestamp:    1699999999,
		Transactions: []testTx{{HashValue: filled(0x33)}, {HashValue: filled(0x44)}},
	}

	hash := b.Hash()
	require.Equal(t, expectedHash(b), hash)
	require.Equal(t, expectedHash(b), b.BlockHash)

	got, err := b.convertToFieldsValues()
	require.NoError(t, err)

	wantFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}
	if !reflect.DeepEqual(got.Fields, wantFields) {
		t.Fatalf("fields mismatch:\n got:  %#v\n want: %#v", got.Fields, wantFields)
	}

	wantValues := []string{
		strconv.FormatUint(b.BlockNumber, 10),
		fmt.Sprintf("0x%x", hash),
		fmt.Sprintf("0x%x", b.StateRoot()),
		strconv.FormatUint(b.Timestamp, 10),
		strconv.FormatUint(uint64(len(b.Transactions)), 10),
	}
	if !reflect.DeepEqual(got.Values, wantValues) {
		t.Fatalf("values mismatch:\n got:  %#v\n want: %#v", got.Values, wantValues)
	}
}

func TestBlockConvertToFieldsValues_ZeroBlock(t *testing.T) {
	t.Helper()

	b := &Block[testTx, testReceiptError]{} // zero-value block

	zeroHash := b.Hash()
	require.Equal(t, expectedHash(b), zeroHash)

	got, err := b.convertToFieldsValues()
	require.NoError(t, err)

	wantFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}
	if !reflect.DeepEqual(got.Fields, wantFields) {
		t.Fatalf("fields mismatch on zero block:\n got:  %#v\n want: %#v", got.Fields, wantFields)
	}

	wantValues := []string{
		"0",
		fmt.Sprintf("0x%x", zeroHash),
		fmt.Sprintf("0x%x", b.StateRoot()),
		"0",
		"0",
	}
	if !reflect.DeepEqual(got.Values, wantValues) {
		t.Fatalf("values mismatch on zero block:\n got:  %#v\n want: %#v", got.Values, wantValues)
	}
}
