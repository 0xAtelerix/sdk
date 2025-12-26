package gosdk

import (
	"context"

	"github.com/blocto/solana-go-sdk/client"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
)

// MultichainStateAccessor is an interface for accessing multichain state
// Both MDBX and SQLite implementations satisfy this interface
type MultichainStateAccessor interface {
	EVMBlock(ctx context.Context, block apptypes.ExternalBlock) (*evmtypes.Block, error)
	EVMReceipts(ctx context.Context, block apptypes.ExternalBlock) ([]evmtypes.Receipt, error)
	SolanaBlock(ctx context.Context, block apptypes.ExternalBlock) (*client.Block, error)
	Close()
}

// Ensure both implementations satisfy the interface
var (
	_ MultichainStateAccessor = (*MultichainStateAccess)(nil)
	_ MultichainStateAccessor = (*MultichainStateAccessSQL)(nil)
)
