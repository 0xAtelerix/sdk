package appblock

import "errors"

var (
	errTargetNil           = errors.New("target cannot be nil")
	errTargetNotPointer    = errors.New("target must be a pointer")
	errTargetNilPointer    = errors.New("target must be a non-nil pointer")
	errBlockPayloadEmpty   = errors.New("block payload is empty")
	errAppchainDatabase    = errors.New("appchain database cannot be nil")
	errAppBlockValueNil    = errors.New("block cannot be nil")
	errBlockNotFound       = errors.New("block not found")
	errTransactionsMissing = errors.New("block does not store transactions in payload")
	ErrUnsupportedPayload  = errors.New("unsupported block payload type")
	ErrTargetFactoryNil    = errors.New("block target factory not configured")
	errMissingSender       = errors.New("missing sender")
)
