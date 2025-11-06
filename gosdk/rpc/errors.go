package rpc

import "errors"

var (
	ErrMethodNotFound                 = errors.New("method not found")
	ErrHashParameterMustBeString      = errors.New("hash parameter must be a string")
	ErrInvalidHashFormat              = errors.New("invalid hash format")
	ErrInvalidTransactionData         = errors.New("invalid transaction data")
	ErrFailedToParseTransaction       = errors.New("failed to parse transaction")
	ErrFailedToAddTransaction         = errors.New("failed to add transaction")
	ErrFailedToGetReceipt             = errors.New("failed to get receipt")
	ErrFailedToGetPendingTransactions = errors.New("failed to get pending transactions")
	ErrReceiptNotFound                = errors.New("receipt not found")
	ErrTransactionNotFound            = errors.New("transaction not found")
	ErrWrongParamsCount               = errors.New("wrong number of parameters")
	ErrBlockNotFound                  = errors.New("block not found")
	ErrInvalidBlockNumber             = errors.New("invalid block number")
	ErrBlockHasNoTransactions         = errors.New("block has no transactions")
	ErrTransactionIndexOutOfBounds    = errors.New("transaction index out of bounds")
)
