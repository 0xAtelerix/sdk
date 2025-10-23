package appblock

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrMissingBlockTemplate = errors.New("block template not configured")
	ErrUnsupportedPayload   = errors.New("unsupported block payload type")
)

// CloneTarget returns a new zero-valued instance of the provided pointer
// template, ensuring the original template remains untouched.
func CloneTarget[T any](tpl T) (T, error) {
	var zero T

	val := reflect.ValueOf(tpl)
	if !val.IsValid() {
		return zero, ErrMissingBlockTemplate
	}

	typ := val.Type()
	if typ.Kind() != reflect.Pointer {
		return zero, fmt.Errorf("%w: %T", ErrUnsupportedPayload, tpl)
	}

	newVal := reflect.New(typ.Elem())

	result, ok := newVal.Interface().(T)
	if !ok {
		return zero, fmt.Errorf("%w: %T", ErrUnsupportedPayload, tpl)
	}

	return result, nil
}

// structValueFrom unwraps pointer chains on target and returns the underlying
// struct value if present, signalling success via the second return value.
func structValueFrom(target any) (reflect.Value, bool) {
	value := reflect.ValueOf(target)
	if !value.IsValid() {
		return reflect.Value{}, false
	}

	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, false
		}

		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	return value, true
}

// txsField returns the exported `Txs` slice field from the supplied struct value,
// if it exists and is a slice; otherwise it reports false.
func txsField(value reflect.Value) (reflect.Value, bool) {
	field := value.FieldByName("Txs")
	if !field.IsValid() || field.Kind() != reflect.Slice {
		return reflect.Value{}, false
	}

	return field, true
}

// ExtractTransactions returns a copy of the target's exported Txs field if it is
// a slice compatible with AppTx. The returned bool indicates whether the field
// existed and matched the expected type, and the second bool reports whether the
// field was nil.
func ExtractTransactions[AppTx any](target any) (txs []AppTx, hasField bool, fieldNil bool) {
	structValue, ok := structValueFrom(target)
	if !ok {
		return nil, false, false
	}

	field, ok := txsField(structValue)
	if !ok || !field.CanInterface() {
		return nil, false, false
	}

	var zero []AppTx

	expectedType := reflect.TypeOf(zero)
	if expectedType == nil || !field.Type().AssignableTo(expectedType) {
		return nil, false, false
	}

	slice, ok := field.Interface().([]AppTx)
	if !ok {
		return nil, false, false
	}

	return append([]AppTx(nil), slice...), true, field.IsNil()
}
