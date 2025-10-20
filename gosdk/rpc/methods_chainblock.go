package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/chainblock"
)

type ChainBlockMethods[T any] struct {
	cb *chainblock.ChainBlock[T]
}

var (
	errChainBlockMethodsNotInitialized = errors.New("chain block methods not initialized")
	errUnsupportedPayload              = errors.New("unsupported block payload type")
)

func NewChainBlockMethods[T any](
	chainType apptypes.ChainType,
	target *T,
) (*ChainBlockMethods[T], error) {
	cb, err := chainblock.NewChainBlock(chainType, target)
	if err != nil {
		return nil, err
	}

	return &ChainBlockMethods[T]{cb: cb}, nil
}

func (m *ChainBlockMethods[T]) GetChainBlock(
	_ context.Context,
	params []any,
) (any, error) {
	if m == nil {
		return nil, errChainBlockMethodsNotInitialized
	}

	if len(params) != 2 {
		return nil, ErrWrongParamsCount
	}

	chainType, err := parseChainType(params[0])
	if err != nil {
		return nil, err
	}

	target, err := decodeTarget[T](params[1])
	if err != nil {
		return nil, err
	}

	fv, err := chainblock.GetFieldsValues(chainType, target)
	if err != nil {
		return nil, err
	}

	return fv, nil
}

func AddChainBlockMethods[T any](
	server *StandardRPCServer,
	chainType apptypes.ChainType,
	target *T,
) {
	methods, err := NewChainBlockMethods(chainType, target)
	if err != nil {
		return
	}

	server.AddMethod("getChainBlock", methods.GetChainBlock)
}

func parseChainType(v any) (apptypes.ChainType, error) {
	switch value := v.(type) {
	case apptypes.ChainType:
		return value, nil
	case uint64:
		return apptypes.ChainType(value), nil
	case float64:
		if value < 0 || math.Trunc(value) != value {
			return 0, ErrInvalidChainType
		}

		return apptypes.ChainType(uint64(value)), nil
	case string:
		s := strings.TrimSpace(value)

		var (
			parsed uint64
			err    error
		)

		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			parsed, err = strconv.ParseUint(s[2:], 16, 64)
		} else {
			parsed, err = strconv.ParseUint(s, 10, 64)
		}

		if err != nil {
			return 0, ErrInvalidChainType
		}

		return apptypes.ChainType(parsed), nil
	default:
		return 0, ErrInvalidChainType
	}
}

func decodeTarget[T any](param any) (*T, error) {
	switch value := param.(type) {
	case *T:
		return value, nil
	case map[string]any:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}

		var target T
		if err := json.Unmarshal(data, &target); err != nil {
			return nil, err
		}

		return &target, nil
	default:
		return nil, fmt.Errorf("%w: %T", errUnsupportedPayload, param)
	}
}
