package cexvenue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func frozenIdentityJSON(t *testing.T) []byte {
	t.Helper()

	return readFixture(t, filepath.Join("..", "apptypes", "cex_order_book_identity.json"))
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	return raw
}

func u8(v uint8) *uint8 { return &v }

func u32(v uint32) *uint32 { return &v }
