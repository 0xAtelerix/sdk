package appblock

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
)

type testChainBlock struct {
	Num    uint64 `json:"number"`
	Miner  string `json:"miner"`
	Secret string `json:"-"`
	Note   string `json:"note,omitempty"`
}

func (b *testChainBlock) Number() uint64 {
	if b == nil {
		return 0
	}

	return b.Num
}

func (*testChainBlock) Hash() [32]byte      { return [32]byte{} }
func (*testChainBlock) StateRoot() [32]byte { return [32]byte{} }
func (*testChainBlock) Bytes() []byte       { return nil }

type stringBlock string

func (*stringBlock) Number() uint64      { return 0 }
func (*stringBlock) Hash() [32]byte      { return [32]byte{} }
func (*stringBlock) StateRoot() [32]byte { return [32]byte{} }
func (*stringBlock) Bytes() []byte       { return nil }

const testMemoNote = "memo"

type valueChainBlock struct {
	Num    uint64 `json:"number"`
	Miner  string `json:"miner"`
	Secret string `json:"-"`
	Note   string `json:"note,omitempty"`
}

func (b valueChainBlock) Number() uint64    { return b.Num }
func (valueChainBlock) Hash() [32]byte      { return [32]byte{} }
func (valueChainBlock) StateRoot() [32]byte { return [32]byte{} }
func (valueChainBlock) Bytes() []byte       { return nil }

func TestNewAppBlock(t *testing.T) {
	note := "hello"
	target := &testChainBlock{
		Num:    7,
		Miner:  "alice",
		Secret: "hidden",
		Note:   note,
	}

	cb := NewAppBlock(uint64(7), target)
	require.Equal(t, uint64(7), cb.BlockNumber)
	require.Same(t, target, cb.Target)
}

func TestAppBlockToFields_StructPointer(t *testing.T) {
	note := testMemoNote
	target := &testChainBlock{
		Num:    42,
		Miner:  "bob",
		Secret: "shh",
		Note:   note,
	}

	cb := NewAppBlock(uint64(42), target)

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(t, []string{"number", "miner", "-", "note"}, fv.Fields)
	require.Equal(t, []string{"42", "bob", "shh", note}, fv.Values)
}

func TestAppBlockToFields_ValueStruct(t *testing.T) {
	note := testMemoNote
	target := valueChainBlock{
		Num:    5,
		Miner:  "carol",
		Secret: "hidden",
		Note:   note,
	}

	cb := NewAppBlock(uint64(5), target)

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(t, []string{"number", "miner", "-", "note"}, fv.Fields)
	require.Equal(t, []string{"5", "carol", "hidden", note}, fv.Values)
}

func TestAppBlockToFields_NilReceiver(t *testing.T) {
	var cb *AppBlock[*testChainBlock]

	fv, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, gosdk.ErrAppBlockValueNil)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestAppBlockToFields_NilTarget(t *testing.T) {
	var target *testChainBlock

	cb := NewAppBlock(uint64(1), target)
	fv, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, gosdk.ErrAppBlockTargetNilPointer)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestAppBlockToFields_NonStruct(t *testing.T) {
	var sb stringBlock = "payload"

	cb := NewAppBlock(uint64(1), &sb)
	_, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, gosdk.ErrAppBlockTargetNotStruct)
}

func TestBuildFieldMetadata_RespectsTags(t *testing.T) {
	type sample struct {
		ID      string `json:"id,omitempty"`
		Name    string
		private string
		Alias   string `json:"alias"`
		Skip    string `json:"-"`
	}

	meta := buildFieldMetadata(reflect.TypeOf(sample{}))
	require.Equal(t, []fieldMetadata{
		{index: 0, name: "id"},
		{index: 1, name: "Name"},
		{index: 3, name: "alias"},
		{index: 4, name: "-"},
	}, meta)
}

func TestFormatValue(t *testing.T) {
	require.Equal(t, "true", formatValue(reflect.ValueOf(true)))
}
