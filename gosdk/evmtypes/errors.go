package evmtypes

import (
	"github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	// ErrFieldNotFound is returned when a custom field is not found in the raw JSON.
	ErrFieldNotFound    = errors.SDKError("field not found")
	ErrEmptyCustomField = errors.SDKError("raw custom field is empty")
)
