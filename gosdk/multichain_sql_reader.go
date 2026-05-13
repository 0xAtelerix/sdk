package gosdk

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog/log"
	zsqlite "zombiezen.com/go/sqlite"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	"github.com/0xAtelerix/sdk/gosdk/internal/sqlitez"
)

const (
	multichainEVMBlockQuery = `
SELECT raw_block, number
FROM blocks
WHERE hash = ? AND number = ?`
	multichainEVMReceiptsQuery = `
SELECT raw_receipt
FROM receipts
WHERE block_hash = ? AND block_number = ?
ORDER BY tx_index`
	multichainMidnightBlockQuery = `
SELECT hash, number, parent_hash, timestamp, raw_block
FROM blocks
WHERE hash = ? AND number = ?`
	multichainMidnightActionsQuery = `
SELECT contract_addr, action_type, entry_point, state, raw_action
FROM contract_actions
WHERE block_hash = ? AND block_number = ?
ORDER BY id`
)

type multichainSQLiteReader struct {
	mu sync.Mutex

	conn            *zsqlite.Conn
	evmBlock        *zsqlite.Stmt
	evmReceipts     *zsqlite.Stmt
	midnightBlock   *zsqlite.Stmt
	midnightActions *zsqlite.Stmt
}

func openMultichainSQLiteReader(
	ctx context.Context,
	dbPath string,
) (*multichainSQLiteReader, error) {
	conn, err := sqlitez.OpenConn(ctx, dbPath, "ro", sqlitez.OpenOptions{
		QueryOnly:                true,
		DisableWALAutoCheckpoint: true,
	})
	if err != nil {
		return nil, err
	}

	return &multichainSQLiteReader{
		conn: conn,
	}, nil
}

func (r *multichainSQLiteReader) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}

	return r.conn.Close()
}

func (r *multichainSQLiteReader) preparedEVMBlock() (*zsqlite.Stmt, error) {
	if r.evmBlock != nil {
		return r.evmBlock, nil
	}

	stmt, err := r.conn.Prepare(multichainEVMBlockQuery)
	if err != nil {
		return nil, err
	}

	r.evmBlock = stmt

	return stmt, nil
}

func (r *multichainSQLiteReader) preparedEVMReceipts() (*zsqlite.Stmt, error) {
	if r.evmReceipts != nil {
		return r.evmReceipts, nil
	}

	stmt, err := r.conn.Prepare(multichainEVMReceiptsQuery)
	if err != nil {
		return nil, err
	}

	r.evmReceipts = stmt

	return stmt, nil
}

func (r *multichainSQLiteReader) preparedMidnightBlock() (*zsqlite.Stmt, error) {
	if r.midnightBlock != nil {
		return r.midnightBlock, nil
	}

	stmt, err := r.conn.Prepare(multichainMidnightBlockQuery)
	if err != nil {
		return nil, err
	}

	r.midnightBlock = stmt

	return stmt, nil
}

func (r *multichainSQLiteReader) preparedMidnightActions() (*zsqlite.Stmt, error) {
	if r.midnightActions != nil {
		return r.midnightActions, nil
	}

	stmt, err := r.conn.Prepare(multichainMidnightActionsQuery)
	if err != nil {
		return nil, err
	}

	r.midnightActions = stmt

	return stmt, nil
}

func (r *multichainSQLiteReader) readEVMBlockRow(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]byte, int64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldDone := r.conn.SetInterrupt(ctx.Done())
	defer r.conn.SetInterrupt(oldDone)

	stmt, err := r.preparedEVMBlock()
	if err != nil {
		return nil, 0, false, err
	}

	stmt.BindBytes(1, block.BlockHash[:])
	stmt.BindInt64(2, int64(block.BlockNumber))

	hasRow, err := stmt.Step()
	if err != nil {
		_ = resetMultichainSQLiteStmt(stmt)

		return nil, 0, false, err
	}

	if !hasRow {
		if resetErr := resetMultichainSQLiteStmt(stmt); resetErr != nil {
			return nil, 0, false, resetErr
		}

		return nil, 0, false, nil
	}

	rawBlock := sqliteColumnBytesCopy(stmt, 0)

	num := stmt.ColumnInt64(1)
	if resetErr := resetMultichainSQLiteStmt(stmt); resetErr != nil {
		return nil, 0, false, resetErr
	}

	return rawBlock, num, true, nil
}

func (r *multichainSQLiteReader) readEVMReceipts(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]evmtypes.Receipt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldDone := r.conn.SetInterrupt(ctx.Done())
	defer r.conn.SetInterrupt(oldDone)

	stmt, err := r.preparedEVMReceipts()
	if err != nil {
		return nil, err
	}

	stmt.BindBytes(1, block.BlockHash[:])
	stmt.BindInt64(2, int64(block.BlockNumber))

	defer func() {
		if err := resetMultichainSQLiteStmt(stmt); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("reset evm receipts statement")
		}
	}()

	var receipts []evmtypes.Receipt

	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read eth receipts: %w, chainID %d, block number %d, block hash %s",
				err,
				block.ChainID,
				block.BlockNumber,
				hex.EncodeToString(block.BlockHash[:]),
			)
		}

		if !hasRow {
			return receipts, nil
		}

		if stmt.ColumnIsNull(0) {
			log.Error().
				Uint64("block", block.BlockNumber).
				Uint64("chain", block.ChainID).
				Msg("receipt not found")
			time.Sleep(100 * time.Millisecond)

			continue
		}

		raw := sqliteColumnBytesCopy(stmt, 0)

		var receipt evmtypes.Receipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return nil, fmt.Errorf("decode receipt: %w", err)
		}

		receipt.Raw = raw
		receipts = append(receipts, receipt)
	}
}

func (r *multichainSQLiteReader) readMidnightBlock(
	ctx context.Context,
	block apptypes.ExternalBlock,
) (*MidnightBlock, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldDone := r.conn.SetInterrupt(ctx.Done())
	defer r.conn.SetInterrupt(oldDone)

	stmt, err := r.preparedMidnightBlock()
	if err != nil {
		return nil, err
	}

	stmt.BindBytes(1, block.BlockHash[:])
	stmt.BindInt64(2, int64(block.BlockNumber))

	hasRow, err := stmt.Step()
	if err != nil {
		_ = resetMultichainSQLiteStmt(stmt)

		return nil, err
	}

	if !hasRow {
		if resetErr := resetMultichainSQLiteStmt(stmt); resetErr != nil {
			return nil, resetErr
		}

		return nil, errSQLiteRowNotFound
	}

	blockOut := &MidnightBlock{
		Hash:       sqliteColumnBytesCopy(stmt, 0),
		Number:     uint64(stmt.ColumnInt64(1)),
		ParentHash: sqliteColumnBytesCopy(stmt, 2),
		Timestamp:  stmt.ColumnInt64(3),
		RawBlock:   sqliteColumnBytesCopy(stmt, 4),
	}
	if resetErr := resetMultichainSQLiteStmt(stmt); resetErr != nil {
		return nil, resetErr
	}

	return blockOut, nil
}

func (r *multichainSQLiteReader) readMidnightActions(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]MidnightContractAction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldDone := r.conn.SetInterrupt(ctx.Done())
	defer r.conn.SetInterrupt(oldDone)

	stmt, err := r.preparedMidnightActions()
	if err != nil {
		return nil, err
	}

	stmt.BindBytes(1, block.BlockHash[:])
	stmt.BindInt64(2, int64(block.BlockNumber))

	defer func() {
		if err := resetMultichainSQLiteStmt(stmt); err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("reset midnight actions statement")
		}
	}()

	var actions []MidnightContractAction

	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return nil, fmt.Errorf("query midnight contract actions: %w", err)
		}

		if !hasRow {
			return actions, nil
		}

		action := MidnightContractAction{
			ContractAddr: sqliteColumnBytesCopy(stmt, 0),
			ActionType:   stmt.ColumnText(1),
			State:        sqliteColumnBytesCopy(stmt, 3),
			RawAction:    sqliteColumnBytesCopy(stmt, 4),
		}
		if !stmt.ColumnIsNull(2) {
			action.EntryPoint = stmt.ColumnText(2)
		}

		actions = append(actions, action)
	}
}

func sqliteColumnBytesCopy(stmt *zsqlite.Stmt, col int) []byte {
	buf := make([]byte, stmt.ColumnLen(col))
	stmt.ColumnBytes(col, buf)

	return buf
}

func resetMultichainSQLiteStmt(stmt *zsqlite.Stmt) error {
	if resetErr := stmt.Reset(); resetErr != nil {
		_ = stmt.ClearBindings()

		return resetErr
	}

	return stmt.ClearBindings()
}
