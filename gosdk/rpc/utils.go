package rpc

import (
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
)

const (
	healthStatusHealthy = "healthy"
	jsonRPCVersion      = "2.0"
)

// writeError writes a JSON-RPC error response
func (*StandardRPCServer) writeError(
	w http.ResponseWriter,
	code int,
	message string,
) {
	response := JSONRPCResponse{
		JSONRPC: jsonRPCVersion,
		Error: &Error{
			Code:    code,
			Message: message,
		},
		ID: nil,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

// newErrorResponse creates a standard JSON-RPC error response
func newErrorResponse(err *Error, id any) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: jsonRPCVersion,
		Error:   err,
		ID:      id,
	}
}

// parseHash converts a hex string to a 32-byte array
func parseHash(hashStr string) ([32]byte, error) {
	var result [32]byte

	// Remove 0x prefix if present
	hashStr = strings.TrimPrefix(hashStr, "0x")

	// Decode hex
	bytes, err := hex.DecodeString(hashStr)
	if err != nil {
		return result, fmt.Errorf("invalid hex string: %w", err)
	}

	if len(bytes) != 32 {
		return result, fmt.Errorf("%w, got %d", ErrHashMustBe32Bytes, len(bytes))
	}

	copy(result[:], bytes)

	return result, nil
}

// parseBlockNumber normalizes a variety of numeric and string inputs into an
// unsigned block number, returning ErrInvalidBlockNumber when the value is
// negative, empty, or otherwise unsupported.
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
