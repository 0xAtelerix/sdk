package gosdk

type SDKError string

func (e SDKError) Error() string {
	return string(e)
}

const (
	ErrCorruptedFile  = SDKError("corrupted file")
	ErrUnknownChain   = SDKError("unknown chain")
	ErrMissingTxBatch = SDKError("missing tx batch")
	ErrNoReceipts     = SDKError("no receipts found")

	ErrWrongBlock = SDKError("wrong block hash")
)
