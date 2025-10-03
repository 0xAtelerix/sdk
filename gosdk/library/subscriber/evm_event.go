package subscriber

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
)

// returns a prefix if no abi given
func EVMEventKey(
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	eventName string,
) []byte {
	var key [64]byte // 4+32+32[:28]
	binary.BigEndian.PutUint32(key[:4], uint32(chainID))

	nameHash := sha256.Sum256([]byte(eventName))

	copy(key[4:36], contract[:])
	copy(key[36:], nameHash[:28])

	return key[:]
}
