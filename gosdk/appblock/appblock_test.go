package appblock

import (
	"math/big"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestAppBlockToFields_EthereumBlock(t *testing.T) {
	header := &gethtypes.Header{Number: big.NewInt(42)}
	block := gethtypes.NewBlock(header, nil, nil, nil)
	expectedTime := time.Unix(170, 0).UTC()
	block.ReceivedAt = expectedTime
	block.ReceivedFrom = "peer"

	cb := &AppBlock[*gethtypes.Block]{BlockNumber: 42, Target: block}

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(
		t,
		map[string]string{
			"ReceivedAt":   expectedTime.String(),
			"ReceivedFrom": "peer",
		},
		pairMap(fv),
	)
}

func TestAppBlockToFields_SolanaBlock(t *testing.T) {
	sol := &client.Block{Blockhash: "hash", PreviousBlockhash: "prev"}

	cb := &AppBlock[*client.Block]{BlockNumber: 99, Target: sol}

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(
		t,
		map[string]string{
			"Blockhash":         "hash",
			"BlockTime":         "",
			"BlockHeight":       "",
			"PreviousBlockhash": "prev",
			"ParentSlot":        "0",
			"Transactions":      "[]",
			"Signatures":        "[]",
			"Rewards":           "[]",
		},
		pairMap(fv),
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

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"id": "abc", "Height": "5"}, pairMap(fv))
}

func TestAppBlockToFields_NilTarget(t *testing.T) {
	cb := &AppBlock[*struct{}]{BlockNumber: 1, Target: nil}

	fv, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, errTargetNilPointer)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestAppBlockToFields_NilReceiver(t *testing.T) {
	var cb *AppBlock[*struct{}]

	fv, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, errAppBlockValueNil)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestAppBlockToFields_NilInterfaceTarget(t *testing.T) {
	var payload any

	cb := &AppBlock[any]{BlockNumber: 1, Target: payload}

	fv, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, errAppBlockValueNil)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestAppBlockToFields_NonStructPointer(t *testing.T) {
	value := "hello"
	cb := &AppBlock[*string]{BlockNumber: 1, Target: &value}

	fv, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, ErrTargetNotStruct)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}

func TestAppBlockToFields_NonStructValue(t *testing.T) {
	cb := &AppBlock[int]{BlockNumber: 1, Target: 99}

	fv, err := cb.ToFieldsAndValues()
	require.ErrorIs(t, err, ErrTargetNotStruct)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
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

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"Name": name, "Count": "7"}, pairMap(fv))
}

func TestAppBlockToFields_ValueStruct(t *testing.T) {
	type value struct {
		Height int
		Label  string
	}

	payload := value{Height: 3, Label: "node"}
	cb := &AppBlock[value]{BlockNumber: 2, Target: payload}

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(t, map[string]string{"Height": "3", "Label": "node"}, pairMap(fv))
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

	fv, err := cb.ToFieldsAndValues()
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"Data": "{11}",
		"List": "[alpha beta]",
	}, pairMap(fv))
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
