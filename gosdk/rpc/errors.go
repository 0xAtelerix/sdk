package rpc

import sdkerrors "github.com/0xAtelerix/sdk/gosdk/library/errors"

const (
	ErrMethodNotFound                 = sdkerrors.SDKError("method not found")
	ErrHashParameterMustBeString      = sdkerrors.SDKError("hash parameter must be a string")
	ErrInvalidHashFormat              = sdkerrors.SDKError("invalid hash format")
	ErrInvalidTransactionData         = sdkerrors.SDKError("invalid transaction data")
	ErrFailedToParseTransaction       = sdkerrors.SDKError("failed to parse transaction")
	ErrFailedToAddTransaction         = sdkerrors.SDKError("failed to add transaction")
	ErrFailedToGetReceipt             = sdkerrors.SDKError("failed to get receipt")
	ErrFailedToGetPendingTransactions = sdkerrors.SDKError("failed to get pending transactions")
	ErrReceiptNotFound                = sdkerrors.SDKError("receipt not found")
	ErrTransactionNotFound            = sdkerrors.SDKError("transaction not found")
	ErrWrongParamsCount               = sdkerrors.SDKError("wrong number of parameters")
	ErrBlockNotFound                  = sdkerrors.SDKError("block not found")
	ErrInvalidBlockNumber             = sdkerrors.SDKError("invalid block number")
	ErrBlockHasNoTransactions         = sdkerrors.SDKError("block has no transactions")
	ErrTransactionIndexOutOfBounds    = sdkerrors.SDKError("transaction index out of bounds")
)
