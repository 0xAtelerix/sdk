package block

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func TestChainBlockDelegatesToInnerBlock(t *testing.T) {
	t.Parallel()

	inner := sampleBlock()
	cb := ChainBlock{
		ChainID: 99,
		Block:   inner,
	}

	require.Equal(t, inner.Number(), cb.Number())
	require.Equal(t, expectedChainBlockHash(cb.ChainID, inner.Hash()), cb.Hash())
	require.Equal(t, inner.StateRoot(), cb.StateRoot())
}

func TestChainBlockHandlesNil(t *testing.T) {
	t.Parallel()

	var cb *ChainBlock
	require.Zero(t, cb.Number())
	require.Equal(t, [32]byte{}, cb.Hash())
	require.Equal(t, [32]byte{}, cb.StateRoot())
	require.Nil(t, cb.Bytes())

	cb = &ChainBlock{ChainID: 10}
	require.Zero(t, cb.Number())
	require.Equal(t, [32]byte{}, cb.Hash())
	require.Equal(t, [32]byte{}, cb.StateRoot())
	require.Nil(t, cb.Bytes())
}

func TestChainBlockBytes(t *testing.T) {
	t.Parallel()

	inner := sampleBlock()
	cb := ChainBlock{
		ChainID: 42,
		Block:   inner,
	}

	data := cb.Bytes()
	require.NotNil(t, data)

	var payload struct {
		ChainID uint64 `cbor:"chainId"`
		Block   []byte `cbor:"block"`
	}
	require.NoError(t, cbor.Unmarshal(data, &payload))
	require.Equal(t, uint64(42), payload.ChainID)
	require.Equal(t, inner.Bytes(), payload.Block)
}

func expectedChainBlockHash(chainID uint64, innerHash [32]byte) [32]byte {
	var chainBuf [8]byte
	binary.BigEndian.PutUint64(chainBuf[:], chainID)

	chainHash := sha256.Sum256(chainBuf[:])

	hasher := sha256.New()
	_, _ = hasher.Write(chainHash[:])
	_, _ = hasher.Write(innerHash[:])

	var out [32]byte
	copy(out[:], hasher.Sum(nil))

	return out
}
