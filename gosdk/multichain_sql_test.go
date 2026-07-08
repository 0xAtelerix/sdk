package gosdk

import (
	"context"
	"database/sql"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/goccy/go-json"
	_ "github.com/mattn/go-sqlite3" // sqlite driver
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
)

// Ensures the SQLite-backed MultichainStateAccessSQL can read blocks/receipts that match the simpleappchain schema.
func TestMultichainStateAccessSQL_EthBlockAndReceipts(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tmp := t.TempDir()

	chainDir := filepath.Join(tmp, "evm1")
	require.NoError(t, os.MkdirAll(chainDir, 0o755))
	dbPath := filepath.Join(chainDir, "sqlite")

	// Prepare a SQLite fixture with the schema used by simpleappchain.
	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `
CREATE TABLE blocks (
	hash BLOB PRIMARY KEY,
	number INTEGER,
	raw_block BLOB
);
CREATE TABLE receipts (
	block_hash BLOB,
	block_number INTEGER,
	tx_index INTEGER,
	raw_receipt BLOB
);
`)
	require.NoError(t, err)

	// Build a minimal Ethereum block and receipt and store them as JSON blobs.
	header := &evmtypes.Header{
		Number: (*hexutil.Big)(big.NewInt(10)),
		Time:   hexutil.Uint64(time.Now().Unix()),
	}

	header.Hash = header.ComputeHash()

	ethBlock := evmtypes.NewBlock(header, nil)
	rawBlock, err := json.Marshal(ethBlock)
	require.NoError(t, err)

	r := gethtypes.Receipt{
		TxHash:      common.HexToHash("0x01"),
		BlockHash:   ethBlock.Hash,
		BlockNumber: (*big.Int)(header.Number),
		Status:      gethtypes.ReceiptStatusSuccessful,
		Logs:        []*gethtypes.Log{},
	}
	rawReceipt, err := json.Marshal(r)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO blocks(hash, number, raw_block) VALUES(?, ?, ?)",
		ethBlock.Hash.Bytes(), header.Number.ToInt().Uint64(), rawBlock,
	)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO receipts(block_hash, block_number, tx_index, raw_receipt) VALUES(?, ?, ?, ?)",
		ethBlock.Hash.Bytes(), header.Number.ToInt().Uint64(), 0, rawReceipt,
	)
	require.NoError(t, err)

	// Open through the SQL multichain helper and read back.
	msa, err := NewMultichainStateAccessSQL(
		ctx,
		MultichainConfig{library.EthereumChainID: chainDir},
	)
	require.NoError(t, err)

	extBlock := apptypes.ExternalBlock{
		ChainID:     uint64(library.EthereumChainID),
		BlockNumber: header.Number.ToInt().Uint64(),
		BlockHash:   ethBlock.Hash,
	}

	gotBlock, err := msa.EVMBlock(ctx, extBlock)
	require.NoError(t, err)
	require.Equal(t, header.Number.ToInt().Uint64(), gotBlock.Number.ToInt().Uint64())
	require.Equal(t, ethBlock.Hash, gotBlock.Hash)

	rcpts, err := msa.EVMReceipts(ctx, extBlock)
	require.NoError(t, err)
	require.Len(t, rcpts, 1)
	require.Equal(t, ethBlock.Hash, rcpts[0].BlockHash)
}

func TestMultichainStateAccessSQL_MidnightBlockAndActions(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	chainDir := filepath.Join(t.TempDir(), "midnight")
	require.NoError(t, os.MkdirAll(chainDir, 0o755))
	extBlock := seedMidnightSQLFixture(ctx, t, chainDir)

	msa, err := NewMultichainStateAccessSQL(
		ctx,
		MultichainConfig{library.MidnightPreviewChainID: chainDir},
	)
	require.NoError(t, err)

	defer msa.Close()

	gotBlock, err := msa.MidnightBlockByHash(ctx, extBlock)
	require.NoError(t, err)
	require.Equal(t, extBlock.BlockHash[:], gotBlock.Hash)
	require.Equal(t, extBlock.BlockNumber, gotBlock.Number)

	actions, err := msa.MidnightContractActions(ctx, extBlock)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, []byte("contract"), actions[0].ContractAddr)
	require.Equal(t, "call", actions[0].ActionType)
	require.Equal(t, "entry", actions[0].EntryPoint)
}

func TestMultichainStateAccessSQL_EVMBlockReturnsWhenContextExpires(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	chainDir := filepath.Join(t.TempDir(), "evm1")
	require.NoError(t, os.MkdirAll(chainDir, 0o755))
	dbPath := filepath.Join(chainDir, "sqlite")

	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `
CREATE TABLE blocks (
	hash BLOB PRIMARY KEY,
	number INTEGER,
	raw_block BLOB
);
CREATE TABLE receipts (
	block_hash BLOB,
	block_number INTEGER,
	tx_index INTEGER,
	raw_receipt BLOB
);
`)
	require.NoError(t, err)

	msa, err := NewMultichainStateAccessSQL(
		ctx,
		MultichainConfig{library.EthereumChainID: chainDir},
	)
	require.NoError(t, err)

	defer msa.Close()

	extBlock, _ := makeEVMBlockFixture(t, 10)

	readCtx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		_, readErr := msa.EVMBlock(readCtx, extBlock)
		errCh <- readErr
	}()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("EVMBlock did not return after context deadline")
	}
}

func TestMultichainStateAccessSQL_EVMBlockSeesBlockInsertedAfterMissLoop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	chainDir := filepath.Join(t.TempDir(), "evm1")
	require.NoError(t, os.MkdirAll(chainDir, 0o755))
	dbPath := filepath.Join(chainDir, "sqlite")

	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(t, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
CREATE TABLE blocks (
	hash BLOB PRIMARY KEY,
	number INTEGER,
	raw_block BLOB
);
`)
	require.NoError(t, err)

	msa, err := NewMultichainStateAccessSQL(
		ctx,
		MultichainConfig{library.EthereumChainID: chainDir},
	)
	require.NoError(t, err)

	defer msa.Close()

	extBlock, rawBlock := makeEVMBlockFixture(t, 10)

	readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		_, readErr := msa.EVMBlock(readCtx, extBlock)
		errCh <- readErr
	}()

	time.Sleep(200 * time.Millisecond)

	_, err = db.ExecContext(ctx,
		"INSERT INTO blocks(hash, number, raw_block) VALUES(?, ?, ?)",
		extBlock.BlockHash[:], extBlock.BlockNumber, rawBlock,
	)
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("EVMBlock did not see block inserted after miss loop")
	}
}

func BenchmarkMultichainStateAccessSQL_EVMReads(b *testing.B) {
	ctx := b.Context()
	chainDir := filepath.Join(b.TempDir(), "evm1")
	require.NoError(b, os.MkdirAll(chainDir, 0o755))
	extBlock := seedEVMSQLFixture(ctx, b, chainDir)

	db, err := openSQLite(ctx, filepath.Join(chainDir, "sqlite"), "ro")
	require.NoError(b, err)

	defer db.Close()

	msa, err := NewMultichainStateAccessSQL(
		ctx,
		MultichainConfig{library.EthereumChainID: chainDir},
	)
	require.NoError(b, err)

	defer msa.Close()

	b.Run("direct_database_sql_block", func(b *testing.B) {
		for range b.N {
			_, err := legacyEVMBlockSQL(ctx, db, extBlock)
			require.NoError(b, err)
		}
	})

	b.Run("accessor_block", func(b *testing.B) {
		for range b.N {
			_, err := msa.EVMBlock(ctx, extBlock)
			require.NoError(b, err)
		}
	})

	b.Run("direct_database_sql_receipts", func(b *testing.B) {
		for range b.N {
			_, err := legacyEVMReceiptsSQL(ctx, db, extBlock)
			require.NoError(b, err)
		}
	})

	b.Run("accessor_receipts", func(b *testing.B) {
		for range b.N {
			_, err := msa.EVMReceipts(ctx, extBlock)
			require.NoError(b, err)
		}
	})
}

func BenchmarkMultichainStateAccessSQL_MidnightReads(b *testing.B) {
	ctx := b.Context()
	chainDir := filepath.Join(b.TempDir(), "midnight")
	require.NoError(b, os.MkdirAll(chainDir, 0o755))
	extBlock := seedMidnightSQLFixture(ctx, b, chainDir)

	db, err := openSQLite(ctx, filepath.Join(chainDir, "sqlite"), "ro")
	require.NoError(b, err)

	defer db.Close()

	msa, err := NewMultichainStateAccessSQL(
		ctx,
		MultichainConfig{library.MidnightPreviewChainID: chainDir},
	)
	require.NoError(b, err)

	defer msa.Close()

	b.Run("direct_database_sql_block", func(b *testing.B) {
		for range b.N {
			_, err := legacyMidnightBlockSQL(ctx, db, extBlock)
			require.NoError(b, err)
		}
	})

	b.Run("accessor_block", func(b *testing.B) {
		for range b.N {
			_, err := msa.MidnightBlockByHash(ctx, extBlock)
			require.NoError(b, err)
		}
	})

	b.Run("direct_database_sql_actions", func(b *testing.B) {
		for range b.N {
			_, err := legacyMidnightActionsSQL(ctx, db, extBlock)
			require.NoError(b, err)
		}
	})

	b.Run("accessor_actions", func(b *testing.B) {
		for range b.N {
			_, err := msa.MidnightContractActions(ctx, extBlock)
			require.NoError(b, err)
		}
	})
}

func makeEVMBlockFixture(
	tb testing.TB,
	number int64,
) (apptypes.ExternalBlock, []byte) {
	tb.Helper()

	header := &evmtypes.Header{
		Number: (*hexutil.Big)(big.NewInt(number)),
		Time:   hexutil.Uint64(time.Now().Unix()),
	}
	header.Hash = header.ComputeHash()

	ethBlock := evmtypes.NewBlock(header, nil)
	rawBlock, err := json.Marshal(ethBlock)
	require.NoError(tb, err)

	return apptypes.ExternalBlock{
		ChainID:     uint64(library.EthereumChainID),
		BlockNumber: uint64(number),
		BlockHash:   ethBlock.Hash,
	}, rawBlock
}

func seedEVMSQLFixture(ctx context.Context, tb testing.TB, chainDir string) apptypes.ExternalBlock {
	tb.Helper()

	dbPath := filepath.Join(chainDir, "sqlite")
	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(tb, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `
CREATE TABLE blocks (
	hash BLOB PRIMARY KEY,
	number INTEGER,
	raw_block BLOB
);
CREATE TABLE receipts (
	block_hash BLOB,
	block_number INTEGER,
	tx_index INTEGER,
	raw_receipt BLOB
);
`)
	require.NoError(tb, err)

	header := &evmtypes.Header{
		Number: (*hexutil.Big)(big.NewInt(10)),
		Time:   hexutil.Uint64(time.Now().Unix()),
	}
	header.Hash = header.ComputeHash()

	ethBlock := evmtypes.NewBlock(header, nil)
	rawBlock, err := json.Marshal(ethBlock)
	require.NoError(tb, err)

	receipt := gethtypes.Receipt{
		TxHash:      common.HexToHash("0x01"),
		BlockHash:   ethBlock.Hash,
		BlockNumber: (*big.Int)(header.Number),
		Status:      gethtypes.ReceiptStatusSuccessful,
		Logs:        []*gethtypes.Log{},
	}
	rawReceipt, err := json.Marshal(receipt)
	require.NoError(tb, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO blocks(hash, number, raw_block) VALUES(?, ?, ?)",
		ethBlock.Hash.Bytes(), header.Number.ToInt().Uint64(), rawBlock,
	)
	require.NoError(tb, err)

	_, err = db.ExecContext(ctx,
		"INSERT INTO receipts(block_hash, block_number, tx_index, raw_receipt) VALUES(?, ?, ?, ?)",
		ethBlock.Hash.Bytes(), header.Number.ToInt().Uint64(), 0, rawReceipt,
	)
	require.NoError(tb, err)

	return apptypes.ExternalBlock{
		ChainID:     uint64(library.EthereumChainID),
		BlockNumber: header.Number.ToInt().Uint64(),
		BlockHash:   ethBlock.Hash,
	}
}

func seedMidnightSQLFixture(
	ctx context.Context,
	tb testing.TB,
	chainDir string,
) apptypes.ExternalBlock {
	tb.Helper()

	dbPath := filepath.Join(chainDir, "sqlite")
	db, err := openSQLite(ctx, dbPath, "rwc")
	require.NoError(tb, err)

	defer db.Close()

	_, err = db.ExecContext(ctx, `
CREATE TABLE blocks (
	hash BLOB PRIMARY KEY,
	number INTEGER,
	parent_hash BLOB,
	timestamp INTEGER,
	raw_block BLOB
);
CREATE TABLE contract_actions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	block_hash BLOB,
	block_number INTEGER,
	contract_addr BLOB,
	action_type TEXT,
	entry_point TEXT,
	state BLOB,
	raw_action BLOB
);
`)
	require.NoError(tb, err)

	var blockHash [32]byte
	copy(blockHash[:], []byte("midnight-block-hash"))

	_, err = db.ExecContext(ctx,
		"INSERT INTO blocks(hash, number, parent_hash, timestamp, raw_block) VALUES(?, ?, ?, ?, ?)",
		blockHash[:], uint64(44), []byte("parent"), int64(123), []byte(`{"hash":"x"}`),
	)
	require.NoError(tb, err)

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO contract_actions(block_hash, block_number, contract_addr, action_type, entry_point, state, raw_action)
VALUES(?, ?, ?, ?, ?, ?, ?)`,
		blockHash[:],
		uint64(44),
		[]byte("contract"),
		"call",
		"entry",
		[]byte("state"),
		[]byte("raw"),
	)
	require.NoError(tb, err)

	return apptypes.ExternalBlock{
		ChainID:     uint64(library.MidnightPreviewChainID),
		BlockNumber: 44,
		BlockHash:   blockHash,
	}
}

func legacyEVMBlockSQL(
	ctx context.Context,
	db *sql.DB,
	block apptypes.ExternalBlock,
) (*evmtypes.Block, error) {
	var (
		rawBlock []byte
		num      int64
	)

	if err := db.QueryRowContext(ctx, `
SELECT raw_block, number
FROM blocks
WHERE hash = ? AND number = ?`,
		block.BlockHash[:], block.BlockNumber,
	).Scan(&rawBlock, &num); err != nil {
		return nil, err
	}

	var evmBlock evmtypes.Block
	if err := json.Unmarshal(rawBlock, &evmBlock); err != nil {
		return nil, err
	}

	evmBlock.Raw = rawBlock
	evmBlock.Header.Raw = rawBlock

	if num != int64(block.BlockNumber) {
		return nil, library.ErrWrongBlock
	}

	if computedHash := evmBlock.ComputeHash(); computedHash != block.BlockHash {
		return nil, library.ErrHashMismatch
	}

	return &evmBlock, nil
}

func legacyEVMReceiptsSQL(
	ctx context.Context,
	db *sql.DB,
	block apptypes.ExternalBlock,
) ([]evmtypes.Receipt, error) {
	rows, err := db.QueryContext(ctx, `
SELECT raw_receipt
FROM receipts
WHERE block_hash = ? AND block_number = ?
ORDER BY tx_index`,
		block.BlockHash[:], block.BlockNumber,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []evmtypes.Receipt

	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}

		var receipt evmtypes.Receipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return nil, err
		}

		receipt.Raw = raw
		receipts = append(receipts, receipt)
	}

	return receipts, rows.Err()
}

func legacyMidnightBlockSQL(
	ctx context.Context,
	db *sql.DB,
	block apptypes.ExternalBlock,
) (*MidnightBlock, error) {
	var out MidnightBlock
	if err := db.QueryRowContext(ctx, `
SELECT hash, number, parent_hash, timestamp, raw_block
FROM blocks
WHERE hash = ? AND number = ?`,
		block.BlockHash[:], block.BlockNumber,
	).Scan(&out.Hash, &out.Number, &out.ParentHash, &out.Timestamp, &out.RawBlock); err != nil {
		return nil, err
	}

	return &out, nil
}

func legacyMidnightActionsSQL(
	ctx context.Context,
	db *sql.DB,
	block apptypes.ExternalBlock,
) ([]MidnightContractAction, error) {
	rows, err := db.QueryContext(ctx, `
SELECT contract_addr, action_type, entry_point, state, raw_action
FROM contract_actions
WHERE block_hash = ? AND block_number = ?
ORDER BY id`,
		block.BlockHash[:], block.BlockNumber,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []MidnightContractAction

	for rows.Next() {
		var (
			action     MidnightContractAction
			entryPoint sql.NullString
		)

		if err := rows.Scan(
			&action.ContractAddr,
			&action.ActionType,
			&entryPoint,
			&action.State,
			&action.RawAction,
		); err != nil {
			return nil, err
		}

		if entryPoint.Valid {
			action.EntryPoint = entryPoint.String
		}

		actions = append(actions, action)
	}

	return actions, rows.Err()
}
