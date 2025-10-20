package chainblock

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
)

func TestGetFieldsValues_Success(t *testing.T) {
	payload := &struct {
		ID   string `json:"id"`
		Note string
	}{
		ID:   "123",
		Note: "ok",
	}

	fv, err := GetFieldsValues(gosdk.EthereumChainID, payload)
	require.NoError(t, err)
	require.Equal(t, []string{"id", "Note"}, fv.Fields)
	require.Equal(t, []string{"123", "ok"}, fv.Values)
}

func TestGetFieldsValues_NilTarget(t *testing.T) {
	fv, err := GetFieldsValues[string](gosdk.EthereumChainID, nil)
	require.Error(t, err)
	require.Empty(t, fv.Fields)
	require.Empty(t, fv.Values)
}
