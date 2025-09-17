package gosdk

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	geth "github.com/ethereum/go-ethereum/ethclient"
	"github.com/goccy/go-json"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type FixtureWriter[T any] struct {
	DB       kv.RwDB
	ChainID  apptypes.ChainType
	Iter     Iterator[T]
	Interval time.Duration
}

func (fw *FixtureWriter[T]) Run(ctx context.Context) error {
	t := time.NewTicker(fw.Interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			item, err := fw.Iter.Next(ctx)
			if errors.Is(err, io.EOF) {
				return nil
			}

			if err != nil {
				return err
			}

			if err := fw.writeOne(ctx, item); err != nil {
				return err
			}
		}
	}
}

func (fw *FixtureWriter[T]) writeOne(ctx context.Context, item T) error {
	switch v := any(item).(type) {
	case *gethtypes.Block:
		return fw.putEthBlock(ctx, v)
	case gethtypes.Block:
		return fw.putEthBlock(ctx, &v)
	case *gosdk.EthereumBlock:
		return fw.putEthereumBlock(ctx, v)
	case gosdk.EthereumBlock:
		return fw.putEthereumBlock(ctx, &v)
	case []gethtypes.Receipt:
		return fw.putEthReceipts(ctx, v)
	case *client.Block:
		return fw.putSolBlock(ctx, v)
	case client.Block:
		return fw.putSolBlock(ctx, &v)
	default:
		return fmt.Errorf("%w: %T", gosdk.ErrUnsupportedFixture, item)
	}
}

// --- Writers fixed to match the readers ---

func (fw *FixtureWriter[T]) putEthBlock(ctx context.Context, b *gethtypes.Block) error {
	num := b.NumberU64()
	h := b.Hash()
	k := gosdk.EthBlockKey(num, h)

	ethBlock := gosdk.NewEthereumBlock(b)

	enc, err := json.Marshal(ethBlock)
	if err != nil {
		return err
	}

	return fw.DB.Update(ctx, func(tx kv.RwTx) error {
		return tx.Put(gosdk.EthBlocks, k, enc)
	})
}

func (fw *FixtureWriter[T]) putEthereumBlock(ctx context.Context, b *gosdk.EthereumBlock) error {
	num := b.Header.Number.Uint64()
	h := b.Header.Hash()
	k := gosdk.EthBlockKey(num, h)

	enc, err := json.Marshal(b)
	if err != nil {
		return err
	}

	return fw.DB.Update(ctx, func(tx kv.RwTx) error {
		return tx.Put(gosdk.EthBlocks, k, enc)
	})
}

func (fw *FixtureWriter[T]) putEthReceipts(ctx context.Context, recs []gethtypes.Receipt) error {
	if len(recs) == 0 {
		return nil
	}

	num := recs[0].BlockNumber.Uint64()
	h := recs[0].BlockHash

	return fw.DB.Update(ctx, func(tx kv.RwTx) error {
		for i, rc := range recs {
			k := gosdk.EthReceiptKey(num, h, uint32(i))

			enc, err := json.Marshal(rc)
			if err != nil {
				return err
			}

			if err = tx.Put(gosdk.EthReceipts, k, enc); err != nil {
				return err
			}
		}

		return nil
	})
}

func (fw *FixtureWriter[T]) putSolBlock(ctx context.Context, b *client.Block) error {
	// Make sure this numeric matches what you use as ExternalBlock.BlockNumber on read
	// (Slot is a good choice; keep it consistent end-to-end).
	k := gosdk.SolBlockKey(uint64(*b.BlockHeight))

	enc, err := json.Marshal(b)
	if err != nil {
		return err
	}

	return fw.DB.Update(ctx, func(tx kv.RwTx) error {
		return tx.Put(gosdk.SolanaBlocks, k, enc)
	})
}

type Iterator[T any] interface {
	Next(ctx context.Context) (T, error) // io.EOF when done
	Close() error
}

type EthBlockRPCIterator struct {
	cl       *geth.Client
	cur, end uint64
	closed   bool
}

func NewEthBlockRPCIterator(cl *geth.Client, start, end uint64) *EthBlockRPCIterator {
	return &EthBlockRPCIterator{cl: cl, cur: start, end: end}
}

func (it *EthBlockRPCIterator) Next(ctx context.Context) (*gethtypes.Block, error) {
	if it.closed {
		return nil, io.EOF
	}

	if it.cur > it.end {
		return nil, io.EOF
	}

	b, err := it.cl.BlockByNumber(ctx, new(big.Int).SetUint64(it.cur))
	if err != nil {
		return nil, err
	}

	it.cur++

	return b, nil
}

//nolint:unparam // for closer interface
func (it *EthBlockRPCIterator) Close() error {
	it.closed = true

	return nil
}

type EthReceiptsRPCIterator struct {
	cl       *geth.Client
	cur, end uint64
	closed   bool
}

func NewEthReceiptsRPCIterator(cl *geth.Client, start, end uint64) *EthReceiptsRPCIterator {
	return &EthReceiptsRPCIterator{cl: cl, cur: start, end: end}
}

func (it *EthReceiptsRPCIterator) Next(ctx context.Context) ([]*gethtypes.Receipt, error) {
	if it.closed {
		return nil, io.EOF
	}

	if it.cur > it.end {
		return nil, io.EOF
	}

	blk, err := it.cl.BlockByNumber(ctx, new(big.Int).SetUint64(it.cur))
	if err != nil {
		return nil, err
	}

	recs := make([]*gethtypes.Receipt, 0, blk.Transactions().Len())
	for _, tx := range blk.Transactions() {
		rc, err := it.cl.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			return nil, err
		}

		recs = append(recs, rc)
	}

	it.cur++

	return recs, nil
}

//nolint:unparam // for closer interface
func (it *EthReceiptsRPCIterator) Close() error {
	it.closed = true

	return nil
}

type SolBlockRPCIterator struct {
	cl       *client.Client
	cur, end uint64
	closed   bool
}

func NewSolBlockRPCIterator(cl *client.Client, start, end uint64) *SolBlockRPCIterator {
	return &SolBlockRPCIterator{cl: cl, cur: start, end: end}
}

func (it *SolBlockRPCIterator) Next(ctx context.Context) (*client.Block, error) {
	if it.closed {
		return nil, io.EOF
	}

	if it.cur > it.end {
		return nil, io.EOF
	}

	blk, err := it.cl.GetBlock(ctx, it.cur)
	if err != nil {
		return nil, err
	}

	it.cur++

	return blk, nil
}

//nolint:unparam // for closer interface
func (it *SolBlockRPCIterator) Close() error {
	it.closed = true

	return nil
}

// EthBlockFileIterator: each line is hex-encoded RLP(Block) or raw RLP if you wire different reader
type EthBlockFileIterator struct {
	f   *os.File
	sc  *bufio.Scanner
	dec func([]byte) (*gosdk.EthereumBlock, error)
}

func NewEthBlockFileIterator(path string) (*EthBlockFileIterator, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<27) // up to ~128MB lines

	return &EthBlockFileIterator{
		f:  f,
		sc: sc,
		dec: func(b []byte) (*gosdk.EthereumBlock, error) {
			// assume b is raw JSON. If you store hex, decode hex first.
			var blk gosdk.EthereumBlock
			if err := json.Unmarshal(b, &blk); err != nil {
				return nil, err
			}

			return &blk, nil
		},
	}, nil
}

func (it *EthBlockFileIterator) Next(_ context.Context) (*gosdk.EthereumBlock, error) {
	if !it.sc.Scan() {
		if err := it.sc.Err(); err != nil {
			return nil, err
		}

		return nil, io.EOF
	}

	line := it.sc.Bytes()

	return it.dec(bytes.Clone(line))
}

func (it *EthBlockFileIterator) Close() error {
	return it.f.Close()
}

// EthReceiptsFileIterator: per line, JSON array of hex-rlp receipts or raw RLP blobs
type EthReceiptsFileIterator struct {
	f  *os.File
	sc *bufio.Scanner
}

func NewEthReceiptsFileIterator(path string) (*EthReceiptsFileIterator, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<27)

	return &EthReceiptsFileIterator{f: f, sc: sc}, nil
}

func (it *EthReceiptsFileIterator) Next(_ context.Context) ([]gethtypes.Receipt, error) {
	if !it.sc.Scan() {
		if err := it.sc.Err(); err != nil {
			return nil, err
		}

		return nil, io.EOF
	}
	// Expect: JSON array of base64/hex RLP receipts or raw JSON receipts you control.
	// For simplicity, assume JSON array of raw RLP blobs (base64) – adjust to your file.
	var entries []gethtypes.Receipt
	if err := json.Unmarshal(it.sc.Bytes(), &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

func (it *EthReceiptsFileIterator) Close() error {
	return it.f.Close()
}

// SolBlockFileIterator: each line JSON(client.Block)
type SolBlockFileIterator struct {
	f  *os.File
	sc *bufio.Scanner
}

func NewSolBlockFileIterator(path string) (*SolBlockFileIterator, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<27)

	return &SolBlockFileIterator{f: f, sc: sc}, nil
}

func (it *SolBlockFileIterator) Next(_ context.Context) (*client.Block, error) {
	if !it.sc.Scan() {
		if err := it.sc.Err(); err != nil {
			return nil, err
		}

		return nil, io.EOF
	}

	var blk client.Block
	if err := json.Unmarshal(it.sc.Bytes(), &blk); err != nil {
		return nil, err
	}

	return &blk, nil
}

func (it *SolBlockFileIterator) Close() error {
	return it.f.Close()
}
