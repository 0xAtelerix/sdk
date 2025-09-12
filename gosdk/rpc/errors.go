package rpc

import "errors"

var (
	ErrMethodNotFound                      = errors.New("method not found")
	ErrGetTransactionReceiptRequires1Param = errors.New(
		"getTransactionReceipt requires exactly 1 parameter",
	)
	ErrGetTransactionByHashRequires1Param = errors.New(
		"getTransactionByHash requires exactly 1 parameter",
	)
	ErrSendTransactionRequires1Param = errors.New(
		"sendTransaction requires exactly 1 parameter",
	)
	ErrGetTransactionStatusRequires1Param = errors.New(
		"getTransactionStatus requires exactly 1 parameter",
	)
	ErrHashParameterMustBeString      = errors.New("hash parameter must be a string")
	ErrInvalidHashFormat              = errors.New("invalid hash format")
	ErrInvalidTransactionData         = errors.New("invalid transaction data")
	ErrFailedToParseTransaction       = errors.New("failed to parse transaction")
	ErrFailedToAddTransaction         = errors.New("failed to add transaction")
	ErrFailedToGetReceipt             = errors.New("failed to get receipt")
	ErrFailedToGetPendingTransactions = errors.New("failed to get pending transactions")
	ErrHashMustBe32Bytes              = errors.New("hash must be 32 bytes")
	ErrReceiptNotFound                = errors.New("receipt not found")
	ErrTransactionNotFound            = errors.New("transaction not found")
)
