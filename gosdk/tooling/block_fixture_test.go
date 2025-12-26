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
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/scheme"
)

// --- minimal test fixture writer matching your read-side keying ---

type testFixtureWriter struct{ db kv.RwDB }

func (w *testFixtureWriter) putEVMBlock(t *testing.T, blk *evmtypes.Block) {
	t.Helper()

	num := blk.Number.ToInt().Uint64()
	h := blk.Hash

	key := make([]byte, 8+32)
	binary.BigEndian.PutUint64(key[:8], num)
	copy(key[8:], h[:])

	enc, err := json.Marshal(blk)
	require.NoError(t, err)

	require.NoError(t, w.db.Update(t.Context(), func(tx kv.RwTx) error {
		return tx.Put(scheme.EvmBlocks, key, enc)
	}))
}

func (w *testFixtureWriter) putEvmReceipts(
	t *testing.T,
	blockNum uint64,
	blockHash common.Hash,
	recs []*evmtypes.Receipt,
) {
	t.Helper()
	err := w.db.Update(t.Context(), func(tx kv.RwTx) error {
		for i, r := range recs {
			// evmtypes.Receipt already has all the fields we need in RPC format
			enc, err := json.Marshal(r)
			if err != nil {
				return err
			}

			key := gosdk.EVMReceiptKey(blockNum, blockHash, uint32(i))

			if err = tx.Put(scheme.EvmReceipts, key, enc); err != nil {
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
		return tx.Put(scheme.SolanaBlocks, key, enc)
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

func makeEvmBlock(t *testing.T, num uint64) *evmtypes.Block {
	t.Helper()

	// Canonical empty trie roots
	emptyTrieRoot := common.HexToHash(
		"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
	)
	emptyUnclesHash := common.HexToHash(
		"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
	)

	header := &evmtypes.Header{
		Number:           (*hexutil.Big)(new(big.Int).SetUint64(num)),
		ParentHash:       common.BytesToHash([]byte(fmt.Sprintf("parent-%d", num-1))),
		Sha3Uncles:       emptyUnclesHash,
		Miner:            common.Address{},
		StateRoot:        emptyTrieRoot,
		TransactionsRoot: emptyTrieRoot,
		ReceiptsRoot:     emptyTrieRoot,
		LogsBloom:        gethtypes.Bloom{},
		Difficulty:       (*hexutil.Big)(big.NewInt(1)),
		GasLimit:         30_000_000,
		GasUsed:          0,
		Time:             hexutil.Uint64(time.Now().Unix()),
		ExtraData:        []byte("test"),
		BaseFeePerGas:    (*hexutil.Big)(big.NewInt(7)),
	}

	header.Hash = header.ComputeHash()

	return evmtypes.NewBlock(header, []evmtypes.Transaction{})
}

func makeEvmReceipts(
	t *testing.T,
	blockNumber uint64,
	blockHash common.Hash,
	transactions []evmtypes.Transaction,
	n int,
) []*evmtypes.Receipt {
	t.Helper()

	recs := make([]*evmtypes.Receipt, n)
	for i := range n {
		var txHash common.Hash
		if i < len(transactions) {
			txHash = transactions[i].Hash
		} else {
			// deterministic dummy if you didn't populate transactions
			txHash[0] = byte(i)
		}

		r := &evmtypes.Receipt{
			Status:            1,
			CumulativeGasUsed: hexutil.Uint64(21000),
			GasUsed:           hexutil.Uint64(21000),
			BlockNumber:       hexutil.Uint64(blockNumber),
			BlockHash:         blockHash,
			TransactionIndex:  hexutil.Uint64(i),
			TxHash:            txHash,
			Logs: []*gethtypes.Log{
				{
					Topics: []common.Hash{{}},
				},
			},
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

	blk := makeEvmBlock(t, blockNum)
	fw.putEVMBlock(t, blk)

	recs := makeEvmReceipts(t, blockNum, blk.Hash, blk.Transactions, 3)
	fw.putEvmReceipts(t, blockNum, blk.Hash, recs)

	writerDB.Close()

	// open read-only accessor and read back
	chainDBs, err := gosdk.NewMultichainStateAccessDB(gosdk.MultichainConfig{
		apptypes.ChainType(chainID): evmDBPath,
	})
	require.NoError(t, err)

	mc := gosdk.NewMultichainStateAccess(chainDBs)
	defer mc.Close()

	ctx := context.Background()

	outBlk, err := mc.EVMBlock(
		ctx,
		apptypes.MakeExternalBlock(chainID, blockNum, blk.Hash),
	)
	require.NoError(t, err)
	require.Equal(t, blockNum, outBlk.Number.ToInt().Uint64())
	require.Equal(t, blk.Hash, outBlk.Hash)

	outRecs, err := mc.EVMReceipts(
		ctx,
		apptypes.MakeExternalBlock(chainID, blockNum, blk.Hash),
	)

	require.NoError(t, err)
	require.Len(t, outRecs, len(recs))

	for _, r := range outRecs {
		require.Equal(t, blk.Hash, r.BlockHash)
		require.Equal(t, blk.Number.ToInt().Uint64(), uint64(r.BlockNumber))
	}
}

func TestSolana_Block_RoundTrip(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	svmDBPath := filepath.Join(tmp, "svmdb")

	// create writer DB with Solana tables and write fixture
	writerDB := openMDBXAt(t, svmDBPath, gosdk.SolanaTables())

	fw := &testFixtureWriter{db: writerDB}

	const chainID = uint64(library.SolanaDevnetChainID) // use one of your Solana ChainType values

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

func TestEVMBlockFileIterator_Pipeline(t *testing.T) {
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
		blk := makeEvmBlock(t, n)

		var b []byte

		b, err = json.Marshal(blk)
		require.NoError(t, err)

		var evmBlock evmtypes.Block

		err = json.Unmarshal(b, &evmBlock)
		require.NoError(t, err)

		_, err = f.Write(append(b, '\n'))
		require.NoError(t, err)

		want = append(want, rec{num: n, hash: blk.Hash})
	}

	require.NoError(t, f.Close())

	// --- Arrange: open DB writer
	evmDBPath := filepath.Join(tmp, "evmdb")
	writerDB := openMDBXAt(t, evmDBPath, gosdk.EvmTables())

	// --- Act: iterate file -> write via FixtureWriter.writeOne
	it, err := NewEVMBlockFileIterator(path)
	require.NoError(t, err)

	fw := &FixtureWriter[*evmtypes.Block]{DB: writerDB}

	ctx := t.Context()

	for {
		var blk *evmtypes.Block

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
		out, err := mc.EVMBlock(ctx, apptypes.MakeExternalBlock(1, w.num, w.hash))
		require.NoError(t, err)
		require.Equal(t, w.num, out.Number.ToInt().Uint64())
		require.Equal(t, w.hash, out.Hash)
	}
}

func TestEVMReceiptsFileIterator_Pipeline(t *testing.T) {
	t.Parallel()

	// --- Arrange: create file with json-encoded [][]byte of RLP receipts per line
	tmp := t.TempDir()
	path := filepath.Join(tmp, "eth_receipts.json")

	start, end := uint64(200), uint64(201)

	type blockInfo struct {
		num  uint64
		hash common.Hash
		nTx  int
	}

	var blocks []blockInfo

	f, err := os.Create(path)
	require.NoError(t, err)

	for n := start; n <= end; n++ {
		nTx := 3

		// Build receipts with required fields (initially with dummy block hash)
		entries := make([]*evmtypes.Receipt, 0, nTx)
		for i := range nTx {
			txHash := common.Hash{}
			txHash[0] = byte(i)
			txHash[1] = byte(n)

			rc := &evmtypes.Receipt{
				Status:            1,
				CumulativeGasUsed: hexutil.Uint64(21000 * uint64(i+1)),
				GasUsed:           hexutil.Uint64(21000),
				BlockNumber:       hexutil.Uint64(n),
				BlockHash:         common.Hash{}, // Will be updated after block creation
				TransactionIndex:  hexutil.Uint64(i),
				TxHash:            txHash,
				Logs: []*gethtypes.Log{
					{Topics: []common.Hash{{}}},
				},
			}

			entries = append(entries, rc)
		}

		// Create block
		blk := makeEvmBlock(t, n)

		// Update receipts with the correct block hash
		for _, r := range entries {
			r.BlockHash = blk.Hash
		}

		var line []byte

		line, err = json.Marshal(entries)
		require.NoError(t, err)

		_, err = f.Write(append(line, '\n'))
		require.NoError(t, err)

		blocks = append(blocks, blockInfo{
			num:  n,
			hash: blk.Hash,
			nTx:  nTx,
		})
	}

	require.NoError(t, f.Close())

	// --- Arrange: open DB writer
	evmDBPath := filepath.Join(tmp, "evmdb")

	writerDB := openMDBXAt(t, evmDBPath, gosdk.EvmTables())
	defer writerDB.Close()

	ctx := t.Context()

	// --- Act: iterate file -> write via FixtureWriter.writeOne
	it, err := NewEVMReceiptsFileIterator(path)
	require.NoError(t, err)

	fw := &FixtureWriter[[]*evmtypes.Receipt]{DB: writerDB}

	i := 0

	for {
		var recs []*evmtypes.Receipt

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
		out, err := mc.EVMReceipts(ctx, apptypes.MakeExternalBlock(1, b.num, b.hash))
		require.NoError(t, err)
		require.Len(t, out, b.nTx)

		for i, r := range out {
			require.Equal(t, b.hash, r.BlockHash)
			require.Equal(t, b.num, uint64(r.BlockNumber))
			require.Equal(t, uint64(i), uint64(r.TransactionIndex))
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
		library.SolanaDevnetChainID: svmDBPath,
	})
	require.NoError(t, err)

	mc := gosdk.NewMultichainStateAccess(chainDBs)
	defer mc.Close()

	for _, w := range want {
		out, err := mc.SolanaBlock(
			ctx,
			apptypes.MakeExternalBlock(uint64(library.SolanaDevnetChainID), uint64(w.slot), w.hash),
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
