// Package appblock provides helpers for decoding, storing, and inspecting
// application-specific blocks persisted by the SDK.
package appblock

import (
	"fmt"
	"reflect"
	"strings"
)

// AppBlock is a generic wrapper that associates a decoded payload with the block
// number it belongs to. Target holds the payload value used when exporting
// metadata.
type AppBlock[T any] struct {
	BlockNumber uint64
	Target      T
}

// NewAppBlock constructs an AppBlock using the provided block number and target.
func NewAppBlock[T any](blockNumber uint64, target T) *AppBlock[T] {
	return &AppBlock[T]{
		BlockNumber: blockNumber,
		Target:      target,
	}
}

type fieldMetadata struct {
	index int
	name  string
}

// FieldsValues represents decoded struct fields and their stringified values.
type FieldsValues struct {
	Fields []string
	Values []string
}

// ToFieldsAndValues returns the exported field names of the underlying Target. The
// method respects `json` struct tags when present (using the tag name before
// any comma options), falling back to the struct field name otherwise. When the
// target is nil or not a struct (or pointer to a struct) the function returns an error.
func (cb *AppBlock[T]) ToFieldsAndValues() (FieldsValues, error) {
	if cb == nil {
		return FieldsValues{}, errAppBlockValueNil
	}

	value := reflect.ValueOf(cb.Target)
	if !value.IsValid() {
		return FieldsValues{}, errAppBlockValueNil
	}

	elem := value
	for elem.Kind() == reflect.Pointer {
		if elem.IsNil() {
			return FieldsValues{}, errTargetNilPointer
		}

		elem = elem.Elem()
	}

	if elem.Kind() != reflect.Struct {
		return FieldsValues{}, ErrTargetNotStruct
	}

	typ := elem.Type()
	metadata := buildFieldMetadata(typ)
	fields := make([]string, 0, len(metadata))
	values := make([]string, 0, len(metadata))

	for _, meta := range metadata {
		fields = append(fields, meta.name)
		values = append(values, formatValue(elem.Field(meta.index)))
	}

	return FieldsValues{Fields: fields, Values: values}, nil
}

// buildFieldMetadata gathers metadata for exported fields, respecting JSON tags
// when present, so field names can be displayed consistently.
func buildFieldMetadata(typ reflect.Type) []fieldMetadata {
	count := typ.NumField()
	result := make([]fieldMetadata, 0, count)

	for i := range count {
		field := typ.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}

		name := field.Tag.Get("json")
		if name != "" && name != "-" {
			if idx := strings.Index(name, ","); idx != -1 {
				name = name[:idx]
			}
		}

		if name == "" {
			name = field.Name
		}

		result = append(result, fieldMetadata{
			index: i,
			name:  name,
		})
	}

	return result
}

// formatValue converts a reflect.Value into its string representation while
// safely dereferencing pointers and handling nil values.
func formatValue(field reflect.Value) string {
	if !field.IsValid() {
		return ""
	}

	value := field
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}

		value = value.Elem()
	}

	if !value.IsValid() {
		return ""
	}

	return fmt.Sprintf("%v", value.Interface())
}
