package gosdk

import "bytes"

type SDKError string

func (e SDKError) Error() string {
	return string(e)
}

const (
	ErrCorruptedFile         = SDKError("corrupted file")
	ErrUnknownChain          = SDKError("unknown chain")
	ErrMissingTxBatch        = SDKError("missing tx batch")
	ErrNoValidatorSet        = SDKError("no valset")
	ErrWrongBlock            = SDKError("wrong block hash")
	ErrEmptyTxBatchDB        = SDKError("tx batch db is nil")
	ErrUnsupportedEntityType = SDKError("unsupported entity type")
	ErrMalformedKey          = SDKError("malformed key")
	ErrUnsupportedFixture    = SDKError("unsupported type in FixtureWriter")

	ErrAppBlockTargetNil           = SDKError("target cannot be nil")
	ErrAppBlockTargetNotPointer    = SDKError("target must be a pointer")
	ErrAppBlockTargetNilPointer    = SDKError("target must be a non-nil pointer")
	ErrAppBlockPayloadEmpty        = SDKError("block payload is empty")
	ErrAppBlockDatabaseNil         = SDKError("appchain database cannot be nil")
	ErrAppBlockValueNil            = SDKError("block cannot be nil")
	ErrAppBlockNotFound            = SDKError("block not found")
	ErrAppBlockTransactionsMissing = SDKError("block does not store transactions in payload")
	ErrAppBlockUnsupportedPayload  = SDKError("unsupported block payload type")
	ErrAppBlockTargetTemplateNil   = SDKError("block target not configured")
	ErrAppBlockTargetNotStruct     = SDKError("block target must be a struct or pointer to struct")
	ErrAppBlockMissingSender       = SDKError("missing sender")
)

//nolint:gochecknoglobals // read only
var endOfEpochSuffix = bytes.Repeat(
	[]byte{0xFF},
	28,
)
