package tokens

import (
	"reflect"
	"strings"
	"sync"
)

type fieldPlan struct {
	// map[lowercased abi name] -> reflect index path (supports embedded)
	index map[string][]int
}

// it can be used in a concurrent way
// Memoization
//
//nolint:gochecknoglobals // there's no way to do the same with functions
var planCache sync.Map // map[reflect.Type]*fieldPlan

func getPlan(t reflect.Type) *fieldPlan {
	if v, ok := planCache.Load(t); ok {
		actualPlan, exists := v.(*fieldPlan)

		if exists {
			return actualPlan
		}
	}

	p := &fieldPlan{index: make(map[string][]int)}
	buildPlan(t, nil, p)

	// Ensure only one stored (avoid races)
	if actual, loaded := planCache.LoadOrStore(t, p); loaded {
		actualPlan, ok := actual.(*fieldPlan)

		if ok {
			return actualPlan
		}
	}

	return p
}

// Recursively build plan for exported fields (handles embedded structs).
func buildPlan(t reflect.Type, prefix []int, p *fieldPlan) {
	// Follow pointer-to-struct
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		path := append(append([]int(nil), prefix...), i)

		// If anonymous embedded struct, recurse
		if sf.Anonymous &&
			(sf.Type.Kind() == reflect.Struct || (sf.Type.Kind() == reflect.Pointer && sf.Type.Elem().Kind() == reflect.Struct)) {
			buildPlan(sf.Type, path, p)

			continue
		}

		name := sf.Tag.Get("abi")
		if name == "" {
			name = sf.Name
		}

		p.index[strings.ToLower(name)] = path
	}
}
