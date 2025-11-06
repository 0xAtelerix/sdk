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

	ErrBlockMarshalling              = SDKError("failed to marshal block")
	ErrBlockWrite                    = SDKError("failed to write block")
	ErrBlockTransactionsWrite        = SDKError("failed to write block transactions")
	ErrTransactionsMarshalling       = SDKError("failed to marshal transactions")
	ErrTransactionLookupWrite        = SDKError("failed to write transaction lookup")
	ErrExternalTransactionsGet       = SDKError("failed to get external transactions")
	ErrExternalTransactionsUnmarshal = SDKError("failed to unmarshal external transactions")
)

//nolint:gochecknoglobals // read only
var endOfEpochSuffix = bytes.Repeat(
	[]byte{0xFF},
	28,
)
