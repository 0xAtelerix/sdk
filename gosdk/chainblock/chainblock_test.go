package chainblock

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
)

func TestChainBlockToFields_EthereumBlock(t *testing.T) {
	header := &gethtypes.Header{Number: big.NewInt(42)}
	block := gethtypes.NewBlock(header, nil, nil, nil)

	cb := &ChainBlock[gethtypes.Block]{
		ChainType: gosdk.EthereumChainID,
		Target:    block,
	}

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

func TestChainBlockToFields_SolanaBlock(t *testing.T) {
	sol := &client.Block{Blockhash: "hash", PreviousBlockhash: "prev"}

	cb := &ChainBlock[client.Block]{
		ChainType: gosdk.SolanaChainID,
		Target:    sol,
	}

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

func TestChainBlockToFields_CustomStruct(t *testing.T) {
	type custom struct {
		ID     string `json:"id"`
		Height int
		note   string
	}

	payload := &custom{ID: "abc", Height: 5}

	cb := &ChainBlock[custom]{
		ChainType: gosdk.EthereumChainID,
		Target:    payload,
	}

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

func TestNewChainBlock_Success(t *testing.T) {
	payload := &struct{ Name string }{Name: "Alice"}

	cb, err := NewChainBlock(gosdk.EthereumChainID, payload)
	require.NoError(t, err)
	require.NotNil(t, cb)
	require.Equal(t, gosdk.EthereumChainID, cb.ChainType)
	require.Same(t, payload, cb.Target)
}

func TestNewChainBlock_NilTarget(t *testing.T) {
	cb, err := NewChainBlock[string](gosdk.EthereumChainID, nil)
	require.Error(t, err)
	require.Nil(t, cb)
}

func TestChainBlockToFields_NilTarget(t *testing.T) {
	cb := &ChainBlock[struct{}]{ChainType: gosdk.EthereumChainID, Target: nil}

	fv := cb.ToFieldsAndValues()
	require.Nil(t, fv.Fields)
	require.Nil(t, fv.Values)
}

func TestChainBlockToFields_NonStructPointer(t *testing.T) {
	value := "hello"
	cb := &ChainBlock[string]{ChainType: gosdk.EthereumChainID, Target: &value}

	fv := cb.ToFieldsAndValues()
	require.Nil(t, fv.Fields)
	require.Nil(t, fv.Values)
}

func TestChainBlockToFields_PointerFields(t *testing.T) {
	type sample struct {
		Name  *string
		Count *int
	}

	name := "bob"
	count := 7
	payload := &sample{Name: &name, Count: &count}

	cb := &ChainBlock[sample]{ChainType: gosdk.EthereumChainID, Target: payload}

	fv := cb.ToFieldsAndValues()
	require.Equal(
		t,
		map[string]string{"Name": name, "Count": "7"},
		pairMap(fv),
	)
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

		out[name] = stringify(elem.Field(i))
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
