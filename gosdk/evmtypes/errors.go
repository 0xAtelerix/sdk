package evmtypes

import "errors"

// ErrFieldNotFound is returned when a custom field is not found in the raw JSON.
var ErrFieldNotFound = errors.New("field not found")

// ErrRawJSONNotAvailable is returned when Raw JSON is not set on a type.
var ErrRawJSONNotAvailable = errors.New("raw JSON not available")
