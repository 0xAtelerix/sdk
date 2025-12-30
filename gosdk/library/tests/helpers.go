package tests

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func MustType(t *testing.T, s string) abi.Type {
	t.Helper()

	tt, err := abi.NewType(s, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	return tt
}

func Addr(i byte) common.Address {
	var a common.Address

	a[19] = i

	return a
}

func AddrTopic(a common.Address) common.Hash {
	var h common.Hash
	copy(h[12:], a[:]) // ABI-encoded indexed address (left-padded to 32)

	return h
}

func MustPack(t *testing.T, args abi.Arguments, vs ...any) []byte {
	t.Helper()

	b, err := args.Pack(vs...)
	require.NoError(t, err)

	return b
}
