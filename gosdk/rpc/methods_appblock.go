package rpc

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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
	fmt.Println("AppBlockMethods[T), GetAppBlock")
	if m == nil {
		return nil, errAppBlockMethodsNotInitialized
	}

	if len(params) != 2 {
		return nil, ErrWrongParamsCount
	}

	blockNumber, err := parseBlockNumber(params[0])
	if err != nil {
		return nil, err
	}

	target, err := parseTarget[T](params[1])
	if err != nil {
		return nil, err
	}

	var payload []byte

	err = m.appchainDB.View(ctx, func(tx kv.Tx) error {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, blockNumber)

		fmt.Printf("retrieve block data from gosdk.BlocksBucket by key, blocknumber: %d\n", blockNumber)
		data, getErr := tx.GetOne(gosdk.BlocksBucket, key)
		if getErr != nil {
			return getErr
		}
		fmt.Println("retrieval successful, block data:", hex.EncodeToString(data))

		if len(data) == 0 {
			return ErrBlockNotFound
		}

		payload = append([]byte(nil), data...)

		fmt.Println("payload:", hex.EncodeToString(payload))

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

func parseTarget[T any](param any) (T, error) {
	var zero T

	switch value := param.(type) {
	case T:
		return value, nil
	case map[string]any:
		if err := validateBlockPayloadKeys[T](value); err != nil {
			return zero, err
		}

		data, err := json.Marshal(value)
		if err != nil {
			return zero, fmt.Errorf("encode block payload: %w", err)
		}

		var target T
		if err := json.Unmarshal(data, &target); err != nil {
			return zero, fmt.Errorf("decode block payload: %w", err)
		}

		return target, nil
	default:
		return zero, fmt.Errorf("%w: %T", errUnsupportedPayload, param)
	}
}

func validateBlockPayloadKeys[T any](payload map[string]any) error {
	if len(payload) == 0 {
		return fmt.Errorf("%w: %T", errUnsupportedPayload, payload)
	}

	typ := reflect.TypeOf((*T)(nil)).Elem()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("%w: %T", errUnsupportedPayload, payload)
	}

	allowed := make(map[string]struct{})

	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
			if idx := strings.Index(tag, ","); idx != -1 {
				tag = tag[:idx]
			}

			if tag != "" {
				name = tag
			}
		}

		allowed[name] = struct{}{}
	}

	for key := range payload {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%w: %T", errUnsupportedPayload, payload)
		}
	}

	return nil
}
