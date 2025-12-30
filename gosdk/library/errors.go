package library

import (
	"bytes"

	"github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	ErrCorruptedFile  = errors.SDKError("corrupted file")
	ErrUnknownChain   = errors.SDKError("unknown chain")
	ErrMissingTxBatch = errors.SDKError("missing tx batch")
	ErrNoValidatorSet = errors.SDKError("no valset")

	ErrWrongBlock   = errors.SDKError("wrong block hash")
	ErrHashMismatch = errors.SDKError("block hash mismatch: stored hash does not match computed hash")

	ErrEmptyTxBatchDB = errors.SDKError("tx batch db is nil")

	ErrUnsupportedEntityType = errors.SDKError("unsupported entity type")
	ErrMalformedKey          = errors.SDKError("malformed key")
	ErrUnsupportedFixture    = errors.SDKError("unsupported type in FixtureWriter")

	ErrBlockMarshalling              = errors.SDKError("failed to marshal block")
	ErrBlockWrite                    = errors.SDKError("failed to write block")
	ErrBlockTransactionsWrite        = errors.SDKError("failed to write block transactions")
	ErrTransactionsMarshalling       = errors.SDKError("failed to marshal transactions")
	ErrTransactionLookupWrite        = errors.SDKError("failed to write transaction lookup")
	ErrExternalTransactionsGet       = errors.SDKError("failed to get external transactions")
	ErrExternalTransactionsUnmarshal = errors.SDKError("failed to unmarshal external transactions")

	ErrNotImplemented = errors.SDKError("not implemented")
)

//nolint:gochecknoglobals // read only
var EndOfEpochSuffix = bytes.Repeat(
	[]byte{0xFF},
	28,
)
