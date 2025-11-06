package gosdk

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
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

	enc, err := json.Marshal(blk)
	require.NoError(t, err)

	require.NoError(t, w.db.Update(t.Context(), func(tx kv.RwTx) error {
		return tx.Put(gosdk.EthBlocks, key, enc)
	}))
}

func (w *testFixtureWriter) putEthereumBlock(t *testing.T, blk *gosdk.EthereumBlock) {
	t.Helper()

	num := blk.Header.Number.Uint64()
	h := blk.Header.Hash()

	key := make([]byte, 8+32)
	binary.BigEndian.PutUint64(key[:8], num)
	copy(key[8:], h[:])

	enc, err := json.Marshal(blk)
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

			key := gosdk.EthReceiptKey(blockNum, blockHash, uint32(i))

			if err = tx.Put(gosdk.EthReceipts, key, enc); err != nil {
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

	enc, err := json.Marshal(blk)
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

func makeEthBlock(t *testing.T, num uint64) *gosdk.EthereumBlock {
	t.Helper()

	// canonical empty trie root used by Ethereum
	emptyStateRoot := common.HexToHash(
		"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
	)

	// Derive canonical roots for “no txs / no receipts” blocks.
	emptyTxRoot := gethtypes.DeriveSha(gethtypes.Transactions{}, trie.NewStackTrie(nil))
	emptyReceiptsRoot := gethtypes.DeriveSha(gethtypes.Receipts{}, trie.NewStackTrie(nil))
	uncleHash := gethtypes.CalcUncleHash(nil) // same as EmptyUncleHash on recent geth

	h := gethtypes.Header{
		ParentHash:  common.BytesToHash([]byte(fmt.Sprintf("parent-%d", num-1))),
		UncleHash:   uncleHash,
		Coinbase:    common.Address{}, // 0x000... miner
		Root:        emptyStateRoot,
		TxHash:      emptyTxRoot,
		ReceiptHash: emptyReceiptsRoot,
		Bloom:       gethtypes.Bloom{},
		Difficulty:  big.NewInt(1),               // arbitrary non-zero
		Number:      new(big.Int).SetUint64(num), // required
		GasLimit:    30_000_000,
		GasUsed:     0,
		Time:        uint64(time.Now().Unix()),
		Extra:       []byte("test"),
		// Post-London blocks typically have BaseFee; keep it non-nil to mimic real chain
		BaseFee: big.NewInt(7),
		// Optional post-Shanghai/Cancun fields may stay nil:
		// WithdrawalsHash, BlobGasUsed, ExcessBlobGas, ParentBeaconRoot, RequestsHash
	}

	return &gosdk.EthereumBlock{
		Header: h,
	}
}

func makeEthReceipts(
	t *testing.T,
	blockNumber *big.Int,
	blockHash common.Hash,
	ethTransactions gethtypes.Transactions,
	n int,
) []*gethtypes.Receipt {
	t.Helper()

	recs := make([]*gethtypes.Receipt, n)
	for i := range n {
		r := &gethtypes.Receipt{
			// minimally set fields:
			Status:            1,
			CumulativeGasUsed: 21000,
			GasUsed:           21000,
			BlockNumber:       new(big.Int).Set(blockNumber),
			BlockHash:         blockHash,
			TransactionIndex:  uint(i),
			Logs: []*gethtypes.Log{
				{
					Topics: []common.Hash{{}},
				},
			},
		}

		// TxHash: if your block has real txs:
		if txs := ethTransactions; i < txs.Len() {
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
	fw.putEthereumBlock(t, blk)

	recs := makeEthReceipts(t, blk.Header.Number, blk.Header.Hash(), blk.Body.Transactions, 3)
	fw.putEthReceipts(t, blockNum, blk.Header.Hash(), recs)

	writerDB.Close()

	// open read-only accessor and read back
	chainDBs, err := gosdk.NewMultichainStateAccessDB(gosdk.MultichainConfig{
		apptypes.ChainType(chainID): evmDBPath,
	})
	require.NoError(t, err)

	mc := gosdk.NewMultichainStateAccess(chainDBs)
	defer mc.Close()

	ctx := context.Background()

	outBlk, err := mc.EthBlock(
		ctx,
		apptypes.MakeExternalBlock(chainID, blockNum, blk.Header.Hash()),
	)
	require.NoError(t, err)
	require.Equal(t, blockNum, outBlk.Header.Number.Uint64())
	require.Equal(t, blk.Header.Hash(), outBlk.Header.Hash())

	outRecs, err := mc.EthReceipts(
		ctx,
		apptypes.MakeExternalBlock(chainID, blockNum, blk.Header.Hash()),
	)

	require.NoError(t, err)
	require.Len(t, outRecs, len(recs))

	for _, r := range outRecs {
		require.Equal(t, blk.Header.Hash(), r.BlockHash)
		require.Equal(t, blk.Header.Number.Uint64(), r.BlockNumber.Uint64())
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
	chainDBs, err := gosdk.NewMultichainStateAccessDB(gosdk.MultichainConfig{
		apptypes.ChainType(chainID): svmDBPath,
	})
	require.NoError(t, err)

	mc := gosdk.NewMultichainStateAccess(chainDBs)

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

// Iterators

func TestEthBlockFileIterator_Pipeline(t *testing.T) {
	t.Parallel()

	// --- Arrange: create file with raw json blocks (one per line)
	tmp := t.TempDir()
	path := filepath.Join(tmp, "eth_blocks.json")

	start, end := uint64(100), uint64(102)

	type rec struct {
		num  uint64
		hash common.Hash
	}

	var want []rec

	f, err := os.Create(path)
	require.NoError(t, err)

	for n := start; n <= end; n++ {
		blk := makeEthBlock(t, n)

		var b []byte

		b, err = json.Marshal(blk)
		require.NoError(t, err)

		var ethBlock gosdk.EthereumBlock

		err = json.Unmarshal(b, &ethBlock)
		require.NoError(t, err)

		_, err = f.Write(append(b, '\n'))
		require.NoError(t, err)

		want = append(want, rec{num: n, hash: blk.Header.Hash()})
	}

	require.NoError(t, f.Close())

	// --- Arrange: open DB writer
	evmDBPath := filepath.Join(tmp, "evmdb")
	writerDB := openMDBXAt(t, evmDBPath, gosdk.EvmTables())

	// --- Act: iterate file -> write via FixtureWriter.writeOne
	it, err := NewEthBlockFileIterator(path)
	require.NoError(t, err)

	fw := &FixtureWriter[*gosdk.EthereumBlock]{DB: writerDB}

	ctx := t.Context()

	for {
		var blk *gosdk.EthereumBlock

		blk, err = it.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)
		require.NoError(t, fw.writeOne(ctx, blk), blk)
	}

	err = it.Close()
	require.NoError(t, err)

	writerDB.Close()

	// --- Assert: read back using MultichainStateAccess

	chainDBs, err := gosdk.NewMultichainStateAccessDB(gosdk.MultichainConfig{
		apptypes.ChainType(1): evmDBPath,
	})
	require.NoError(t, err)

	mc := gosdk.NewMultichainStateAccess(chainDBs)
	defer mc.Close()

	for _, w := range want {
		out, err := mc.EthBlock(ctx, apptypes.MakeExternalBlock(1, w.num, w.hash))
		require.NoError(t, err)
		require.Equal(t, w.num, out.Header.Number.Uint64())
		require.Equal(t, w.hash, out.Header.Hash())
	}
}

func TestEthReceiptsFileIterator_Pipeline(t *testing.T) {
	t.Parallel()

	// --- Arrange: create file with json-encoded [][]byte of RLP receipts per line
	tmp := t.TempDir()
	path := filepath.Join(tmp, "eth_receipts.json")

	start, end := uint64(200), uint64(201)

	var blocks []struct {
		num  uint64
		hash common.Hash
		nTx  int
	}

	f, err := os.Create(path)
	require.NoError(t, err)

	for n := start; n <= end; n++ {
		var bh common.Hash

		bh[0], bh[1], bh[2], bh[3] = byte(n), 0xAA, 0xBB, 0xCC
		nTx := 3

		// Build receipts with required fields, then RLP-encode each
		entries := make([]gethtypes.Receipt, 0, nTx)
		for i := range nTx {
			rc := gethtypes.Receipt{
				Status:            1,
				CumulativeGasUsed: 21000 * uint64(i+1),
				GasUsed:           21000,
				BlockNumber:       new(big.Int).SetUint64(n),
				BlockHash:         bh,
				TransactionIndex:  uint(i),
				Logs: []*gethtypes.Log{
					{Topics: []common.Hash{{}}},
				},
			}
			// deterministic TxHash
			rc.TxHash[0] = byte(i)
			rc.TxHash[1] = byte(n)

			entries = append(entries, rc)
		}

		var line []byte

		line, err = json.Marshal(entries)
		require.NoError(t, err)

		_, err = f.Write(append(line, '\n'))
		require.NoError(t, err)

		blocks = append(blocks, struct {
			num  uint64
			hash common.Hash
			nTx  int
		}{num: n, hash: bh, nTx: nTx})
	}

	require.NoError(t, f.Close())

	// --- Arrange: open DB writer
	evmDBPath := filepath.Join(tmp, "evmdb")

	writerDB := openMDBXAt(t, evmDBPath, gosdk.EvmTables())
	defer writerDB.Close()

	// --- Act: iterate file -> write via FixtureWriter.writeOne
	it, err := NewEthReceiptsFileIterator(path)
	require.NoError(t, err)

	fw := &FixtureWriter[[]gethtypes.Receipt]{DB: writerDB}
	ctx := t.Context()

	i := 0

	for {
		var recs []gethtypes.Receipt

		recs, err = it.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err, i)
		require.NoError(t, fw.writeOne(ctx, recs))

		i++
	}

	err = it.Close()
	require.NoError(t, err)

	writerDB.Close()

	// --- Assert: read back using MultichainStateAccess
	chainDBs, err := gosdk.NewMultichainStateAccessDB(gosdk.MultichainConfig{
		apptypes.ChainType(1): evmDBPath,
	})
	require.NoError(t, err)

	mc := gosdk.NewMultichainStateAccess(chainDBs)
	defer mc.Close()

	for _, b := range blocks {
		out, err := mc.EthReceipts(ctx, apptypes.MakeExternalBlock(1, b.num, b.hash))
		require.NoError(t, err)
		require.Len(t, out, b.nTx)

		for i, r := range out {
			require.Equal(t, b.hash, r.BlockHash)
			require.Equal(t, b.num, r.BlockNumber.Uint64())
			require.Equal(t, uint(i), r.TransactionIndex)
		}
	}
}

func TestSolBlockFileIterator_Pipeline(t *testing.T) {
	t.Parallel()

	// --- Arrange: create file with CBOR-encoded client.Block per line
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sol_blocks.cbor")

	start, end := int64(12345), int64(12347)

	type rec struct {
		slot int64
		hash [32]byte
	}

	var want []rec

	f, err := os.Create(path)
	require.NoError(t, err)

	for slot := start; slot <= end; slot++ {
		var h [32]byte

		_, _ = rand.Read(h[:])

		blk := &client.Block{
			BlockHeight: ptr(slot),
			Blockhash:   base58.Encode(h[:]),
		}

		var b []byte

		b, err = json.Marshal(blk)
		require.NoError(t, err)

		_, err = f.Write(append(b, '\n'))
		require.NoError(t, err)

		want = append(want, rec{slot: slot, hash: h})
	}

	require.NoError(t, f.Close())

	// --- Arrange: open DB writer (Solana tables)
	svmDBPath := filepath.Join(tmp, "svmdb")
	writerDB := openMDBXAt(t, svmDBPath, gosdk.SolanaTables())

	// --- Act: iterate file -> write via FixtureWriter.writeOne
	it, err := NewSolBlockFileIterator(path)
	require.NoError(t, err)

	fw := &FixtureWriter[*client.Block]{DB: writerDB}
	ctx := t.Context()

	for {
		var blk *client.Block

		blk, err = it.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)
		require.NoError(t, fw.writeOne(ctx, blk))
	}

	err = it.Close()
	require.NoError(t, err)

	writerDB.Close()

	// --- Assert: read back using MultichainStateAccess
	chainDBs, err := gosdk.NewMultichainStateAccessDB(gosdk.MultichainConfig{
		gosdk.SolanaDevnetChainID: svmDBPath,
	})
	require.NoError(t, err)

	mc := gosdk.NewMultichainStateAccess(chainDBs)
	defer mc.Close()

	for _, w := range want {
		out, err := mc.SolanaBlock(
			ctx,
			apptypes.MakeExternalBlock(uint64(gosdk.SolanaDevnetChainID), uint64(w.slot), w.hash),
		)
		require.NoError(t, err)
		require.NotNil(t, out.BlockHeight)
		require.Equal(t, w.slot, *out.BlockHeight)
		got, err := base58.Decode(out.Blockhash)
		require.NoError(t, err)
		require.True(t, bytes.Equal(w.hash[:], got), "hash mismatch: want %s got %s",
			hex.EncodeToString(w.hash[:]), out.Blockhash)
	}
}

// --- tiny helpers ---

func ptr[T any](v T) *T { return &v }
