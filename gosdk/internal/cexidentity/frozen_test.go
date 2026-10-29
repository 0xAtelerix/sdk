package cexidentity

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const frozenTablePath = "../../apptypes/cex_order_book_identity.json"

func TestValidateFrozenAcceptsTheV9Table(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(frozenTablePath)
	require.NoError(t, err)
	require.NoError(t, ValidateFrozen(contents))
}

// TestValidateFrozenRejectsAnyEdit: changing one byte, appending or removing
// anything fails the check.
func TestValidateFrozenRejectsAnyEdit(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(frozenTablePath)
	require.NoError(t, err)

	for _, edited := range [][]byte{
		append([]byte{}, contents[:len(contents)-1]...),
		append(append([]byte{}, contents...), '\n'),
		func() []byte {
			changed := append([]byte{}, contents...)
			changed[len(changed)/2] ^= 1

			return changed
		}(),
		nil,
	} {
		err := ValidateFrozen(edited)
		require.ErrorIs(t, err, errFrozenTableChanged)
		require.ErrorContains(t, err, "SymbolID")
	}
}
