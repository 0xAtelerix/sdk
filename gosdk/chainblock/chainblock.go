package chainblock

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// ChainBlock is a generic wrapper that associates a decoded payload with the
// chain identifier it belongs to. Target must point to a struct so it can be
// introspected for field names when exporting metadata.
type ChainBlock[T any] struct {
	ChainType apptypes.ChainType
	Target    *T
}

// ErrNilTarget is returned when a nil payload pointer is provided.
var ErrNilTarget = errors.New("target cannot be nil")

// FieldsValues represents decoded struct fields and their stringified values.
type FieldsValues struct {
	Fields []string
	Values []string
}

// NewChainBlock constructs a ChainBlock ensuring the payload pointer is non-nil.
func NewChainBlock[T any](chainType apptypes.ChainType, target *T) (*ChainBlock[T], error) {
	if target == nil {
		return nil, ErrNilTarget
	}

	return &ChainBlock[T]{
		ChainType: chainType,
		Target:    target,
	}, nil
}

// ToFieldsAndValues returns the exported field names of the underlying Target. The
// method respects `json` struct tags when present (using the tag name before
// any comma options), falling back to the struct field name otherwise. When the
// target is nil or not a struct pointer the function returns nil.
func (cb *ChainBlock[T]) ToFieldsAndValues() FieldsValues {
	if cb == nil || cb.Target == nil {
		return FieldsValues{}
	}

	value := reflect.ValueOf(cb.Target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return FieldsValues{}
	}

	elem := value.Elem()
	if elem.Kind() != reflect.Struct {
		return FieldsValues{}
	}

	typ := elem.Type()
	fields := make([]string, 0, typ.NumField())
	values := make([]string, 0, typ.NumField())

	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}

		name := field.Tag.Get("json")
		if name != "" && name != "-" {
			name = strings.Split(name, ",")[0]
		}

		if name == "" {
			name = field.Name
		}

		fields = append(fields, name)
		values = append(values, stringify(elem.Field(i)))
	}

	return FieldsValues{Fields: fields, Values: values}
}

func stringify(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}

		return stringify(v.Elem())
	}

	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Array,
		reflect.Slice,
		reflect.Map,
		reflect.Struct,
		reflect.Interface,
		reflect.Pointer,
		reflect.UnsafePointer,
		reflect.Chan,
		reflect.Func,
		reflect.Complex64,
		reflect.Complex128:
		return fmt.Sprint(v.Interface())
	case reflect.Invalid:
		return ""
	}

	return ""
}
