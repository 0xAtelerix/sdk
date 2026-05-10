package apptypes

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func TestCEXPriceLevelCBORUsesCompactArray(t *testing.T) {
	t.Parallel()

	encMode, err := cbor.CanonicalEncOptions().EncMode()
	require.NoError(t, err)

	payload, err := encMode.Marshal(CEXPriceLevel{
		Price:    "0.40987654",
		Quantity: "1234.56789012",
	})
	require.NoError(t, err)

	var asArray []string
	require.NoError(t, cbor.Unmarshal(payload, &asArray))
	require.Equal(t, []string{"0.40987654", "1234.56789012"}, asArray)

	var asMap map[uint64]string
	require.Error(t, cbor.Unmarshal(payload, &asMap))
}
