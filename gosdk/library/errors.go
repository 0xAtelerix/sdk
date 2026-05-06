package library

import (
	"bytes"

	sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"
)

const (
	ErrCorruptedFile  = sdkerrors.SDKError("corrupted file")
	ErrUnknownChain   = sdkerrors.SDKError("unknown chain")
	ErrMissingTxBatch = sdkerrors.SDKError("missing tx batch")
	ErrNoValidatorSet = sdkerrors.SDKError("no valset")

	ErrWrongBlock   = sdkerrors.SDKError("wrong block hash")
	ErrHashMismatch = sdkerrors.SDKError(
		"block hash mismatch: stored hash does not match computed hash",
	)

	ErrEmptyTxBatchDB = sdkerrors.SDKError("tx batch db is nil")

	ErrUnsupportedEntityType = sdkerrors.SDKError("unsupported entity type")
	ErrMalformedKey          = sdkerrors.SDKError("malformed key")
	ErrUnsupportedFixture    = sdkerrors.SDKError("unsupported type in FixtureWriter")

	ErrBlockMarshalling              = sdkerrors.SDKError("failed to marshal block")
	ErrBlockWrite                    = sdkerrors.SDKError("failed to write block")
	ErrBlockTransactionsWrite        = sdkerrors.SDKError("failed to write block transactions")
	ErrTransactionsMarshalling       = sdkerrors.SDKError("failed to marshal transactions")
	ErrTransactionLookupWrite        = sdkerrors.SDKError("failed to write transaction lookup")
	ErrExternalTransactionsGet       = sdkerrors.SDKError("failed to get external transactions")
	ErrExternalTransactionsUnmarshal = sdkerrors.SDKError(
		"failed to unmarshal external transactions",
	)

	ErrNotImplemented = sdkerrors.SDKError("not implemented")
)

//nolint:gochecknoglobals // read only
var EndOfEpochSuffix = bytes.Repeat(
	[]byte{0xFF},
	28,
)
