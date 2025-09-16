package gosdk

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/mr-tron/base58"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
)

const (
	ChainIDBucket = "chainid"
	EthBlocks     = "blocks"
	SolanaBlocks  = "solana_blocks"
)

func EvmTables() kv.TableCfg {
	return kv.TableCfg{
		ChainIDBucket:         {},
		EthBlocks:             {},
		receipt.ReceiptBucket: {},
	}
}

func SolanaTables() kv.TableCfg {
	return kv.TableCfg{
		ChainIDBucket:         {},
		SolanaBlocks:          {},
		receipt.ReceiptBucket: {},
	}
}

type MultichainStateAccess struct {
	stateAccessDB map[ChainType]kv.RoDB
	mu            sync.RWMutex
}

type MultichainConfig map[ChainType]string // chainID, chainDBpath

func NewMultichainStateAccess(cfg MultichainConfig) (*MultichainStateAccess, error) {
	multichainStateDB := MultichainStateAccess{
		stateAccessDB: make(map[ChainType]kv.RoDB, len(cfg)),
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
				Uint64("chainID", uint64(chainID)).
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

func (sa *MultichainStateAccess) EthBlock(
	ctx context.Context,
	block apptypes.ExternalBlock,
) (*EthereumBlock, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	if _, ok := sa.stateAccessDB[ChainType(block.ChainID)]; !ok {
		return nil, fmt.Errorf("%w, no DB for chainID, %v", ErrUnknownChain, block.ChainID)
	}

	key := EthBlockKey(block.BlockNumber, block.BlockHash)

	ethBlock := EthereumBlock{}

	err := sa.stateAccessDB[ChainType(block.ChainID)].View(ctx, func(tx kv.Tx) error {
		v, err := tx.GetOne(EthBlocks, key)
		if err != nil {
			return err
		}

		return json.Unmarshal(v, &ethBlock)
	})
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read eth block: %w, chainID %d, block number %d, block hash %s",
			err,
			block.ChainID,
			block.BlockNumber,
			hex.EncodeToString(block.BlockHash[:]),
		)
	}

	ethBlockHash := ethBlock.Header.Hash()
	if ethBlockHash != block.BlockHash {
		return nil, fmt.Errorf(
			"%w, chainID %d; got block number %d, hash %s; expected block number %d, hash %s",
			ErrWrongBlock,
			block.ChainID,
			ethBlock.Header.Number.Uint64(),
			hex.EncodeToString(ethBlockHash[:]),
			block.BlockNumber,
			hex.EncodeToString(block.BlockHash[:]),
		)
	}

	return &ethBlock, nil
}

func (sa *MultichainStateAccess) EthReceipts(
	ctx context.Context,
	block apptypes.ExternalBlock,
) ([]gethtypes.Receipt, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	if _, ok := sa.stateAccessDB[ChainType(block.ChainID)]; !ok {
		return nil, fmt.Errorf("%w, no DB for chainID, %v", ErrUnknownChain, block.ChainID)
	}

	key := EthReceiptKey(block.BlockNumber, block.BlockHash)

	var blockReceipts []gethtypes.Receipt

	err := sa.stateAccessDB[ChainType(block.ChainID)].View(ctx, func(tx kv.Tx) error {
		return tx.ForPrefix(receipt.ReceiptBucket, key, func(_, v []byte) error {
			r := gethtypes.Receipt{}

			dbErr := json.Unmarshal(v, &r)
			if dbErr != nil {
				return dbErr
			}

			blockReceipts = append(blockReceipts, r)

			return nil
		})
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
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	db, ok := sa.stateAccessDB[ChainType(block.ChainID)]
	if !ok {
		return nil, fmt.Errorf("%w, no DB for chainID, %v", ErrUnknownChain, block.ChainID)
	}

	key := SolBlockKey(block.BlockNumber)

	var solBlock client.Block

	err := db.View(ctx, func(tx kv.Tx) error {
		v, err := tx.GetOne(SolanaBlocks, key)
		if err != nil {
			return err
		}

		return json.Unmarshal(v, &solBlock)
	})
	if err != nil {
		return nil, err
	}

	got, err := base58.Decode(solBlock.Blockhash)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(block.BlockHash[:], got) {
		return nil, fmt.Errorf(
			"%w: expected %s, got %s",
			ErrWrongBlock,
			string(block.BlockHash[:]),
			solBlock.Blockhash,
		)
	}

	return &solBlock, nil
}

// ViewDB may be not deterministic because on diffenet validators you may have different tip.
// You can rely on received finalized external blocks that you have received from consensus.
func (sa *MultichainStateAccess) ViewDB(
	ctx context.Context,
	chainID ChainType,
	fn func(tx kv.Tx) error,
) error {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	db, ok := sa.stateAccessDB[chainID]
	if !ok {
		return fmt.Errorf("%w, no DB for chainID, %d", ErrUnknownChain, chainID)
	}

	return db.View(ctx, fn)
}

func (sa *MultichainStateAccess) Close() {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	for _, db := range sa.stateAccessDB {
		db.Close()
	}
}

// EthBlocks: [8 bytes blockNumber][32 bytes blockHash]  => total 40 bytes
func EthBlockKey(num uint64, hash [32]byte) []byte {
	key := make([]byte, 8+32)
	binary.BigEndian.PutUint64(key[:8], num)
	copy(key[8:], hash[:])

	return key
}

// EthReceipts: [8 bytes blockNumber][32 bytes blockHash][4 bytes txIndex] => 44 bytes
func EthReceiptKey(blockNumber uint64, blockHash [32]byte, txIndex ...uint32) []byte {
	keyLength := 44
	if txIndex == nil {
		keyLength = 40
	}

	key := make([]byte, keyLength)
	binary.BigEndian.PutUint64(key[:8], blockNumber)
	copy(key[8:40], blockHash[:])

	if len(txIndex) > 0 {
		binary.BigEndian.PutUint32(key[40:], txIndex[0])
	}

	return key
}

// SolanaBlocks: [8 bytes blockNumber]  (you read by ExternalBlock.BlockNumber)
// If you use Slot as BlockNumber, write Slot here.
func SolBlockKey(num uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, num)

	return key
}

type EthereumBlock struct {
	Header gethtypes.Header
	Body   gethtypes.Body
}

func NewEthereumBlock(b *gethtypes.Block) *EthereumBlock {
	return &EthereumBlock{
		Header: *b.Header(),
		Body:   *b.Body(),
	}
}
