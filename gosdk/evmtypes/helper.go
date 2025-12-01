package evmtypes

import (
	"fmt"

	"github.com/goccy/go-json"
)

// GetCustomFieldFromRaw extracts a chain-specific field from raw JSON.
// This is a helper function used by all EVM types to avoid code duplication.
func GetCustomFieldFromRaw(raw json.RawMessage, fieldName string) (any, error) {
	if len(raw) == 0 {
		return nil, ErrRawJSONNotAvailable
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw JSON: %w", err)
	}

	value, exists := data[fieldName]
	if !exists {
		return nil, ErrFieldNotFound
	}

	return value, nil
}
