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
)

func NewMultichainStateAccessSQLDB(
	ctx context.Context,
	cfg MultichainConfig,
) (map[apptypes.ChainType]*sql.DB, error) {
	stateAccessDBs := make(map[apptypes.ChainType]*sql.DB)

	for chainID, path := range cfg {
		dbPath := filepath.Join(filepath.Clean(path), "sqlite")

		dsn := fmt.Sprintf("file:%s?mode=ro&cache=shared&uri=true", dbPath)
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
				log.Error().Err(pingErr).Str("dsn", dsn).Msg("SQLite ping failed")

				err = db.Close()
				if err != nil {
					log.Error().Err(err).Msg("Failed to close DB")
				}

				time.Sleep(time.Second)

				if maxTries == 0 {
					return nil, pingErr
				}

				maxTries--

				continue
			}

			log.Info().
				Uint64("chainID", uint64(chainID)).
				Str("path", dbPath).
				Msg("SQLite read‑only DB opened")

			if _, err := db.ExecContext(ctx, "PRAGMA query_only = ON;"); err != nil {
				log.Warn().Err(err).Msg("Unable to enforce query_only; continue anyway")
			}

			stateAccessDBs[chainID] = db

			break
		}
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
		return nil, fmt.Errorf("%w, no DB for chainID %d", ErrUnknownChain, block.ChainID)
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
			log.Error().Msg(ErrWrongBlock.Error())

			return nil, fmt.Errorf(" %w block number mismatch: got %d, expected %d",
				ErrWrongBlock, num, block.BlockNumber)
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
				ErrHashMismatch,
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
		return nil, fmt.Errorf("%w, no DB for chainID %d", ErrUnknownChain, block.ChainID)
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

// SolanaBlock retrieves a Solana block from SQLite
// TODO: Implement when Solana support is needed
func (*MultichainStateAccessSQL) SolanaBlock(
	_ context.Context,
	_ apptypes.ExternalBlock,
) (*client.Block, error) {
	return nil, ErrNotImplemented
}
