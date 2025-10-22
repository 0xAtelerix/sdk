package rpc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/appblock"
)

type AppBlockMethods[T any] struct {
	appchainDB kv.RwDB
	template   T
}

var (
	errAppBlockMethodsNotInitialized = errors.New("chain block methods not initialized")
	errUnsupportedPayload            = errors.New("unsupported block payload type")
	errMissingBlockTemplate          = errors.New("block template not configured")
)

func NewAppBlockMethods[T any](appchainDB kv.RwDB, target T) *AppBlockMethods[T] {
	return &AppBlockMethods[T]{
		appchainDB: appchainDB,
		template:   target,
	}
}

func (m *AppBlockMethods[T]) GetAppBlock(
	ctx context.Context,
	params []any,
) (any, error) {
	if m == nil {
		return nil, errAppBlockMethodsNotInitialized
	}

	if len(params) != 1 {
		return nil, ErrWrongParamsCount
	}

	blockNumber, err := parseBlockNumber(params[0])
	if err != nil {
		return nil, err
	}

	var target T

	target, err = cloneTemplate(m.template)
	if err != nil {
		return nil, err
	}

	var payload []byte

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, blockNumber)

		data, getErr := tx.GetOne(gosdk.BlocksBucket, key)
		if getErr != nil {
			return getErr
		}

		if len(data) == 0 {
			return ErrBlockNotFound
		}

		payload = append([]byte(nil), data...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	fv, err := appblock.GetAppBlockByNumber(blockNumber, payload, target)
	if err != nil {
		return nil, err
	}

	return fv, nil
}

func cloneTemplate[T any](tpl T) (T, error) {
	var zero T

	val := reflect.ValueOf(tpl)
	if !val.IsValid() {
		return zero, errMissingBlockTemplate
	}

	typ := val.Type()
	if typ.Kind() != reflect.Pointer {
		return zero, fmt.Errorf("%w: %T", errUnsupportedPayload, tpl)
	}

	newVal := reflect.New(typ.Elem())

	result, ok := newVal.Interface().(T)
	if !ok {
		return zero, fmt.Errorf("%w: %T", errUnsupportedPayload, tpl)
	}

	return result, nil
}

func AddAppBlockMethods[T any](
	server *StandardRPCServer,
	appchainDB kv.RwDB,
	target T,
) {
	methods := NewAppBlockMethods(appchainDB, target)

	server.AddMethod("getAppBlock", methods.GetAppBlock)
}

func parseBlockNumber(v any) (uint64, error) {
	switch value := v.(type) {
	case uint64:
		return value, nil
	case int:
		if value < 0 {
			return 0, ErrInvalidBlockNumber
		}

		return uint64(value), nil
	case int64:
		if value < 0 {
			return 0, ErrInvalidBlockNumber
		}

		return uint64(value), nil
	case uint32:
		return uint64(value), nil
	case int32:
		if value < 0 {
			return 0, ErrInvalidBlockNumber
		}

		return uint64(value), nil
	case float64:
		if value < 0 || math.Trunc(value) != value {
			return 0, ErrInvalidBlockNumber
		}

		return uint64(value), nil
	case string:
		s := strings.TrimSpace(value)

		if s == "" {
			return 0, ErrInvalidBlockNumber
		}

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
			return 0, ErrInvalidBlockNumber
		}

		return parsed, nil
	default:
		return 0, ErrInvalidBlockNumber
	}
}
