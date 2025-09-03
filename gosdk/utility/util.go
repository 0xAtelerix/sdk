package utility

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"strconv"
)

func Flatten(chunks [][]byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	var err error

	for _, chunk := range chunks {
		err = binary.Write(buf, binary.LittleEndian, uint32(len(chunk))) // length
		if err != nil {
			return nil, err
		}

		_, err = buf.Write(chunk) // content
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

const EndOfFile = "EOF"

func Unflatten(data []byte) ([][]byte, error) {
	var result [][]byte

	buf := bytes.NewReader(data)

	for {
		var length uint32

		err := binary.Read(buf, binary.LittleEndian, &length)
		if err != nil {
			if err.Error() == EndOfFile {
				break // корректное завершение
			}

			return nil, err
		}

		chunk := make([]byte, length)

		n, err := buf.Read(chunk)
		if err != nil && err.Error() != EndOfFile {
			return nil, err
		}

		if uint32(n) != length {
			return nil, io.ErrUnexpectedEOF
		}

		result = append(result, chunk)
	}

	return result, nil
}

func CheckHash(flat []byte, want []byte) (bool, error) {
	if len(want) != sha256.Size {
		return false, nil
	}

	h := sha256.New()

	_, err := h.Write(flat)
	if err != nil {
		return false, err
	}

	return bytes.Equal(h.Sum(nil), want), nil
}

type ctxValidatorKey struct{}

func CtxWithValidatorID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = unknownChainID
	}

	return context.WithValue(ctx, ctxValidatorKey{}, id)
}

func ValidatorIDFromCtx(ctx context.Context) string {
	if v := ctx.Value(ctxValidatorKey{}); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}

	return unknownChainID
}

type ctxChainIDKey struct{}

func CtxWithChainID(ctx context.Context, id uint64) context.Context {
	val := unknownChainID
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

	return unknownChainID
}

const unknownChainID = "unknown"
