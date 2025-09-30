package tokens

import "github.com/0xAtelerix/sdk/gosdk"

const (
	ErrNonNilRequired             = gosdk.SDKError("out must be non-nil pointer to struct")
	ErrStructRequired             = gosdk.SDKError("out must point to a struct")
	ErrABIPlanTypeMismatched      = gosdk.SDKError("type mismatch")
	ErrABIUnknownEvent            = gosdk.SDKError("unknown event")
	ErrABIIncorrectNumberOfTopics = gosdk.SDKError(
		"incorrect number of topics with respect to inputs",
	)
)
