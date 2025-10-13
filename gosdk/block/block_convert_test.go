package block

// import (
// 	"fmt"
// 	"reflect"
// 	"testing"
// )

// // filled returns a [32]byte array filled with byte b.
// func filled(b byte) [32]byte {
// 	var a [32]byte
// 	for i := range a {
// 		a[i] = b
// 	}
// 	return a
// }

// func TestConvertToFieldsValues_HappyPath(t *testing.T) {
// 	b := &Block{
// 		BlockNumber: 777,
// 		BlockHash:   filled(0x11),
// 		BlockRoot:   filled(0x22),
// 		Timestamp:   1699999999,
// 	}

// 	got := b.convertToFieldsValues()

// 	// 1) Fields must be the JSON tags in declaration order.
// 	//wantFields := []string{"number", "hash", "stateroot", "timestamp"}
// 	// if !reflect.DeepEqual(got.Fields, wantFields) {
// 	// 	t.Fatalf("Fields mismatch:\n got:  %#v\n want: %#v", got.Fields, wantFields)
// 	// }

// 	// 2) Values must align with fields and use the AppchainBlock methods + formatting.
// 	wantValues := []string{
// 		fmt.Sprintf("%d", b.BlockNumber),
// 		fmt.Sprintf("0x%x", b.BlockHash),
// 		fmt.Sprintf("0x%x", b.BlockRoot),
// 		fmt.Sprintf("%d", b.Timestamp),
// 	}
// 	if !reflect.DeepEqual(got.Values, wantValues) {
// 		t.Fatalf("Values mismatch:\n got:  %#v\n want: %#v", got.Values, wantValues)
// 	}
// }

// // func TestConvertToFieldsValues_ZeroBlock(t *testing.T) {
// // 	b := &Block{} // zero-value block
// // 	got := b.convertToFieldsValues()

// // 	// Always produce 4 fields / 4 values
// // 	if len(got.Fields) != 4 || len(got.Values) != 4 {
// // 		t.Fatalf("expected 4 fields & 4 values, got %d and %d", len(got.Fields), len(got.Values))
// // 	}

// // 	// Field order must be stable (JSON tags in declaration order)
// // 	wantFields := []string{"number", "hash", "stateroot", "timestamp"}
// // 	if !reflect.DeepEqual(got.Fields, wantFields) {
// // 		t.Fatalf("Fields mismatch on zero block:\n got:  %#v\n want: %#v", got.Fields, wantFields)
// // 	}

// // 	// Numeric values are "0"
// // 	if got.Values[0] != "0" {
// // 		t.Fatalf("number: expected 0, got %q", got.Values[0])
// // 	}
// // 	if got.Values[3] != "0" {
// // 		t.Fatalf("timestamp: expected 0, got %q", got.Values[3])
// // 	}

// // 	// Hex values must be "0x" + 64 hex chars (32 bytes), lowercase.
// // 	if v := got.Values[1]; len(v) != 66 || v[:2] != "0x" {
// // 		t.Fatalf("hash hex format: expected 0x-prefixed 64 hex chars, got %q", v)
// // 	}
// // 	if v := got.Values[2]; len(v) != 66 || v[:2] != "0x" {
// // 		t.Fatalf("stateroot hex format: expected 0x-prefixed 64 hex chars, got %q", v)
// // 	}
// // }

// // Optional: makes the "pointer reflect" bug explicit.
// // If convertToFieldsValues panics, this test will fail with a clearer message.
// // func TestConvertToFieldsValues_DoesNotPanic(t *testing.T) {
// // 	b := &Block{
// // 		BlockNumber: 1,
// // 		BlockHash:   filled(0xAB),
// // 		BlockRoot:   filled(0xCD),
// // 		Timestamp:   42,
// // 	}
// // 	defer func() {
// // 		if r := recover(); r != nil {
// // 			t.Fatalf("convertToFieldsValues panicked: %v", r)
// // 		}
// // 	}()
// // 	_ = b.convertToFieldsValues()
// // }
