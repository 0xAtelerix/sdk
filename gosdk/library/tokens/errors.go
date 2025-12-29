package tokens

import (
	"github.com/0xAtelerix/sdk/gosdk/library"
)

const (
	ErrNonNilRequired        = library.SDKError("out must be non-nil pointer to struct")
	ErrStructRequired        = library.SDKError("out must point to a struct")
	ErrABIPlanTypeMismatched = library.SDKError("type mismatch")
	ErrABIUnknownEvent       = library.SDKError("unknown event")
)
