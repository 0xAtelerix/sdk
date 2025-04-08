package gosdk

import (
	"bytes"
	"context"
	"encoding/binary"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"

	"fmt"
	"github.com/0xAtelerix/sdk/gosdk/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog/log"
)

const (
	ChainIDBucket = "chainid"
	EthBlocks     = "blocks"
	EthReceipts   = "receipts"
)

var EvmTables = kv.TableCfg{
	ChainIDBucket: {},
	EthBlocks:     {},
	EthReceipts:   {},
}

// todo add tests
func NewMultichainStateAccess(cfg map[uint32]string) (*MultichainStateAccess, error) {
	multichainStateDB := MultichainStateAccess{
		stateAccessDB: make(map[uint32]kv.RoDB, len(cfg)),
	}
	for chainID, path := range cfg {
		stateAccessDB, err := mdbx.NewMDBX(mdbxlog.New()).
			Path(path).WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
			return EvmTables
		}).Readonly().Open()
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize MDBX")
			return nil, fmt.Errorf("failed to initialize %v db: %w", chainID, err)
		}
		multichainStateDB.stateAccessDB[chainID] = stateAccessDB
	}
	return &multichainStateDB, nil
}

type MultichainStateAccess struct {
	stateAccessDB map[uint32]kv.RoDB
}

func (sa *MultichainStateAccess) EthBlock(block types.ExternalBlock) (*gethtypes.Block, error) {
	if _, ok := sa.stateAccessDB[uint32(block.ChainID)]; !ok {
		return nil, fmt.Errorf("failed to find blockchain db %v", block.ChainID)
	}
	key := make([]byte, 40)
	binary.BigEndian.PutUint64(key[:8], block.BlockNumber)
	copy(key[8:], block.BlockHash[:])
	ethBlock := gethtypes.Block{}
	err := sa.stateAccessDB[uint32(block.ChainID)].View(context.TODO(), func(tx kv.Tx) error {
		v, err := tx.GetOne(EthBlocks, key)
		if err != nil {
			return err
		}
		//todo fixme rlp or faster encoding
		return rlp.DecodeBytes(v, &ethBlock)
	})
	if err != nil {
		return nil, err
	}
	//todo verify block hash

	return &ethBlock, nil
}

func (sa *MultichainStateAccess) EthReceipts(block types.ExternalBlock) ([]*gethtypes.Receipt, error) {
	if _, ok := sa.stateAccessDB[uint32(block.ChainID)]; !ok {
		return nil, fmt.Errorf("failed to find blockchain db %v", block.ChainID)
	}

	key := make([]byte, 72)
	binary.BigEndian.PutUint64(key[:8], block.BlockNumber)
	copy(key[8:8+32], block.BlockHash[:])
	blockReceipts := []*gethtypes.Receipt{}
	err := sa.stateAccessDB[uint32(block.ChainID)].View(context.TODO(), func(tx kv.Tx) error {
		c, err := tx.Cursor(EthReceipts)
		if err != nil {
			return err
		}
		k, v, err := c.Seek(key)
		for ; err == nil && len(k) == 44 && bytes.Equal(key[8:8+32], k[8:8+32]); k, v, err = c.Next() {
			r := gethtypes.Receipt{}
			//todo fixme rlp or faster encoding
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

	//todo verify receipt root

	return blockReceipts, nil
}
