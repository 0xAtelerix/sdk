package evmtypes

import sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"

const (
	// ErrFieldNotFound is returned when a custom field is not found in the raw JSON.
	ErrFieldNotFound = sdkerrors.SDKError("field not found")

	// ErrRawJSONNotAvailable is returned when Raw JSON is not set on a type.
	ErrRawJSONNotAvailable = sdkerrors.SDKError("raw JSON not available")
)
