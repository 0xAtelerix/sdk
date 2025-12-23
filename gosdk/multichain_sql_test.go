package gosdk

import (
	"context"
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
)

// Ensures the SQLite-backed MultichainStateAccessSQL can read blocks/receipts that match the simpleappchain schema.
func TestMultichainStateAccessSQL_EthBlockAndReceipts(t *testing.T) {
	ctx := context.Background()
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
	dbs, err := NewMultichainStateAccessSQLDB(ctx, MultichainConfig{EthereumChainID: chainDir})
	require.NoError(t, err)

	msa := NewMultichainStateAccessSQL(dbs)

	extBlock := apptypes.ExternalBlock{
		ChainID:     uint64(EthereumChainID),
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
