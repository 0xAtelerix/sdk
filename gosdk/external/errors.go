package external

import (
	"errors"
)

// Static errors for validation.
var (
	ErrChainIDRequired = errors.New("chainID must be set")
)
