package appblock

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestAppBlockToFields_EthereumBlock(t *testing.T) {
	header := &gethtypes.Header{Number: big.NewInt(42)}
	block := gethtypes.NewBlock(header, nil, nil, nil)

	cb := &AppBlock[*gethtypes.Block]{BlockNumber: 42, Target: block}

	fv := cb.ToFieldsAndValues()
	require.Equalf(
		t,
		expectedFieldMap(t, block),
		pairMap(fv),
		"expected fields/values %v but got fields %v values %v",
		expectedFieldMap(t, block),
		fv.Fields,
		fv.Values,
	)
}

func TestAppBlockToFields_SolanaBlock(t *testing.T) {
	sol := &client.Block{Blockhash: "hash", PreviousBlockhash: "prev"}

	cb := &AppBlock[*client.Block]{BlockNumber: 99, Target: sol}

	fv := cb.ToFieldsAndValues()
	require.Equalf(
		t,
		expectedFieldMap(t, sol),
		pairMap(fv),
		"expected fields/values %v but got fields %v values %v",
		expectedFieldMap(t, sol),
		fv.Fields,
		fv.Values,
	)
}

func TestAppBlockToFields_CustomStruct(t *testing.T) {
	type custom struct {
		ID     string `json:"id"`
		Height int
		note   string
	}

	payload := &custom{ID: "abc", Height: 5}

	cb := &AppBlock[*custom]{BlockNumber: 5, Target: payload}

	fv := cb.ToFieldsAndValues()
	require.Equalf(
		t,
		expectedFieldMap(t, payload),
		pairMap(fv),
		"expected fields/values %v but got fields %v values %v",
		expectedFieldMap(t, payload),
		fv.Fields,
		fv.Values,
	)
}

func TestNewAppBlock_Success(t *testing.T) {
	payload := &struct{ Name string }{Name: "Alice"}

	cb := NewAppBlock(uint64(7), payload)
	require.NotNil(t, cb)
	require.Equal(t, uint64(7), cb.BlockNumber)
	require.Same(t, payload, cb.Target)
}

func TestNewAppBlock_NilTarget(t *testing.T) {
	cb := NewAppBlock[*string](uint64(7), nil)
	require.NotNil(t, cb)
	require.Nil(t, cb.Target)
}

func TestAppBlockToFields_NilTarget(t *testing.T) {
	cb := &AppBlock[*struct{}]{BlockNumber: 1, Target: nil}

	fv := cb.ToFieldsAndValues()
	require.Nil(t, fv.Fields)
	require.Nil(t, fv.Values)
}

func TestAppBlockToFields_NonStructPointer(t *testing.T) {
	value := "hello"
	cb := &AppBlock[*string]{BlockNumber: 1, Target: &value}

	fv := cb.ToFieldsAndValues()
	require.Nil(t, fv.Fields)
	require.Nil(t, fv.Values)
}

func TestAppBlockToFields_PointerFields(t *testing.T) {
	type sample struct {
		Name  *string
		Count *int
	}

	name := "bob"
	count := 7
	payload := &sample{Name: &name, Count: &count}

	cb := &AppBlock[*sample]{BlockNumber: 12, Target: payload}

	fv := cb.ToFieldsAndValues()
	// fmt.Sprintf prints ints without quotes, so value remains numeric string
	require.Equal(t, map[string]string{"Name": name, "Count": "7"}, pairMap(fv))
}

func TestAppBlockToFields_ValueStruct(t *testing.T) {
	type value struct {
		Height int
		Label  string
	}

	payload := value{Height: 3, Label: "node"}
	cb := &AppBlock[value]{BlockNumber: 2, Target: payload}

	fv := cb.ToFieldsAndValues()
	require.Equal(t, map[string]string{"Height": "3", "Label": "node"}, pairMap(fv))
}

func TestAppBlockToFields_NestedPointers(t *testing.T) {
	type nested struct {
		Label *string
		Child **int
	}

	var (
		label *string
		child *int
	)

	ptr := &child

	payload := &nested{Label: label, Child: ptr}
	cb := &AppBlock[*nested]{BlockNumber: 8, Target: payload}

	fv := cb.ToFieldsAndValues()
	require.Equal(t, map[string]string{"Label": "", "Child": ""}, pairMap(fv))
}

func TestAppBlockToFields_InterfaceFields(t *testing.T) {
	type nested struct {
		Value int
	}

	type sample struct {
		Data any
		List []string
	}

	payload := &sample{
		Data: nested{Value: 11},
		List: []string{"alpha", "beta"},
	}

	cb := &AppBlock[*sample]{BlockNumber: 19, Target: payload}

	fv := cb.ToFieldsAndValues()
	require.Equal(t, map[string]string{
		"Data": "{11}",
		"List": "[alpha beta]",
	}, pairMap(fv))
}

func expectedFieldMap(t *testing.T, target any) map[string]string {
	t.Helper()

	if target == nil {
		return nil
	}

	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		t.Fatal("expected non-nil pointer target")
	}

	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		t.Fatal("expected pointer to struct")
	}

	typ := elem.Type()
	out := make(map[string]string, typ.NumField())

	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name := field.Tag.Get("json")
		if name != "" && name != "-" {
			name = strings.Split(name, ",")[0]
		}

		if name == "" {
			name = field.Name
		}

		out[name] = formatValue(elem.Field(i))
	}

	return out
}

func pairMap(fv FieldsValues) map[string]string {
	if len(fv.Fields) != len(fv.Values) {
		return nil
	}

	out := make(map[string]string, len(fv.Fields))
	for i, field := range fv.Fields {
		out[field] = fv.Values[i]
	}

	return out
}
