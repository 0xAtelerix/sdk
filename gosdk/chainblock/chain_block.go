package chainblock

import (
	"fmt"
	"strconv"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/fxamacker/cbor/v2"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// ChainBlock augments an AppchainBlock with the ChainID it belongs to.
type ChainBlock struct {
	ChainType apptypes.ChainType
	Block     any
}

func NewChainBlock(chainType apptypes.ChainType, blockBytes []byte) *ChainBlock {
	switch chainType {
	case gosdk.EthereumChainID,
		gosdk.PolygonChainID,
		gosdk.BNBChainID,
		gosdk.GnosisChainID,
		gosdk.FantomChainID,
		gosdk.BaseChainID:
		var ethBlock gethtypes.Block
		if err := cbor.Unmarshal(blockBytes, &ethBlock); err != nil {
			panic(err)
		}

		return &ChainBlock{ChainType: chainType, Block: &ethBlock}
	case gosdk.SolanaChainID:
		var solBlock client.Block
		if err := cbor.Unmarshal(blockBytes, &solBlock); err != nil {
			panic(err)
		}

		return &ChainBlock{ChainType: chainType, Block: &solBlock}
	default:
		panic("unsupported chain type")
	}
}

func (cb *ChainBlock) convertToFieldsValues() FieldsValues {
	switch cb.ChainType {
	case gosdk.EthereumChainID,
		gosdk.PolygonChainID,
		gosdk.BNBChainID,
		gosdk.GnosisChainID,
		gosdk.FantomChainID,
		gosdk.BaseChainID:
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

type FieldsValues struct {
	Fields []string
	Values []string
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
		fmt.Sprintf("0x%x", block.Nonce()),
		block.UncleHash().Hex(),
		fmt.Sprintf("0x%x", block.Bloom().Bytes()),
		block.TxHash().Hex(),
		block.Root().Hex(),
		block.ReceiptHash().Hex(),
		block.Coinbase().Hex(),
		block.Difficulty().String(),
		block.Difficulty().String(), // approximate total difficulty
		fmt.Sprintf("0x%x", block.Extra()),
		strconv.FormatUint(block.Size(), 10),
		strconv.FormatUint(block.GasLimit(), 10),
		strconv.FormatUint(block.GasUsed(), 10),
		strconv.FormatUint(block.Time(), 10),
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
		formatBlockTime(block.BlockTime),
	}

	return FieldsValues{Fields: fields, Values: values}
}

func formatBlockTime(t *time.Time) string {
	if t == nil {
		return "0"
	}

	return strconv.FormatInt(t.Unix(), 10)
}
