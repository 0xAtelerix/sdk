package block

import (
	"fmt"
	"strconv"
	
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


func (cb *ChainBlock) convertToFieldsValues() any {
	switch cb.ChainType {
	case gosdk.EthereumChainID, gosdk.PolygonChainID, gosdk.BNBChainID, gosdk.GnosisChainID, gosdk.FantomChainID, gosdk.BaseChainID :
		ethBlock, ok := cb.Block.(*gethtypes.Block)
		if !ok {
			panic("invalid block type for EVM chain")
		}
		return convertEthBlockToFieldsValues(ethBlock)
	case gosdk.SolanaChainID:
		solBlock, ok := cb.Block.(*client.Block)
		if !ok {
			panic("invalid block type for Solana chain")
		}
		return convertSolanaBlockToFieldsValues(solBlock)
	default:
		panic("unsupported chain type")
	}
}

func convertEthBlockToFieldsValues(block *gethtypes.Block) FieldsValues {
	fields := []string{
		"number",
		"hash",
		"parentHash",
		"nonce",
		"sha3Uncles",
		"logsBloom",
		"transactionsRoot",
		"stateRoot",
		"receiptsRoot",
		"miner",
		"difficulty",
		"totalDifficulty",
		"extraData",
		"size",
		"gasLimit",
		"gasUsed",
		"timestamp",
	}

	values := []string{
		block.Number().String(),
		block.Hash().Hex(),
		block.ParentHash().Hex(),
		block.Nonce().String(),
		block.UncleHash().Hex(),
		block.Bloom().String(),
		block.TxHash().Hex(),
		block.Root().Hex(),
		block.ReceiptHash().Hex(),
		block.Coinbase().Hex(),
		block.Difficulty().String(),
		block.Difficulty().String(), // TotalDifficulty is not directly available
		fmt.Sprintf("0x%x", block.Extra()),
		strconv.FormatUint(block.Size().Uint64(), 10),
		strconv.FormatUint(block.GasLimit(), 10),
		strconv.FormatUint(block.GasUsed(), 10),
		strconv.FormatUint(block.Time().Uint64(), 10),
	}

	return FieldsValues{Fields: fields, Values: values}
}

func convertSolanaBlockToFieldsValues(block *client.Block) FieldsValues {
	fields := []string{
		"blockhash",
		"previousBlockhash",
		"parentSlot",
		"transactionsCount",
		"rewardsCount",
		"blockTime",
	}

	values := []string{
		block.Blockhash,
		block.PreviousBlockhash,
		strconv.FormatUint(block.ParentSlot, 10),
		strconv.Itoa(len(block.Transactions)),
		strconv.Itoa(len(block.Rewards)),
		strconv.FormatInt(block.BlockTime, 10),
	}

	return FieldsValues{Fields: fields, Values: values}
}