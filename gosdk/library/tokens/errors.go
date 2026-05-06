package tokens

import (
	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	ErrNonNilRequired        = sdkerrors.SDKError("out must be non-nil pointer to struct")
	ErrStructRequired        = sdkerrors.SDKError("out must point to a struct")
	ErrABIPlanTypeMismatched = sdkerrors.SDKError("type mismatch")
	ErrABIUnknownEvent       = sdkerrors.SDKError("unknown event")
)
