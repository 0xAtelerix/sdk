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

	fv := cb.ToFieldsAndValues()
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

	fv := cb.ToFieldsAndValues()

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

	fv := cb.ToFieldsAndValues()
	require.Equal(t, map[string]string{"id": "abc", "Height": "5"}, pairMap(fv))
}

func TestNewAppBlock_Success(t *testing.T) {
	payload := &struct{ Name string }{Name: "Alice"}

	cb := NewAppBlock(uint64(7), payload)
	require.NotNil(t, cb)
	require.Equal(t, uint64(7), cb.BlockNumber)
	require.Same(t, payload, cb.Target)
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
