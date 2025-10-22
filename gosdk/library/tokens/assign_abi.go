package tokens

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// assignByABI writes name->value into struct fields by `abi:"name"` tag (or field name),
// using a memoized field plan per struct type.
func assignByABI(dst any, args abi.Arguments, values map[string]any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return ErrNonNilRequired
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return ErrStructRequired
	}

	plan := getPlan(rv.Type())

	for _, arg := range args {
		name := strings.ToLower(arg.Name)

		val, ok := values[name]
		if !ok {
			// Missing value (e.g., dynamic indexed types hashed in topics)
			continue
		}

		path, ok := plan.index[name]
		if !ok {
			// destination struct doesn't declare this field
			continue
		}

		dstField := rv.FieldByIndex(path)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}

		src := reflect.ValueOf(val)
		if src.Type().AssignableTo(dstField.Type()) {
			dstField.Set(src)

			continue
		}

		if src.Type().ConvertibleTo(dstField.Type()) {
			dstField.Set(src.Convert(dstField.Type()))

			continue
		}

		return fmt.Errorf(
			"%w for %q: have %v want %v",
			ErrABIPlanTypeMismatched,
			arg.Name,
			src.Type(),
			dstField.Type(),
		)
	}

	return nil
}

func indexed(arguments abi.Arguments) abi.Arguments {
	var ret []abi.Argument

	for _, arg := range arguments {
		if arg.Indexed {
			ret = append(ret, arg)
		}
	}

	return ret
}
