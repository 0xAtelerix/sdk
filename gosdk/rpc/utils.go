package rpc

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/goccy/go-json"
)

// writeError writes a JSON-RPC error response
func (*StandardRPCServer) writeError(
	w http.ResponseWriter,
	code int,
	message string,
	id any,
) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &Error{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
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
