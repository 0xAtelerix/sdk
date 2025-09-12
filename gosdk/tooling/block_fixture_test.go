package gosdk

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
)

// --- minimal test fixture writer matching your read-side keying ---

type testFixtureWriter struct{ db kv.RwDB }

func (w *testFixtureWriter) putEthBlock(t *testing.T, blk *gethtypes.Block) {
	t.Helper()

	num := blk.NumberU64()
	h := blk.Hash()

	key := make([]byte, 8+32)
	binary.BigEndian.PutUint64(key[:8], num)
	copy(key[8:], h[:])

	enc, err := rlp.EncodeToBytes(blk)
	require.NoError(t, err)

	require.NoError(t, w.db.Update(t.Context(), func(tx kv.RwTx) error {
		return tx.Put(gosdk.EthBlocks, key, enc)
	}))
}

func (w *testFixtureWriter) putEthReceipts(
	t *testing.T,
	blockNum uint64,
	blockHash common.Hash,
	recs []*gethtypes.Receipt,
) {
	t.Helper()
	err := w.db.Update(t.Context(), func(tx kv.RwTx) error {
		for i, r := range recs {
			// ensure fields are set in case caller forgot
			if r.BlockNumber == nil {
				r.BlockNumber = new(big.Int).SetUint64(blockNum)
			}

			r.BlockHash = blockHash
			r.TransactionIndex = uint(i)

			// encode
			// BEWARE: RLP encode gives same empty response for some reason.
			enc, err := json.Marshal(r)
			if err != nil {
				return err
			}

			key := ethReceiptKey(blockNum, blockHash, uint32(i))

			if err = tx.Put(receipt.ReceiptBucket, key, enc); err != nil {
				return err
			}
		}

		return nil
	})
	require.NoError(t, err)
}

func (w *testFixtureWriter) putSolBlock(t *testing.T, blk *client.Block) {
	t.Helper()

	key := make([]byte, 8)

	require.NotNil(t, blk.BlockHeight)
	binary.BigEndian.PutUint64(key, uint64(*blk.BlockHeight))

	enc, err := cbor.Marshal(blk)
	require.NoError(t, err)

	require.NoError(t, w.db.Update(t.Context(), func(tx kv.RwTx) error {
		return tx.Put(gosdk.SolanaBlocks, key, enc)
	}))
}

// --- helpers ---

func openMDBXAt(t *testing.T, path string, tables kv.TableCfg) kv.RwDB {
	t.Helper()

	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(path).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg { return tables }).
		Open()
	require.NoError(t, err)

	return db
}

// build a tiny synthetic geth block with stable hash for tests
func makeEthBlock(t *testing.T, num uint64) *gethtypes.Block {
	t.Helper()

	h := &gethtypes.Header{
		Number:   new(big.Int).SetUint64(num),
		Time:     uint64(time.Now().Unix()),
		GasLimit: 30_000_000,
	}
	// NewBlockWithHeader returns a *Block using only the header (no txs/uncles/withdrawals)
	blk := gethtypes.NewBlockWithHeader(h)

	return blk
}

func makeEthReceipts(t *testing.T, blk *gethtypes.Block, n int) []*gethtypes.Receipt {
	t.Helper()

	recs := make([]*gethtypes.Receipt, n)
	for i := range n {
		r := &gethtypes.Receipt{
			// minimally set fields:
			Status:            1,
			CumulativeGasUsed: 21000,
			GasUsed:           21000,
			BlockNumber:       new(big.Int).Set(blk.Number()),
			BlockHash:         blk.Hash(),
			TransactionIndex:  uint(i),
			Logs: []*gethtypes.Log{
				{
					Topics: []common.Hash{{}},
				},
			},
		}

		// TxHash: if your block has real txs:
		if txs := blk.Transactions(); i < txs.Len() {
			r.TxHash = txs[i].Hash()
		} else {
			// deterministic dummy if you didn’t populate transactions
			var h common.Hash

			h[0] = byte(i)
			r.TxHash = h
		}

		recs[i] = r
	}

	return recs
}

func rand32() [32]byte {
	var b [32]byte

	_, _ = rand.Read(b[:])

	return b
}

// --- tests ---

func TestEthereum_BlockAndReceipts_RoundTrip(t *testing.T) {
	t.Parallel()

	evmDBPath := t.TempDir()

	// create writer DB with EVM tables and write fixtures
	writerDB := openMDBXAt(t, evmDBPath, gosdk.EvmTables())

	fw := &testFixtureWriter{db: writerDB}

	const chainID = uint64(1)

	blockNum := uint64(100)

	blk := makeEthBlock(t, blockNum)
	fw.putEthBlock(t, blk)

	recs := makeEthReceipts(t, blk, 3)
	fw.putEthReceipts(t, blockNum, blk.Hash(), recs)

	writerDB.Close()

	// open read-only accessor and read back
	//nolint:exhaustive // it is a config
	mc, err := gosdk.NewMultichainStateAccess(gosdk.MultichainConfig{
		gosdk.ChainType(chainID): evmDBPath,
	})
	require.NoError(t, err)

	defer mc.Close()

	ctx := context.Background()

	outBlk, err := mc.EthBlock(ctx, apptypes.MakeExternalBlock(chainID, blockNum, blk.Hash()))
	require.NoError(t, err)
	require.Equal(t, blockNum, outBlk.NumberU64())
	require.Equal(t, blk.Hash(), outBlk.Hash())

	outRecs, err := mc.EthReceipts(ctx, apptypes.MakeExternalBlock(chainID, blockNum, blk.Hash()))

	require.NoError(t, err)
	require.Len(t, outRecs, len(recs))

	for _, r := range outRecs {
		require.Equal(t, blk.Hash(), r.BlockHash)
		require.Equal(t, blk.Number().Uint64(), r.BlockNumber.Uint64())
	}
}

func TestSolana_Block_RoundTrip(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	svmDBPath := filepath.Join(tmp, "svmdb")

	// create writer DB with Solana tables and write fixture
	writerDB := openMDBXAt(t, svmDBPath, gosdk.SolanaTables())

	fw := &testFixtureWriter{db: writerDB}

	const chainID = uint64(gosdk.SolanaDevnetChainID) // use one of your Solana ChainType values

	slot := int64(12345)

	wantHashBin := rand32()
	sol := &client.Block{
		BlockHeight: &slot,
		Blockhash:   base58.Encode(wantHashBin[:]),
		// other fields can stay zero for this test
	}
	fw.putSolBlock(t, sol)

	writerDB.Close()

	// open accessor and read back
	//nolint:exhaustive // it is a config
	mc, err := gosdk.NewMultichainStateAccess(gosdk.MultichainConfig{
		gosdk.ChainType(chainID): svmDBPath,
	})
	require.NoError(t, err)

	defer mc.Close()

	ctx := context.Background()

	out, err := mc.SolanaBlock(ctx, apptypes.MakeExternalBlock(chainID, uint64(slot), wantHashBin))
	require.NoError(t, err)
	require.NotNil(t, out.BlockHeight)
	require.Equal(t, slot, *out.BlockHeight)
	// verify base58 hash matches what we expected
	gotHashBin, _ := base58.Decode(out.Blockhash)
	require.True(t, bytes.Equal(wantHashBin[:], gotHashBin), "hash mismatch: want %s got %s",
		hex.EncodeToString(wantHashBin[:]), out.Blockhash)
}
