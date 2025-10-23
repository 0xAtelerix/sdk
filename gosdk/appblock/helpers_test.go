package appblock

import (
	"crypto/sha256"
	"reflect"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type testReceipt struct{}

func (testReceipt) TxHash() [32]byte                 { return [32]byte{} }
func (testReceipt) Status() apptypes.TxReceiptStatus { return apptypes.ReceiptConfirmed }
func (testReceipt) Error() string                    { return "" }

type testInjectTx[R testReceipt] struct {
	From  string
	Value int
}

func (t testInjectTx[R]) Hash() [32]byte {
	return sha256.Sum256([]byte(t.From + strconv.Itoa(t.Value)))
}

func (testInjectTx[R]) Process(kv.RwTx) (R, []apptypes.ExternalTransaction, error) {
	var r R

	return r, nil, nil
}

type templateWithTxs struct {
	Txs []testInjectTx[testReceipt]
}

type templateWithPointerTxs struct {
	Txs *[]testInjectTx[testReceipt]
}

type templateWithoutTxs struct {
	Number string
}

type numberProvider interface {
	Number() string
}

type interfaceBlock struct {
	value string
}

func (b *interfaceBlock) Number() string {
	return b.value
}

func TestCloneTarget(t *testing.T) {
	template := templateWithTxs{}

	cloned, err := CloneTarget(&template)
	require.NoError(t, err)
	require.NotNil(t, cloned)
	require.NotSame(t, &template, cloned)
}

func TestCloneTarget_InvalidTemplate(t *testing.T) {
	cloned, err := CloneTarget(templateWithTxs{})
	require.Error(t, err)
	require.Equal(t, templateWithTxs{}, cloned)
}

func TestCloneTarget_NilPointerTemplate(t *testing.T) {
	var template *templateWithTxs

	cloned, err := CloneTarget(template)
	require.NoError(t, err)
	require.NotNil(t, cloned)
}

func TestCloneTarget_InterfacePointerTemplate(t *testing.T) {
	var template numberProvider = &interfaceBlock{value: "42"}

	cloned, err := CloneTarget(template)
	require.NoError(t, err)
	require.NotSame(t, template, cloned)

	casted, ok := cloned.(*interfaceBlock)
	require.True(t, ok)
	require.Empty(t, casted.Number())
}

func TestExtractTransactions_ReturnsSliceWhenPresent(t *testing.T) {
	transactions := []testInjectTx[testReceipt]{{From: "alice", Value: 1}}
	block := &templateWithTxs{Txs: transactions}

	txs, ok, wasNil := ExtractTransactions[testInjectTx[testReceipt]](block)
	require.True(t, ok)
	require.False(t, wasNil)
	require.Equal(t, transactions, txs)
}

func TestExtractTransactions_HandlesNilSlice(t *testing.T) {
	block := &templateWithTxs{Txs: nil}

	txs, ok, wasNil := ExtractTransactions[testInjectTx[testReceipt]](block)
	require.True(t, ok)
	require.True(t, wasNil)
	require.Empty(t, txs)
}

func TestExtractTransactions_RejectedForIncompatibleTypes(t *testing.T) {
	block := &templateWithoutTxs{Number: "42"}

	txs, ok, wasNil := ExtractTransactions[testInjectTx[testReceipt]](block)
	require.False(t, ok)
	require.False(t, wasNil)
	require.Nil(t, txs)
}

func TestStructValueFrom(t *testing.T) {
	tests := map[string]struct {
		input          any
		expectOK       bool
		expectIsStruct bool
	}{
		"nil":                {input: nil, expectOK: false},
		"non-struct":         {input: 42, expectOK: false},
		"struct":             {input: templateWithoutTxs{}, expectOK: true, expectIsStruct: true},
		"pointer":            {input: &templateWithoutTxs{}, expectOK: true, expectIsStruct: true},
		"pointer to pointer": {input: new(*templateWithoutTxs), expectOK: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			value, ok := structValueFrom(tt.input)
			if !tt.expectOK {
				require.False(t, ok)

				return
			}

			require.True(t, ok)
			require.Equal(t, reflect.Struct, value.Kind())
		})
	}
}

func TestTxsField(t *testing.T) {
	value := reflect.ValueOf(templateWithTxs{})
	field, ok := txsField(value)
	require.True(t, ok)
	require.Equal(t, reflect.Slice, field.Kind())

	_, ok = txsField(reflect.ValueOf(templateWithoutTxs{}))
	require.False(t, ok)
}
