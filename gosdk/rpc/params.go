package rpc

import (
	"strconv"
	"strings"
)

// parseNumber converts an incoming JSON-RPC parameter into a uint64.
// It accepts numeric values, decimal strings, or 0x-prefixed hex strings.
func parseNumber(v any) (uint64, error) {
	switch n := v.(type) {
	case uint64:
		return n, nil
	case string:
		s := strings.TrimSpace(n)
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			ui, err := strconv.ParseUint(s[2:], 16, 64)
			if err != nil {
				return 0, ErrInvalidBlockNumber
			}

			return ui, nil
		}

		ui, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, ErrInvalidBlockNumber
		}

		return ui, nil
	default:
		return 0, ErrInvalidBlockNumber
	}
}
