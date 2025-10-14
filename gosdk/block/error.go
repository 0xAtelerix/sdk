package block

import "errors"

var (
	ErrNoBlocks                       = errors.New("no blocks found")
	ErrUnsupportedTransactionPayload  = errors.New("unsupported transaction payload")
	ErrDecodeTransactionPayloadFailed = errors.New("decode transaction payload failed")
	ErrUnsupportedBlockType           = errors.New("unsupported block type")
)
