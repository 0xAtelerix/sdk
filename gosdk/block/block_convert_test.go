package block

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
)

// filled returns a [32]byte array populated with the provided byte.
func filled(b byte) [32]byte {
	var a [32]byte
	for i := range a {
		a[i] = b
	}

	return a
}

func TestBlockConvertToFieldsValues_HappyPath(t *testing.T) {
	t.Helper()

	b := &Block{
		BlockNumber: 777,
		BlockHash:   filled(0x11),
		BlockRoot:   filled(0x22),
		Timestamp:   1699999999,
	}

	got := b.convertToFieldsValues()

	wantFields := []string{"number", "hash", "stateroot", "timestamp"}
	if !reflect.DeepEqual(got.Fields, wantFields) {
		t.Fatalf("fields mismatch:\n got:  %#v\n want: %#v", got.Fields, wantFields)
	}

	wantValues := []string{
		strconv.FormatUint(b.BlockNumber, 10),
		fmt.Sprintf("0x%x", b.Hash()),
		fmt.Sprintf("0x%x", b.StateRoot()),
		strconv.FormatUint(b.Timestamp, 10),
	}
	if !reflect.DeepEqual(got.Values, wantValues) {
		t.Fatalf("values mismatch:\n got:  %#v\n want: %#v", got.Values, wantValues)
	}
}

func TestBlockConvertToFieldsValues_ZeroBlock(t *testing.T) {
	t.Helper()

	b := &Block{} // zero-value block
	got := b.convertToFieldsValues()

	wantFields := []string{"number", "hash", "stateroot", "timestamp"}
	if !reflect.DeepEqual(got.Fields, wantFields) {
		t.Fatalf("fields mismatch on zero block:\n got:  %#v\n want: %#v", got.Fields, wantFields)
	}

	wantValues := []string{
		"0",
		fmt.Sprintf("0x%x", b.Hash()),
		fmt.Sprintf("0x%x", b.StateRoot()),
		"0",
	}
	if !reflect.DeepEqual(got.Values, wantValues) {
		t.Fatalf("values mismatch on zero block:\n got:  %#v\n want: %#v", got.Values, wantValues)
	}
}
