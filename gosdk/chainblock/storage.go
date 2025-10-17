package chainblock

import (
	"encoding/binary"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	// Bucket stores external chain blocks indexed by their chain type and block number.
	Bucket = "chainblocks"
)

// Key encodes a chain type and block number into an MDBX key.
// Layout: [4 bytes chainType | 8 bytes blockNumber] (big endian).
func Key(chainType apptypes.ChainType, number uint64) []byte {
	out := make([]byte, 12)
	binary.BigEndian.PutUint32(out[:4], uint32(chainType))
	binary.BigEndian.PutUint64(out[4:], number)

	return out
}

// prefix extracts the key prefix used for a specific chain type (first 4 bytes).
func prefix(chainType apptypes.ChainType) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, uint32(chainType))

	return out
}
