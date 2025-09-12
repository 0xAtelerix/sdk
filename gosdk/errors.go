package gosdk

type SDKError string

func (e SDKError) Error() string {
	return string(e)
}

const (
	ErrCorruptedFile  = SDKError("corrupted file")
	ErrUnknownChain   = SDKError("unknown chain")
	ErrMissingTxBatch = SDKError("missing tx batch")

	ErrWrongBlock = SDKError("wrong block hash")

	ErrUnsupportedEntityType = SDKError("unsupported entity type")
	ErrMalformedKey          = SDKError("malformed key")
	ErrUnsupportedFixture    = SDKError("unsupported type in FixtureWriter")
)
