package rpc

import (
	"context"
	"encoding/binary"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// BlockMethods provides RPC methods to query blocks from the appchain.
type BlockMethods[
	appTx apptypes.AppTransaction[R],
	R apptypes.Receipt,
	Block apptypes.AppchainBlock,
] struct {
	appchainDB kv.RwDB
}

// NewBlockMethods creates a new BlockMethods instance.
func NewBlockMethods[
	appTx apptypes.AppTransaction[R],
	R apptypes.Receipt,
	Block apptypes.AppchainBlock,
](
	appchainDB kv.RwDB,
) *BlockMethods[appTx, R, Block] {
	return &BlockMethods[appTx, R, Block]{
		appchainDB: appchainDB,
	}
}

// GetBlock retrieves a single block by block number.
// Params: [blockNumber] - if blockNumber is omitted, returns the latest block
// Returns: Block object with all custom fields
func (m *BlockMethods[appTx, R, Block]) GetBlock(
	ctx context.Context,
	params []any,
) (any, error) {
	if len(params) > 1 {
		return nil, ErrWrongParamsCount
	}

	var (
		blockNumber uint64
		err         error
	)

	if len(params) == 0 {
		// No params provided - get the latest block
		err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
			blockNumber, _, err = gosdk.GetLastBlock(tx)

			return err
		})
		if err != nil {
			return nil, err
		}

		if blockNumber == 0 {
			return nil, ErrBlockNotFound
		}
	} else {
		// Block number provided
		blockNumber, err = parseBlockNumber(params[0])
		if err != nil {
			return nil, err
		}
	}

	var payload []byte

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		// Use the same key format as WriteBlock in appchain.go
		var number [8]byte
		binary.BigEndian.PutUint64(number[:], blockNumber)

		payload, err = tx.GetOne(gosdk.BlocksBucket, number[:])
		if err != nil {
			return err
		}

		if len(payload) == 0 {
			return ErrBlockNotFound
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Blocks are now stored in CBOR format (cbor.Marshal(block))
	var block Block
	if err := cbor.Unmarshal(payload, &block); err != nil {
		return nil, err
	}

	return block, nil
}

// AddBlockMethods registers all block query RPC methods on the server
func AddBlockMethods[
	appTx apptypes.AppTransaction[R],
	R apptypes.Receipt,
	Block apptypes.AppchainBlock,
](
	server *StandardRPCServer,
	appchainDB kv.RwDB,
) {
	methods := NewBlockMethods[appTx, R, Block](
		appchainDB,
	)

	server.AddMethod("getBlock", methods.GetBlock)
}
