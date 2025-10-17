package block

import (
	
	"github.com/fxamacker/cbor/v2"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/blocto/solana-go-sdk/client"

)

// ChainBlock augments an AppchainBlock with the ChainID it belongs to.
type ChainBlock struct {
	ChainType apptypes.ChainType
	Block   any
}

func NewChainBlock(chainType apptypes.ChainType, blockBytes []byte) *ChainBlock {
	cb :=  &ChainBlock{}
	switch chainType {
	case gosdk.EthereumChainID, gosdk.PolygonChainID, gosdk.BNBChainID, gosdk.GnosisChainID, gosdk.FantomChainID, gosdk.BaseChainID :
		var ethBlock gethtypes.Block
		if err := cbor.Unmarshal(blockBytes, &ethBlock); err != nil {
			panic(err)
		}
		cb = &ChainBlock{
			ChainType: chainType,
			Block:     &ethBlock,
		}
	case gosdk.SolanaChainID:
		var solBlock client.Block
		if err := cbor.Unmarshal(blockBytes, &solBlock); err != nil {
			panic(err)
		}
		cb = &ChainBlock{
			ChainType: chainType,
			Block:     &solBlock,
		}
	default:
		panic("unsupported chain type")

	}
	return cb
}
