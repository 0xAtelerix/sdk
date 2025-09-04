package gosdk

type SDKError string

func (e SDKError) Error() string {
	return string(e)
}

const (
	ErrCorruptedFile  = SDKError("corrupted file")
	ErrUnknownChain   = SDKError("no DB for chainID")
	ErrMissingTxBatch = SDKError("missing tx batch")
	ErrNoReceipts     = SDKError("no receipts found")
)
