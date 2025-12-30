package evmtypes

import "github.com/0xAtelerix/sdk/gosdk/library/errors"

// ErrFieldNotFound is returned when a custom field is not found in the raw JSON.
var ErrFieldNotFound = errors.SDKError("field not found")

// ErrRawJSONNotAvailable is returned when Raw JSON is not set on a type.
var ErrRawJSONNotAvailable = errors.SDKError("raw JSON not available")
