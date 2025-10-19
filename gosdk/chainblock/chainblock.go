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

var (
	// ErrUnsupportedChainType indicates that we do not have decode/format support
	// for the provided chain identifier.
	ErrUnsupportedChainType = errors.New("unsupported chain type")

	errMissingFormatter    = errors.New("missing chain block formatter")
	errNilChainBlock       = errors.New("chain block is nil")
	errUnexpectedBlockType = errors.New("unexpected block type")
)

type chainAdapter struct {
	supports func(apptypes.ChainType) bool
	decode   func([]byte) (any, error)
	format   func(any) (FieldsValues, error)
}

// ChainBlock augments a raw block payload with the originating chain type and a
// formatter that can render it into tabular field/value pairs for RPC clients.
type ChainBlock struct {
	ChainType apptypes.ChainType
	Block     any

	formatter func(any) (FieldsValues, error)
}

// NewChainBlock decodes the supplied payload into a chain-specific block and
// returns a wrapper that knows how to render it as FieldsValues.
func NewChainBlock(chainType apptypes.ChainType, payload []byte) (*ChainBlock, error) {
	adapter, err := resolveAdapter(chainType)
	if err != nil {
		return nil, err
	}

	block, err := adapter.decode(payload)
	if err != nil {
		return nil, fmt.Errorf("decode chain block: %w", err)
	}

	return newChainBlockFromBlock(chainType, block, adapter)
}

func newChainBlockFromBlock(
	chainType apptypes.ChainType,
	block any,
	adapter chainAdapter,
) (*ChainBlock, error) {
	if adapter.format == nil {
		return nil, fmt.Errorf("%w: %d", errMissingFormatter, chainType)
	}

	return &ChainBlock{
		ChainType: chainType,
		Block:     block,
		formatter: adapter.format,
	}, nil
}

func resolveAdapter(chainType apptypes.ChainType) (chainAdapter, error) {
	adapters := []chainAdapter{
		{
			supports: gosdk.IsEvmChain,
			decode: func(payload []byte) (any, error) {
				return decodeEthereumBlock(payload)
			},
			format: func(block any) (FieldsValues, error) {
				ethBlock, ok := block.(*gethtypes.Block)
				if !ok {
					return FieldsValues{}, fmt.Errorf(
						"%w: expected *types.Block got %T",
						errUnexpectedBlockType,
						block,
					)
				}

				return convertEthBlockToFieldsValues(ethBlock), nil
			},
		},
		{
			supports: gosdk.IsSolanaChain,
			decode: func(payload []byte) (any, error) {
				return decodeSolanaBlock(payload)
			},
			format: func(block any) (FieldsValues, error) {
				solBlock, ok := block.(*client.Block)
				if !ok {
					return FieldsValues{}, fmt.Errorf(
						"%w: expected *client.Block got %T",
						errUnexpectedBlockType,
						block,
					)
				}

				return convertSolanaBlockToFieldsValues(solBlock), nil
			},
		},
	}

	for _, adapter := range adapters {
		if adapter.supports(chainType) {
			return adapter, nil
		}
	}

	return chainAdapter{}, fmt.Errorf("%w: %d", ErrUnsupportedChainType, chainType)
}

func (cb *ChainBlock) convertToFieldsValues() (FieldsValues, error) {
	if cb == nil {
		return FieldsValues{}, errNilChainBlock
	}

	if cb.formatter == nil {
		return FieldsValues{}, fmt.Errorf("%w: %d", errMissingFormatter, cb.ChainType)
	}

	return cb.formatter(cb.Block)
}

// FieldsValues lists the rendered field names and the corresponding string
// values for a decoded chain block.
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

func decodeSolanaBlock(payload []byte) (*client.Block, error) {
	var solBlock client.Block
	if err := cbor.Unmarshal(payload, &solBlock); err != nil {
		return nil, err
	}

	return &solBlock, nil
}
