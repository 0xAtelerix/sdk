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
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
)

func NewMultichainStateAccessSQLDB(
	ctx context.Context,
	cfg MultichainConfig,
) (map[apptypes.ChainType]*sql.DB, error) {
	stateAccessDBs := make(map[apptypes.ChainType]*sql.DB)

	for chainID, path := range cfg {
		dbPath := filepath.Join(filepath.Clean(path), "sqlite")

		db, err := openSQLite(ctx, dbPath, "ro")
		if err != nil {
			return nil, err
		}

		stateAccessDBs[chainID] = db
	}

	return stateAccessDBs, nil
}

type MultichainStateAccessSQL struct {
	mu            sync.RWMutex
	stateAccessDB map[apptypes.ChainType]*sql.DB
}

func NewMultichainStateAccessSQL(dbMap map[apptypes.ChainType]*sql.DB) *MultichainStateAccessSQL {
	return &MultichainStateAccessSQL{
		stateAccessDB: dbMap,
	}
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
			log.Error().Err(err).Msg("block not found")

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
			log.Error().Err(err).Msg("block not found")

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
			return nil, fmt.Errorf("scan receipt raw: %w", err)
		}

		if raw == nil {
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
		}

		return db, nil
	}
}
