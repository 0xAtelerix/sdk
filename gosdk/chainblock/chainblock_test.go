package chainblock

import (
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestConvertEthBlockToFieldsValues(t *testing.T) {
	header := &gethtypes.Header{
		Number:      big.NewInt(42),
		ParentHash:  common.HexToHash("0x01"),
		Nonce:       gethtypes.BlockNonce{0xaa},
		UncleHash:   gethtypes.EmptyUncleHash,
		Bloom:       gethtypes.Bloom{0x01},
		TxHash:      common.Hash{},
		Root:        common.Hash{},
		ReceiptHash: common.Hash{},
		Coinbase:    common.HexToAddress("0x000000000000000000000000000000000000c0de"),
		Difficulty:  big.NewInt(123456),
		Extra:       []byte{0xde, 0xad},
		GasLimit:    1000000,
		GasUsed:     500000,
		Time:        1700000000,
	}
	block := gethtypes.NewBlock(header, nil, nil, nil)

	fv := convertEthBlockToFieldsValues(block)
	require.Equal(t, "42", fv.Values[0])
	require.Equal(t, block.Hash().Hex(), fv.Values[1])
	require.Equal(t, fmt.Sprintf("0x%x", block.Nonce()), fv.Values[3])
	require.Equal(t, strconv.FormatUint(block.GasLimit(), 10), fv.Values[14])
}

func TestConvertSolanaBlockToFieldsValues(t *testing.T) {
	blockTime := time.Unix(1700000000, 0)
	sol := &client.Block{
		Blockhash:         "hash",
		PreviousBlockhash: "prev",
		ParentSlot:        77,
		Transactions:      make([]client.BlockTransaction, 3),
		Rewards:           make([]client.Reward, 2),
		BlockTime:         &blockTime,
	}

	fv := convertSolanaBlockToFieldsValues(sol)
	require.Equal(t, "hash", fv.Values[0])
	require.Equal(t, "3", fv.Values[3])
	require.Equal(t, strconv.FormatInt(blockTime.Unix(), 10), fv.Values[5])
}

func TestChainBlockConvertToFieldsValues_Eth(t *testing.T) {
	header := &gethtypes.Header{
		Number: big.NewInt(1),
	}
	block := gethtypes.NewBlock(header, nil, nil, nil)
	adapter, err := resolveAdapter(gosdk.EthereumChainID)
	require.NoError(t, err)

	cb, err := newChainBlockFromBlock(gosdk.EthereumChainID, block, adapter)
	require.NoError(t, err)

	fv, err := cb.convertToFieldsValues()
	require.NoError(t, err)
	require.Equal(t, block.Number().String(), fv.Values[0])
}

func TestChainBlockConvertToFieldsValues_Solana(t *testing.T) {
	sol := &client.Block{
		Blockhash: "hash",
	}
	adapter, err := resolveAdapter(gosdk.SolanaChainID)
	require.NoError(t, err)

	cb, err := newChainBlockFromBlock(gosdk.SolanaChainID, sol, adapter)
	require.NoError(t, err)

	fv, err := cb.convertToFieldsValues()
	require.NoError(t, err)
	require.Equal(t, "hash", fv.Values[0])
}

func TestNewChainBlock_Ethereum(t *testing.T) {
	header := &gethtypes.Header{
		Number: big.NewInt(100),
	}
	blk := gethtypes.NewBlock(header, nil, nil, nil)
	encoded, err := cbor.Marshal(blk)
	require.NoError(t, err)

	cb, err := NewChainBlock(gosdk.EthereumChainID, encoded)
	require.NoError(t, err)

	require.Equal(t, gosdk.EthereumChainID, cb.ChainType)
	_, ok := cb.Block.(*gethtypes.Block)
	require.True(t, ok)
}

func TestNewChainBlock_Solana(t *testing.T) {
	blockHeight := int64(55)
	blockTime := time.Unix(1700000000, 0)
	sol := client.Block{
		Blockhash:         "sol-hash",
		BlockHeight:       &blockHeight,
		BlockTime:         &blockTime,
		PreviousBlockhash: "prev",
	}
	encoded, err := cbor.Marshal(sol)
	require.NoError(t, err)

	cb, err := NewChainBlock(gosdk.SolanaChainID, encoded)
	require.NoError(t, err)

	require.Equal(t, gosdk.SolanaChainID, cb.ChainType)
	solBlock, ok := cb.Block.(*client.Block)
	require.True(t, ok)
	require.Equal(t, sol.Blockhash, solBlock.Blockhash)
	require.NotNil(t, solBlock.BlockTime)
	require.Equal(t, blockTime.Unix(), solBlock.BlockTime.Unix())
}

func TestNewChainBlock_UnsupportedChainPanics(t *testing.T) {
	cb, err := NewChainBlock(apptypes.ChainType(999999), []byte{})
	require.Error(t, err)
	require.Nil(t, cb)
}

func TestNewChainBlock_EthereumDecodeError(t *testing.T) {
	cb, err := NewChainBlock(gosdk.EthereumChainID, []byte("not-cbor"))
	require.Error(t, err)
	require.Nil(t, cb)
}

func TestNewChainBlockFromBlock_MissingFormatter(t *testing.T) {
	_, err := newChainBlockFromBlock(gosdk.EthereumChainID, nil, chainAdapter{})
	require.ErrorIs(t, err, errMissingFormatter)
}

func TestChainBlockConvertToFieldsValues_WrongType(t *testing.T) {
	adapter, err := resolveAdapter(gosdk.EthereumChainID)
	require.NoError(t, err)

	cb := &ChainBlock{
		ChainType: gosdk.EthereumChainID,
		Block:     &client.Block{},
		formatter: adapter.format,
	}

	_, err = cb.convertToFieldsValues()
	require.ErrorIs(t, err, errUnexpectedBlockType)
}

func TestChainBlockConvertToFieldsValues_NilReceiver(t *testing.T) {
	var cb *ChainBlock

	_, err := cb.convertToFieldsValues()
	require.ErrorIs(t, err, errNilChainBlock)
}

func TestResolveAdapter_UnsupportedChain(t *testing.T) {
	_, err := resolveAdapter(apptypes.ChainType(999))
	require.ErrorIs(t, err, ErrUnsupportedChainType)
}

func TestFormatBlockTimeNil(t *testing.T) {
	require.Equal(t, "0", formatBlockTime(nil))
}

func TestDecodeSolanaBlock(t *testing.T) {
	original := &client.Block{
		Blockhash:         "test-hash",
		PreviousBlockhash: "previous-hash",
		ParentSlot:        123,
		Transactions:      make([]client.BlockTransaction, 2),
		Rewards:           make([]client.Reward, 1),
	}

	payload, err := cbor.Marshal(original)
	require.NoError(t, err)

	block, err := decodeSolanaBlock(payload)
	require.NoError(t, err)
	require.Equal(t, original.Blockhash, block.Blockhash)
	require.Equal(t, original.PreviousBlockhash, block.PreviousBlockhash)
	require.Equal(t, original.ParentSlot, block.ParentSlot)
	require.Len(t, block.Transactions, len(original.Transactions))
	require.Len(t, block.Rewards, len(original.Rewards))
}

func TestDecodeSolanaBlock_InvalidPayload(t *testing.T) {
	_, err := decodeSolanaBlock([]byte("not cbor"))
	require.Error(t, err)
}
