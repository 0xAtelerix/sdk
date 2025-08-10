package utility

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
	"strconv"
)

func Flatten(chunks [][]byte) []byte {
	buf := new(bytes.Buffer)
	for _, chunk := range chunks {
		binary.Write(buf, binary.LittleEndian, uint32(len(chunk))) // Пишем длину
		buf.Write(chunk)                                           // Пишем содержимое
	}
	return buf.Bytes()
}

func Unflatten(data []byte) ([][]byte, error) {
	var result [][]byte
	buf := bytes.NewReader(data)

	for {
		var length uint32
		err := binary.Read(buf, binary.LittleEndian, &length)
		if err != nil {
			if err.Error() == "EOF" {
				break // корректное завершение
			}
			return nil, err
		}

		chunk := make([]byte, length)
		n, err := buf.Read(chunk)
		if err != nil && err.Error() != "EOF" {
			return nil, err
		}

		if uint32(n) != length {
			return nil, io.ErrUnexpectedEOF
		}

		result = append(result, chunk)
	}

	return result, nil
}

func CheckHash(flat []byte, want []byte) bool {
	if len(want) != sha256.Size {
		return false
	}
	var h hash.Hash = sha256.New()
	h.Write(flat)
	return bytes.Equal(h.Sum(nil), want)
}

type ctxValidatorKey struct{}

func CtxWithValidatorID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = "unknown"
	}
	return context.WithValue(ctx, ctxValidatorKey{}, id)
}

func ValidatorIDFromCtx(ctx context.Context) string {
	if v := ctx.Value(ctxValidatorKey{}); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}

type ctxChainIDKey struct{}

func CtxWithChainID(ctx context.Context, id uint64) context.Context {
	val := "unknown"
	if id != 0 {
		val = strconv.FormatUint(id, 10)
	}
	return context.WithValue(ctx, ctxChainIDKey{}, val)
}
func ChainIDFromCtx(ctx context.Context) string {
	if v := ctx.Value(ctxChainIDKey{}); v != nil {
		if u, ok := v.(string); ok {
			return u
		}
	}
	return "unknown"
}
