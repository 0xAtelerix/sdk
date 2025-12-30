package gosdk

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/evmtypes"
	evmtypes2 "github.com/0xAtelerix/sdk/gosdk/evmtypes/custom_fields"
)

var (
	ErrNoAccessorForChain   = errors.New("no accessor for chain")
	ErrAccessorTypeMismatch = errors.New("accessor type mismatch for chain")
)

type EVMRouter struct {
	byChain map[apptypes.ChainType]any // holds MultichainStateAccessor[T evmtypes.CustomFields] values
}

func NewEVMRouter() *EVMRouter {
	return &EVMRouter{byChain: make(map[apptypes.ChainType]any)}
}

func Register[T evmtypes2.CustomFields](
	r *EVMRouter,
	chain apptypes.ChainType,
	acc MultichainStateAccessor[T],
) {
	r.byChain[chain] = acc
}

func EVMBlockAs[T evmtypes2.CustomFields](
	ctx context.Context,
	r *EVMRouter,
	blk apptypes.ExternalBlock,
) (*evmtypes.Block[T], error) {
	accAny, ok := r.byChain[apptypes.ChainType(blk.ChainID)]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrNoAccessorForChain, blk.ChainID)
	}

	acc, ok := accAny.(MultichainStateAccessor[T])
	if !ok {
		return nil, fmt.Errorf("%w: chain %d", ErrAccessorTypeMismatch, blk.ChainID)
	}

	return acc.EVMBlock(ctx, blk)
}

func EVMReceiptsAs[T evmtypes2.CustomFields](
	ctx context.Context,
	r *EVMRouter,
	blk apptypes.ExternalBlock,
) ([]evmtypes.Receipt[T], error) {
	accAny, ok := r.byChain[apptypes.ChainType(blk.ChainID)]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrNoAccessorForChain, blk.ChainID)
	}

	acc, ok := accAny.(MultichainStateAccessor[T])
	if !ok {
		return nil, fmt.Errorf("%w: chain %d", ErrAccessorTypeMismatch, blk.ChainID)
	}

	return acc.EVMReceipts(ctx, blk)
}

func EVMBlock[T evmtypes2.CustomFields](
	ctx context.Context,
	r *EVMRouter,
	blk apptypes.ExternalBlock,
) (*evmtypes.Block[T], error) {
	accAny, ok := r.byChain[apptypes.ChainType(blk.ChainID)]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrNoAccessorForChain, blk.ChainID)
	}

	acc, ok := accAny.(MultichainStateAccessor[T])
	if !ok {
		return nil, fmt.Errorf("%w: chain %d", ErrAccessorTypeMismatch, blk.ChainID)
	}

	return acc.EVMBlock(ctx, blk)
}
