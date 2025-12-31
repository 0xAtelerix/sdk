package rpc

import "github.com/0xAtelerix/sdk/gosdk/library/errors"

const (
	ErrMethodNotFound                 = errors.SDKError("method not found")
	ErrHashParameterMustBeString      = errors.SDKError("hash parameter must be a string")
	ErrInvalidHashFormat              = errors.SDKError("invalid hash format")
	ErrInvalidTransactionData         = errors.SDKError("invalid transaction data")
	ErrFailedToParseTransaction       = errors.SDKError("failed to parse transaction")
	ErrFailedToAddTransaction         = errors.SDKError("failed to add transaction")
	ErrFailedToGetReceipt             = errors.SDKError("failed to get receipt")
	ErrFailedToGetPendingTransactions = errors.SDKError("failed to get pending transactions")
	ErrReceiptNotFound                = errors.SDKError("receipt not found")
	ErrTransactionNotFound            = errors.SDKError("transaction not found")
	ErrWrongParamsCount               = errors.SDKError("wrong number of parameters")
	ErrBlockNotFound                  = errors.SDKError("block not found")
	ErrInvalidBlockNumber             = errors.SDKError("invalid block number")
	ErrBlockHasNoTransactions         = errors.SDKError("block has no transactions")
	ErrTransactionIndexOutOfBounds    = errors.SDKError("transaction index out of bounds")
)
