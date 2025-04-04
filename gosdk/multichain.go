package gosdk

import (
	"github.com/0xAtelerix/sdk/gosdk/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/umbracle/ethgo"
)

type MultichainStateAccess struct {
	AppchainDB kv.RoDB
}

func (sa *MultichainStateAccess) EthBlock(block types.ExternalBlock) (ethgo.Block, error) {
	return ethgo.Block{}, nil
}

func (sa *MultichainStateAccess) EthReceipts(block types.ExternalBlock) ([]ethgo.Receipt, error) {
	return []ethgo.Receipt{}, nil
}

func (sa *MultichainStateAccess) EthReceipt(chainID uint32, hash [32]byte) (ethgo.Receipt, error) {
	return ethgo.Receipt{}, nil
}
