package gosdk

import (
	"context"
	"encoding/hex"
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
	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const errSQLiteRowNotFound sdkerrors.SDKError = "sqlite row not found"

type MultichainStateAccessSQL struct {
	mu            sync.RWMutex
	stateAccessDB map[apptypes.ChainType]*multichainSQLiteReader
}

// NewMultichainStateAccessSQL opens read-only SQLite databases for each chain
// in the config and returns a ready-to-use accessor.
func NewMultichainStateAccessSQL(
	ctx context.Context,
	cfg MultichainConfig,
) (*MultichainStateAccessSQL, error) {
	stateAccessDBs := make(map[apptypes.ChainType]*multichainSQLiteReader)

	for chainID, path := range cfg {
		dbPath := filepath.Join(filepath.Clean(path), "sqlite")

		db, err := openMultichainSQLiteReader(ctx, dbPath)
		if err != nil {
			// Close any already-opened DBs on failure.
			for _, opened := range stateAccessDBs {
				if closeErr := opened.Close(); closeErr != nil {
					log.Ctx(ctx).Warn().
						Err(closeErr).
						Msg("close multichain sqlite db after open failure")
				}
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

	i := 0
	for {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Millisecond * 100):
			}
		}

		i++

		rawBlock, num, found, err := db.readEVMBlockRow(ctx, block)
		if err != nil {
			log.Error().
				Err(err).
				Uint64("block", block.BlockNumber).
				Uint64("chain", block.ChainID).
				Msg("block not found")

			return nil, fmt.Errorf(
				"failed to read eth block: %w, chainID %d, block number %d, block hash %s",
				err,
				block.ChainID,
				block.BlockNumber,
				hex.EncodeToString(block.BlockHash[:]),
			)
		}

		if !found {
			log.Error().
				Err(errSQLiteRowNotFound).
				Uint64("block", block.BlockNumber).
				Uint64("chain", block.ChainID).
				Msg("block not found")

			continue
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

	receipts, err := db.readEVMReceipts(ctx, block)
	if err != nil {
		return nil, err
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

	mb, err := db.readMidnightBlock(ctx, block)
	if err != nil {
		return nil, fmt.Errorf(
			"read midnight block: %w, chainID %d, block %d, hash %s",
			err, block.ChainID, block.BlockNumber,
			hex.EncodeToString(block.BlockHash[:]),
		)
	}

	return mb, nil
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

	actions, err := db.readMidnightActions(ctx, block)
	if err != nil {
		return nil, err
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
