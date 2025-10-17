package chainblock

import (
	"context"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const testBucket = "chain-blocks"

func TestGetBlock_Ethereum(t *testing.T) {
	t.Parallel()

	db := createDB(t, testBucket)
	defer db.Close()

	header := &gethtypes.Header{
		Number:     big.NewInt(12),
		ParentHash: common.HexToHash("0x1"),
		Extra:      []byte{0xca, 0xfe},
		GasLimit:   30_000_000,
		GasUsed:    1_500_000,
		Time:       1_700_000_000,
	}
	block := gethtypes.NewBlock(header, nil, nil, nil)

	payload := encodeStoredChainBlock(t, gosdk.EthereumChainID, block)

	key := Key(gosdk.EthereumChainID, 12)

	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.Put(testBucket, key, payload)
	}))

	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
		fv, err := GetChainBlock(tx, testBucket, gosdk.EthereumChainID, key, nil)
		require.NoError(t, err)

		require.Equal(t, block.Number().String(), fv.Values[0])
		require.Equal(t, block.Hash().Hex(), fv.Values[1])
		require.Equal(t, strconv.FormatUint(block.GasLimit(), 10), fv.Values[14])

		return nil
	}))
}

func TestGetBlock_Solana(t *testing.T) {
	t.Parallel()

	db := createDB(t, testBucket)
	defer db.Close()

	blockTime := time.Unix(1_700_000_000, 0)
	sol := client.Block{
		Blockhash:         "hash",
		PreviousBlockhash: "prev",
		ParentSlot:        77,
		Transactions:      make([]client.BlockTransaction, 2),
		Rewards:           make([]client.Reward, 1),
		BlockTime:         &blockTime,
	}
	payload := encodeStoredChainBlock(t, gosdk.SolanaChainID, sol)

	key := Key(gosdk.SolanaChainID, 55)

	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.Put(testBucket, key, payload)
	}))

	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
		fv, err := GetChainBlock(tx, testBucket, gosdk.SolanaChainID, key, nil)
		require.NoError(t, err)

		require.Equal(t, sol.Blockhash, fv.Values[0])
		require.Equal(t, strconv.Itoa(len(sol.Transactions)), fv.Values[3])
		require.Equal(t, strconv.FormatInt(blockTime.Unix(), 10), fv.Values[5])

		return nil
	}))
}

func TestGetBlock_NoValue(t *testing.T) {
	t.Parallel()

	db := createDB(t, testBucket)
	defer db.Close()

	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
		_, err := GetChainBlock(
			tx,
			testBucket,
			gosdk.EthereumChainID,
			Key(gosdk.EthereumChainID, 1),
			nil,
		)
		require.ErrorIs(t, err, ErrNoBlocks)

		return nil
	}))
}

func TestGetBlocks_ReturnsNewestFirst(t *testing.T) {
	t.Parallel()

	db := createDB(t, testBucket)
	defer db.Close()

	blocks := []*gethtypes.Block{
		gethtypes.NewBlock(&gethtypes.Header{Number: big.NewInt(2)}, nil, nil, nil),
		gethtypes.NewBlock(&gethtypes.Header{Number: big.NewInt(5)}, nil, nil, nil),
		gethtypes.NewBlock(&gethtypes.Header{Number: big.NewInt(9)}, nil, nil, nil),
	}

	require.NoError(t, db.Update(context.Background(), func(tx kv.RwTx) error {
		for _, blk := range blocks {
			key := Key(gosdk.EthereumChainID, blk.NumberU64())

			val := encodeStoredChainBlock(t, gosdk.EthereumChainID, blk)
			if err := tx.Put(testBucket, key, val); err != nil {
				return err
			}
		}

		return nil
	}))

	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
		fv, err := GetChainBlocks(tx, testBucket, gosdk.EthereumChainID, 3)
		require.NoError(t, err)
		require.Len(t, fv, 3)

		require.Equal(t, "9", fv[0].Values[0])
		require.Equal(t, "5", fv[1].Values[0])
		require.Equal(t, "2", fv[2].Values[0])

		return nil
	}))
}

func TestGetBlocks_EmptyBucket(t *testing.T) {
	t.Parallel()

	db := createDB(t, testBucket)
	defer db.Close()

	require.NoError(t, db.View(context.Background(), func(tx kv.Tx) error {
		_, err := GetChainBlocks(tx, testBucket, gosdk.EthereumChainID, 1)
		require.ErrorIs(t, err, ErrNoBlocks)

		return nil
	}))
}

func encodeStoredChainBlock(tb testing.TB, chainType apptypes.ChainType, block any) []byte {
	tb.Helper()

	var (
		blockBytes []byte
		err        error
	)

	switch {
	case gosdk.IsEvmChain(chainType):
		ethBlock, ok := block.(*gethtypes.Block)
		require.True(tb, ok, "expected geth block for EVM chain")

		blockBytes, err = cbor.Marshal(gosdk.NewEthereumBlock(ethBlock))
	case gosdk.IsSolanaChain(chainType):
		blockBytes, err = cbor.Marshal(block)
	default:
		tb.Fatalf("unsupported chain type %d", chainType)
	}

	require.NoError(tb, err)

	raw, err := cbor.Marshal(storedChainBlock{
		ChainType: chainType,
		Block:     blockBytes,
	})
	require.NoError(tb, err)

	return raw
}

func createDB(tb testing.TB, buckets ...string) kv.RwDB {
	tb.Helper()

	db := memdb.NewTestDB(tb)
	require.NoError(tb, db.Update(context.Background(), func(tx kv.RwTx) error {
		for _, bucket := range buckets {
			if err := tx.CreateBucket(bucket); err != nil {
				return err
			}
		}

		return nil
	}))

	return db
}
