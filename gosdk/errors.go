package gosdk

import "bytes"

type SDKError string

func (e SDKError) Error() string {
	return string(e)
}

const (
	ErrCorruptedFile  = SDKError("corrupted file")
	ErrUnknownChain   = SDKError("unknown chain")
	ErrMissingTxBatch = SDKError("missing tx batch")
	ErrNoValidatorSet = SDKError("no valset")

	ErrWrongBlock = SDKError("wrong block hash")

	ErrEmptyTxBatchDB = SDKError("tx batch db is nil")

	ErrUnsupportedEntityType = SDKError("unsupported entity type")
	ErrMalformedKey          = SDKError("malformed key")
	ErrUnsupportedFixture    = SDKError("unsupported type in FixtureWriter")
)

//nolint:gochecknoglobals // read only
var EndOfEpochSuffix = bytes.Repeat(
	[]byte{0xFF},
	28,
)
