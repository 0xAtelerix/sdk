package block

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/fxamacker/cbor/v2"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// ChainBlock augments an AppchainBlock with the ChainID it belongs to.
type ChainBlock struct {
	ChainID uint64
	Block   apptypes.AppchainBlock // or map[apptypes.ChainType]apptypes.AppchainBlock
}

// type ChainBlock struct {
// 	ChainID uint64
// 	Block   any
// }


// type ChainBlock struct {
// 	map[uint64]any  // chainID -> inner block
// }


/*
Now we have 2 types of inner blocks: EthereumBlock and SolanaBlock.

type EthereumBlock struct {
	Header gethtypes.Header
	Body   gethtypes.Body
}

type Block struct {
	Blockhash         string
	BlockTime         *time.Time
	BlockHeight       *int64
	PreviousBlockhash string
	ParentSlot        uint64
	Transactions      []BlockTransaction
	Signatures        []string
	Rewards           []Reward
}


*/


var _ apptypes.AppchainBlock = (*ChainBlock)(nil)

// Number returns the inner block number or zero if the block is nil.
func (cb *ChainBlock) Number() uint64 {
	if cb == nil || cb.Block == nil {
		return 0
	}

	return cb.Block.Number()
}

// Hash returns the inner block hash or the zero hash if the block is nil.
func (cb *ChainBlock) Hash() [32]byte {
	if cb == nil || cb.Block == nil {
		return [32]byte{}
	}

	var chainBuf [8]byte
	binary.BigEndian.PutUint64(chainBuf[:], cb.ChainID)

	chainHash := sha256.Sum256(chainBuf[:])
	blockHash := cb.Block.Hash()

	hasher := sha256.New()
	hasher.Write(chainHash[:])
	
	hasher.Write(blockHash[:])

	var out [32]byte
	copy(out[:], hasher.Sum(nil))

	return out
}

// StateRoot returns the inner block state root or zero if the block is nil.
func (cb *ChainBlock) StateRoot() [32]byte {
	if cb == nil || cb.Block == nil {
		return [32]byte{}
	}

	return cb.Block.StateRoot()
}

type chainBlockPayload struct {
	ChainID uint64 `cbor:"chainId"`
	Block   []byte `cbor:"block"`
}

// Bytes encodes the chain-id alongside the inner block bytes. If the block
// is nil it returns nil.
func (cb *ChainBlock) Bytes() []byte {
	if cb == nil || cb.Block == nil {
		return nil
	}

	payload := chainBlockPayload{
		ChainID: cb.ChainID,
		Block:   cb.Block.Bytes(),
	}

	data, err := cbor.Marshal(payload)
	if err != nil {
		return nil
	}

	return data
}
