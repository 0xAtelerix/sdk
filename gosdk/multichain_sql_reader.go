package gosdk

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/goccy/go-json"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for multichain state access.
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
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
	db *sql.DB
}

func openMultichainSQLiteReader(
	ctx context.Context,
	dbPath string,
) (*multichainSQLiteReader, error) {
	db, err := openMultichainSQLiteDB(ctx, dbPath, "ro")
	if err != nil {
		return nil, err
	}

	return &multichainSQLiteReader{db: db}, nil
}

func (r *multichainSQLiteReader) Close() error {
	if r == nil || r.db == nil {
		return nil
	}

	return r.db.Close()
}

func (r *multichainSQLiteReader) readEVMBlockRow(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]byte, int64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var (
		rawBlock []byte
		num      int64
	)

	err := r.db.QueryRowContext(
		ctx,
		multichainEVMBlockQuery,
		block.BlockHash[:],
		block.BlockNumber,
	).Scan(&rawBlock, &num)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, false, nil
		}

		return nil, 0, false, err
	}

	return rawBlock, num, true, nil
}

func (r *multichainSQLiteReader) readEVMReceipts(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]evmtypes.Receipt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rows, err := r.db.QueryContext(
		ctx,
		multichainEVMReceiptsQuery,
		block.BlockHash[:],
		block.BlockNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("sql query receipts: %w", err)
	}
	defer rows.Close()

	var receipts []evmtypes.Receipt

	for rows.Next() {
		var raw []byte

		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf(
				"failed to read eth receipts: %w, chainID %d, block number %d, block hash %s",
				err,
				block.ChainID,
				block.BlockNumber,
				hex.EncodeToString(block.BlockHash[:]),
			)
		}

		if raw == nil {
			log.Error().
				Uint64("block", block.BlockNumber).
				Uint64("chain", block.ChainID).
				Msg("receipt not found")
			time.Sleep(100 * time.Millisecond)

			continue
		}

		var receipt evmtypes.Receipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return nil, fmt.Errorf("decode receipt: %w", err)
		}

		receipt.Raw = raw
		receipts = append(receipts, receipt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	return receipts, nil
}

func (r *multichainSQLiteReader) readMidnightBlock(
	ctx context.Context,
	block apptypes.ExternalBlock,
) (*MidnightBlock, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var mb MidnightBlock

	err := r.db.QueryRowContext(
		ctx,
		multichainMidnightBlockQuery,
		block.BlockHash[:],
		block.BlockNumber,
	).Scan(&mb.Hash, &mb.Number, &mb.ParentHash, &mb.Timestamp, &mb.RawBlock)
	if err != nil {
		return nil, err
	}

	return &mb, nil
}

func (r *multichainSQLiteReader) readMidnightActions(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]MidnightContractAction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rows, err := r.db.QueryContext(
		ctx,
		multichainMidnightActionsQuery,
		block.BlockHash[:],
		block.BlockNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("query midnight contract actions: %w", err)
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
			return nil, fmt.Errorf("scan contract action: %w", err)
		}

		if entryPoint.Valid {
			action.EntryPoint = entryPoint.String
		}

		actions = append(actions, action)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	return actions, nil
}

// openMultichainSQLiteDB opens a live multichain SQLite reader.
// CEX hot-path readers use zombiezen; multichain appchain readers keep
// database/sql because they observe WAL-backed oracle writes while appchain
// execution is waiting on freshly voted blocks.
func openMultichainSQLiteDB(ctx context.Context, dbPath string, mode string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=%s&cache=shared&uri=true", dbPath, mode)
	log.Info().Str("path", dsn).Msg("connecting to sqlite")

	var (
		db  *sql.DB
		err error
	)

	maxTries := 50

	for {
		db, err = sql.Open("sqlite3", dsn)
		if err != nil {
			log.Error().Err(err).Msg("failed to open sqlite db")

			if retryErr := waitMultichainSQLiteRetry(ctx, &maxTries, err); retryErr != nil {
				return nil, retryErr
			}

			continue
		}

		if pingErr := db.PingContext(ctx); pingErr != nil {
			log.Error().Err(pingErr).Str("dsn", dsn).Str("path", dbPath).Msg("sqlite ping failed")

			if closeErr := db.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg("failed to close sqlite db")
			}

			if retryErr := waitMultichainSQLiteRetry(ctx, &maxTries, pingErr); retryErr != nil {
				return nil, retryErr
			}

			continue
		}

		log.Info().Str("path", dbPath).Msg("sqlite db opened")

		if mode == "ro" {
			if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON;"); err != nil {
				log.Warn().Err(err).Msg("unable to enforce query_only; continue anyway")
			}

			if _, err := db.ExecContext(ctx, "PRAGMA wal_autocheckpoint = 0;"); err != nil {
				log.Warn().Err(err).Msg("unable to disable wal_autocheckpoint; continue anyway")
			}
		}

		return db, nil
	}
}

func waitMultichainSQLiteRetry(ctx context.Context, maxTries *int, err error) error {
	if *maxTries == 0 {
		return err
	}

	*maxTries--

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
