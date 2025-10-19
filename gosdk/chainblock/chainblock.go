package chainblock

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/fxamacker/cbor/v2"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

var ErrUnsupportedChainType = errors.New("unsupported chain type")

// ChainBlock augments an AppchainBlock with the ChainID it belongs to.
type ChainBlock struct {
	ChainType apptypes.ChainType
	Block     any
}

func NewChainBlock(chainType apptypes.ChainType, payload []byte) (*ChainBlock, error) {
	switch {
	case gosdk.IsEvmChain(chainType):
		ethBlock, err := decodeEthereumBlock(payload)
		if err != nil {
			return nil, fmt.Errorf("decode ethereum block: %w", err)
		}

		return &ChainBlock{ChainType: chainType, Block: ethBlock}, nil
	case gosdk.IsSolanaChain(chainType):
		solBlock, err := decodeSolanaBlock(payload)
		if err != nil {
			return nil, fmt.Errorf("decode solana block: %w", err)
		}

		return &ChainBlock{ChainType: chainType, Block: solBlock}, nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedChainType, chainType)
	}
}

func (cb *ChainBlock) convertToFieldsValues() FieldsValues {
	switch {
	case gosdk.IsEvmChain(cb.ChainType):
		ethBlock, ok := cb.Block.(*gethtypes.Block)
		if !ok {
			panic("invalid block type for EVM chain")
		}

		return convertEthBlockToFieldsValues(ethBlock)
	case gosdk.IsSolanaChain(cb.ChainType):
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

// convertEthBlockToFieldsValues converts Ethereum block to its fields and values
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

	if block == nil {
		return FieldsValues{Fields: fields, Values: nil}
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

// convertSolanaBlockToFieldsValues converts Solana block to its fields and values
func convertSolanaBlockToFieldsValues(block *client.Block) FieldsValues {
	fields := []string{
		"blockhash",
		"previousBlockhash",
		"parentSlot",
		"transactionsCount",
		"rewardsCount",
		"blockTime",
	}

	if block == nil {
		return FieldsValues{Fields: fields, Values: nil}
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

// decodeEthereumBlock unmarshal payload to Ethereum block
func decodeEthereumBlock(payload []byte) (*gethtypes.Block, error) {
	var stored gosdk.EthereumBlock
	if err := cbor.Unmarshal(payload, &stored); err == nil && stored.Header.Number != nil {
		return gethtypes.NewBlockWithHeader(&stored.Header).WithBody(stored.Body), nil
	}

	var ethBlock gethtypes.Block
	if err := cbor.Unmarshal(payload, &ethBlock); err != nil {
		return nil, err
	}

	return &ethBlock, nil
}

// decodeSolanaBlock unmarshal payload to Solana block
func decodeSolanaBlock(payload []byte) (*client.Block, error) {
	var solBlock client.Block
	if err := cbor.Unmarshal(payload, &solBlock); err != nil {
		return nil, err
	}

	return &solBlock, nil
}
