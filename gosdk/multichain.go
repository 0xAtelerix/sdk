package gosdk

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	ChainIDBucket = "chainid"
	EthBlocks     = "blocks"
	EthReceipts   = "receipts"
	SolanaBlocks  = "solana_blocks"
)

func EvmTables() kv.TableCfg {
	return kv.TableCfg{
		ChainIDBucket: {},
		EthBlocks:     {},
		EthReceipts:   {},
	}
}

func SolanaTables() kv.TableCfg {
	return kv.TableCfg{
		ChainIDBucket: {},
		SolanaBlocks:  {},
	}
}

type MultichainStateAccess struct {
	stateAccessDB map[uint32]kv.RoDB
}

func NewMultichainStateAccess(cfg map[uint32]string) (*MultichainStateAccess, error) {
	multichainStateDB := MultichainStateAccess{
		stateAccessDB: make(map[uint32]kv.RoDB, len(cfg)),
	}
	for chainID, path := range cfg {
		var tableCfg kv.TableCfg

		switch {
		case IsEvmChain(chainID):
			tableCfg = EvmTables()
		case IsSolanaChain(chainID):
			tableCfg = SolanaTables()
		default:
			log.Warn().
				Uint32("chainID", chainID).
				Msg("unknown chain type, using EVM tables by default")

			tableCfg = EvmTables()
		}

		stateAccessDB, err := mdbx.NewMDBX(mdbxlog.New()).
			Path(path).WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return tableCfg
		}).Readonly().Open()
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize MDBX")

			return nil, fmt.Errorf("failed to initialize %v db: %w", chainID, err)
		}

		multichainStateDB.stateAccessDB[chainID] = stateAccessDB
	}

	return &multichainStateDB, nil
}

//nolint:unparam // interface implementation
func (sa *MultichainStateAccess) Close() error {
	for _, db := range sa.stateAccessDB {
		db.Close()
	}

	return nil
}

func (sa *MultichainStateAccess) EthBlock(
	ctx context.Context,
	block apptypes.ExternalBlock,
) (*gethtypes.Block, error) {
	if _, ok := sa.stateAccessDB[uint32(block.ChainID)]; !ok {
		return nil, fmt.Errorf("%w %v", ErrUnknownChain, block.ChainID)
	}

	key := make([]byte, 40)
	binary.BigEndian.PutUint64(key[:8], block.BlockNumber)
	copy(key[8:], block.BlockHash[:])

	ethBlock := gethtypes.Block{}

	err := sa.stateAccessDB[uint32(block.ChainID)].View(ctx, func(tx kv.Tx) error {
		v, err := tx.GetOne(EthBlocks, key)
		if err != nil {
			return err
		}
		// todo fixme rlp or faster encoding
		return rlp.DecodeBytes(v, &ethBlock)
	})
	if err != nil {
		return nil, err
	}
	// todo verify block hash

	return &ethBlock, nil
}

func (sa *MultichainStateAccess) EthReceipts(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]*gethtypes.Receipt, error) {
	if _, ok := sa.stateAccessDB[uint32(block.ChainID)]; !ok {
		return nil, fmt.Errorf("%w %v", ErrUnknownChain, block.ChainID)
	}

	key := make([]byte, 44)
	binary.BigEndian.PutUint64(key[:8], block.BlockNumber)
	copy(key[8:8+32], block.BlockHash[:])

	var blockReceipts []*gethtypes.Receipt

	err := sa.stateAccessDB[uint32(block.ChainID)].View(ctx, func(tx kv.Tx) error {
		c, err := tx.Cursor(EthReceipts)
		if err != nil {
			return err
		}

		k, v, err := c.Seek(key)
		for ; err == nil && len(k) == 44 && bytes.Equal(key[8:8+32], k[8:8+32]); k, v, err = c.Next() {
			r := gethtypes.Receipt{}
			// todo fixme rlp or faster encoding
			err = rlp.DecodeBytes(v, &r)
			if err != nil {
				return err
			}

			blockReceipts = append(blockReceipts, &r)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// todo verify receipt root

	return blockReceipts, nil
}

func (sa *MultichainStateAccess) SolanaBlock(
	ctx context.Context,
	block apptypes.ExternalBlock,
) (*client.Block, error) {
	db, ok := sa.stateAccessDB[uint32(block.ChainID)]
	if !ok {
		return nil, fmt.Errorf("%w %v", ErrUnknownChain, block.ChainID)
	}

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, block.BlockNumber)

	var solBlock client.Block

	err := db.View(ctx, func(tx kv.Tx) error {
		v, err := tx.GetOne(SolanaBlocks, key)
		if err != nil {
			return err
		}

		return cbor.Unmarshal(v, &solBlock)
	})
	if err != nil {
		return nil, err
	}

	////  todo ✅ Проверка целостности по хешу
	//if solBlock.Blockhash != "" && block.BlockHash.String() != solBlock.Blockhash {
	//	return nil, fmt.Errorf("block hash mismatch: expected %s, got %s", block.BlockHash.String(), solBlock.Blockhash)
	//}

	return &solBlock, nil
}

// ViewDB may be not deterministic because on diffenet validators you may have different tip.
// You can rely on received finalized external blocks that you have received from consensus.
func (sa *MultichainStateAccess) ViewDB(
	ctx context.Context,
	chainID uint32,
	fn func(tx kv.Tx) error,
) error {
	db, ok := sa.stateAccessDB[chainID]
	if !ok {
		return fmt.Errorf("%w %d", ErrUnknownChain, chainID)
	}

	return db.View(ctx, fn)
}
