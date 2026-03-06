package gosdk

import (
	"context"

	"github.com/blocto/solana-go-sdk/client"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
)

// MidnightBlock represents a raw Midnight block read from the chain DB.
type MidnightBlock struct {
	Hash       []byte // 32-byte block hash
	Number     uint64
	ParentHash []byte // 32-byte parent block hash
	Timestamp  int64
	RawBlock   []byte // full JSON block from the indexer
}

// MidnightContractAction represents a contract action read from the chain DB.
type MidnightContractAction struct {
	ContractAddr []byte
	ActionType   string // "call", "deploy", or "update"
	EntryPoint   string
	State        []byte // contract state after action
	RawAction    []byte
}

// MultichainStateAccessor is an interface for accessing multichain state.
// Both MDBX and SQLite implementations satisfy this interface.
type MultichainStateAccessor interface {
	EVMBlock(ctx context.Context, block apptypes.ExternalBlock) (*evmtypes.Block, error)
	EVMReceipts(ctx context.Context, block apptypes.ExternalBlock) ([]evmtypes.Receipt, error)
	SolanaBlock(ctx context.Context, block apptypes.ExternalBlock) (*client.Block, error)
	MidnightBlockByHash(ctx context.Context, block apptypes.ExternalBlock) (*MidnightBlock, error)
	MidnightContractActions(
		ctx context.Context,
		block apptypes.ExternalBlock,
	) ([]MidnightContractAction, error)
	Close()
}

// Ensure both implementations satisfy the interface.
var (
	_ MultichainStateAccessor = (*MultichainStateAccess)(nil)
	_ MultichainStateAccessor = (*MultichainStateAccessSQL)(nil)
)
