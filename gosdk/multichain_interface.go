package gosdk

import (
	"context"
	"encoding/json"

	"github.com/blocto/solana-go-sdk/client"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	evmtypes2 "github.com/0xAtelerix/sdk/gosdk/evmtypes/custom_fields"
)

// MultichainStateAccessor is an interface for accessing multichain state
// Both MDBX and SQLite implementations satisfy this interface
type MultichainStateAccessor[T evmtypes2.CustomFields] interface {
	EVMBlock(ctx context.Context, block apptypes.ExternalBlock) (*evmtypes.Block[T], error)
	EVMReceipts(ctx context.Context, block apptypes.ExternalBlock) ([]evmtypes.Receipt[T], error)
	SolanaBlock(ctx context.Context, block apptypes.ExternalBlock) (*client.Block, error)
	Close()
}

// Ensure both implementations satisfy the interface
var (
	_ MultichainStateAccessor[json.RawMessage]               = (*MultichainStateAccess[json.RawMessage])(nil)
	_ MultichainStateAccessor[evmtypes.EthereumCustomFields] = (*MultichainStateAccess[evmtypes.EthereumCustomFields])(nil)

	_ MultichainStateAccessor[json.RawMessage]               = (*MultichainStateAccessSQL[json.RawMessage])(nil)
	_ MultichainStateAccessor[evmtypes.EthereumCustomFields] = (*MultichainStateAccessSQL[evmtypes.EthereumCustomFields])(nil)
)
