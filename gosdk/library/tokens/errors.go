package tokens

import (
	"github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	ErrNonNilRequired        = errors.SDKError("out must be non-nil pointer to struct")
	ErrStructRequired        = errors.SDKError("out must point to a struct")
	ErrABIPlanTypeMismatched = errors.SDKError("type mismatch")
	ErrABIUnknownEvent       = errors.SDKError("unknown event")
)
