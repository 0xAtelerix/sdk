package block

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

func TestBlockConvertToFieldsValues_HappyPath(t *testing.T) {
	t.Helper()

	b := &Block[testTx, testReceipt]{
		BlockNumber:  777,
		BlockHash:    filled(0x11),
		BlockRoot:    filled(0x22),
		Timestamp:    1699999999,
		Transactions: []testTx{{hash: filled(0x33)}, {hash: filled(0x44)}},
	}

	if got, want := b.Hash(), expectedHash(b); got != want {
		t.Fatalf("hash mismatch: got %x want %x", got, want)
	}

	got := b.convertToFieldsValues()

	wantFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}
	if !reflect.DeepEqual(got.Fields, wantFields) {
		t.Fatalf("fields mismatch:\n got:  %#v\n want: %#v", got.Fields, wantFields)
	}

	wantValues := []string{
		strconv.FormatUint(b.BlockNumber, 10),
		fmt.Sprintf("0x%x", b.Hash()),
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

	b := &Block[testTx, testReceipt]{} // zero-value block

	if got, want := b.Hash(), expectedHash(b); got != want {
		t.Fatalf("hash mismatch on zero block: got %x want %x", got, want)
	}

	got := b.convertToFieldsValues()

	wantFields := []string{"number", "hash", "stateroot", "timestamp", "transactions"}
	if !reflect.DeepEqual(got.Fields, wantFields) {
		t.Fatalf("fields mismatch on zero block:\n got:  %#v\n want: %#v", got.Fields, wantFields)
	}

	wantValues := []string{
		"0",
		fmt.Sprintf("0x%x", b.Hash()),
		fmt.Sprintf("0x%x", b.StateRoot()),
		"0",
		"0",
	}
	if !reflect.DeepEqual(got.Values, wantValues) {
		t.Fatalf("values mismatch on zero block:\n got:  %#v\n want: %#v", got.Values, wantValues)
	}
}
