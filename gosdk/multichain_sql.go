package gosdk

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/goccy/go-json"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for multichain state access
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
)

type MultichainStateAccessSQL struct {
	mu            sync.RWMutex
	stateAccessDB map[apptypes.ChainType]*sql.DB
}

// NewMultichainStateAccessSQL opens read-only SQLite databases for each chain
// in the config and returns a ready-to-use accessor.
func NewMultichainStateAccessSQL(
	ctx context.Context,
	cfg MultichainConfig,
) (*MultichainStateAccessSQL, error) {
	stateAccessDBs := make(map[apptypes.ChainType]*sql.DB)

	for chainID, path := range cfg {
		dbPath := filepath.Join(filepath.Clean(path), "sqlite")

		db, err := openSQLite(ctx, dbPath, "ro")
		if err != nil {
			// Close any already-opened DBs on failure.
			for _, opened := range stateAccessDBs {
				opened.Close()
			}

			return nil, err
		}

		stateAccessDBs[chainID] = db
	}

	return &MultichainStateAccessSQL{
		stateAccessDB: stateAccessDBs,
	}, nil
}

func (sa *MultichainStateAccessSQL) EVMBlock(
	ctx context.Context,
	block apptypes.ExternalBlock,
) (*evmtypes.Block, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	db, ok := sa.stateAccessDB[apptypes.ChainType(block.ChainID)]
	if !ok {
		return nil, fmt.Errorf("%w, no DB for chainID %d", library.ErrUnknownChain, block.ChainID)
	}

	var (
		rawBlock []byte
		num      int64
	)

	const query = `
		SELECT raw_block, number
		FROM blocks
		WHERE hash   = ?
		  AND number = ?
	`

	i := 0
	for {
		if i > 0 {
			time.Sleep(time.Millisecond * 100)
		}

		i++

		if err := db.QueryRowContext(ctx, query, block.BlockHash[:], block.BlockNumber).Scan(&rawBlock, &num); err != nil {
			log.Error().
				Err(err).
				Uint64("block", block.BlockNumber).
				Uint64("chain", block.ChainID).
				Msg("block not found")

			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			return nil, fmt.Errorf(
				"failed to read eth block: %w, chainID %d, block number %d, block hash %s",
				err,
				block.ChainID,
				block.BlockNumber,
				hex.EncodeToString(block.BlockHash[:]),
			)
		}

		if num != int64(block.BlockNumber) {
			log.Error().Msg(library.ErrWrongBlock.Error())

			return nil, fmt.Errorf(" %w block number mismatch: got %d, expected %d",
				library.ErrWrongBlock, num, block.BlockNumber)
		}

		var evmBlock evmtypes.Block
		if err := json.Unmarshal(rawBlock, &evmBlock); err != nil {
			log.Error().
				Err(err).
				Uint64("block", block.BlockNumber).
				Str("block", string(rawBlock)).
				Bytes("blockHash", block.BlockHash[:]).
				Msg("cant unmarshal block")

			return nil, fmt.Errorf("failed to unmarshal evm block: %w", err)
		}

		evmBlock.Raw = rawBlock
		evmBlock.Header.Raw = rawBlock

		// Verify block integrity by computing hash from header fields
		// This ensures the block data hasn't been tampered with
		computedHash := evmBlock.ComputeHash()
		if computedHash != block.BlockHash {
			return nil, fmt.Errorf(
				"%w, chainID %d; block %d; computed hash %s does not match expected hash %s",
				library.ErrHashMismatch,
				block.ChainID,
				block.BlockNumber,
				hex.EncodeToString(computedHash[:]),
				hex.EncodeToString(block.BlockHash[:]),
			)
		}

		return &evmBlock, nil
	}
}

func (sa *MultichainStateAccessSQL) EVMReceipts(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]evmtypes.Receipt, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	db, ok := sa.stateAccessDB[apptypes.ChainType(block.ChainID)]
	if !ok {
		return nil, fmt.Errorf("%w, no DB for chainID %d", library.ErrUnknownChain, block.ChainID)
	}

	const q = `
		SELECT raw_receipt
		FROM receipts
		WHERE block_hash  = ?
		  AND block_number = ?
		ORDER BY tx_index
	`

	rows, err := db.QueryContext(ctx, q, block.BlockHash[:], block.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("sql query receipts: %w", err)
	}
	defer rows.Close()

	var receipts []evmtypes.Receipt

	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Error().
					Err(err).
					Uint64("block", block.BlockNumber).
					Uint64("chain", block.ChainID).
					Msg("receipt not found")
				time.Sleep(100 * time.Millisecond)
				continue
			}

			return nil, fmt.Errorf(
				"failed to read eth block: %w, chainID %d, block number %d, block hash %s",
				err,
				block.ChainID,
				block.BlockNumber,
				hex.EncodeToString(block.BlockHash[:]),
			)
		}

		if raw == nil {
			log.Error().
				Err(err).
				Uint64("block", block.BlockNumber).
				Uint64("chain", block.ChainID).
				Msg("receipt not found")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		r := evmtypes.Receipt{}

		err := json.Unmarshal(raw, &r)
		if err != nil {
			return nil, fmt.Errorf("decode receipt: %w", err)
		}

		r.Raw = raw

		receipts = append(receipts, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	return receipts, nil
}

func (sa *MultichainStateAccessSQL) Close() {
	for _, db := range sa.stateAccessDB {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close db")
		}
	}
}

// MidnightBlockByHash reads a Midnight block from the chain-specific SQLite DB.
func (sa *MultichainStateAccessSQL) MidnightBlockByHash(
	ctx context.Context,
	block apptypes.ExternalBlock,
) (*MidnightBlock, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	db, ok := sa.stateAccessDB[apptypes.ChainType(block.ChainID)]
	if !ok {
		return nil, fmt.Errorf(
			"%w, no DB for chainID %d",
			library.ErrUnknownChain, block.ChainID,
		)
	}

	const query = `
		SELECT hash, number, parent_hash, timestamp, raw_block
		FROM blocks
		WHERE hash   = ?
		  AND number = ?
	`

	var mb MidnightBlock

	err := db.QueryRowContext(
		ctx, query, block.BlockHash[:], block.BlockNumber,
	).Scan(&mb.Hash, &mb.Number, &mb.ParentHash, &mb.Timestamp, &mb.RawBlock)
	if err != nil {
		return nil, fmt.Errorf(
			"read midnight block: %w, chainID %d, block %d, hash %s",
			err, block.ChainID, block.BlockNumber,
			hex.EncodeToString(block.BlockHash[:]),
		)
	}

	return &mb, nil
}

// MidnightContractActions reads contract actions for a Midnight block.
func (sa *MultichainStateAccessSQL) MidnightContractActions(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]MidnightContractAction, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	db, ok := sa.stateAccessDB[apptypes.ChainType(block.ChainID)]
	if !ok {
		return nil, fmt.Errorf(
			"%w, no DB for chainID %d",
			library.ErrUnknownChain, block.ChainID,
		)
	}

	const query = `
		SELECT contract_addr, action_type, entry_point, state, raw_action
		FROM contract_actions
		WHERE block_hash = ?
		  AND block_number = ?
		ORDER BY id
	`

	rows, err := db.QueryContext(
		ctx, query, block.BlockHash[:], block.BlockNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("query midnight contract actions: %w", err)
	}
	defer rows.Close()

	var actions []MidnightContractAction

	for rows.Next() {
		var a MidnightContractAction

		var entryPoint sql.NullString

		if err := rows.Scan(
			&a.ContractAddr, &a.ActionType, &entryPoint, &a.State, &a.RawAction,
		); err != nil {
			return nil, fmt.Errorf("scan contract action: %w", err)
		}

		if entryPoint.Valid {
			a.EntryPoint = entryPoint.String
		}

		actions = append(actions, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	return actions, nil
}

// SolanaBlock is not supported for the SQLite-backed multichain store.
func (*MultichainStateAccessSQL) SolanaBlock(
	_ context.Context,
	block apptypes.ExternalBlock,
) (*client.Block, error) {
	return nil, fmt.Errorf(
		"%w, solana not available in sqlite backend for chainID %d",
		library.ErrUnknownChain,
		block.ChainID,
	)
}

// openSQLite opens a SQLite database with the given mode ("ro" for read-only, "rwc" for read/write).
// It mirrors the retry logic used by the MDBX opener so tests and production paths behave similarly.
func openSQLite(ctx context.Context, dbPath, mode string) (*sql.DB, error) {
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
			log.Error().Err(err).Msg("Failed to open SQLite DB")
			time.Sleep(time.Second)

			if maxTries == 0 {
				return nil, err
			}

			maxTries--

			continue
		}

		if pingErr := db.PingContext(ctx); pingErr != nil {
			log.Error().Err(pingErr).Str("dsn", dsn).Str("path", dbPath).Msg("SQLite ping failed")

			if closeErr := db.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg("Failed to close DB")
			}

			time.Sleep(time.Second)

			if maxTries == 0 {
				return nil, pingErr
			}

			maxTries--

			continue
		}

		log.Info().Str("path", dbPath).Msg("SQLite DB opened")

		if mode == "ro" {
			if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON;"); err != nil {
				log.Warn().Err(err).Msg("Unable to enforce query_only; continue anyway")
			}

			// Disable auto-checkpoint on read-only connections.
			// Without this the reader may trigger a FULL checkpoint that
			// blocks on the writer, causing 80%+ CPU in cgocall wait.
			// The writer (pelacli/multichain oracle) handles checkpointing.
			if _, err := db.ExecContext(ctx, "PRAGMA wal_autocheckpoint = 0;"); err != nil {
				log.Warn().Err(err).Msg("Unable to disable wal_autocheckpoint; continue anyway")
			}
		}

		return db, nil
	}
}
