package apptypes

type TxStatus int

const (
	Unknown TxStatus = iota
	Pending
	Batched
	ReadyToProcess
	Processed
)

func (s TxStatus) String() string {
	const unknown = "Unknown"

	switch s {
	case Pending:
		return "Pending"
	case Batched:
		return "Batched"
	case ReadyToProcess:
		return "ReadyToProcess"
	case Processed:
		return "Processed"
	}
	return unknown
}

type TxReceiptStatus uint8

const (
	ReceiptFailed TxReceiptStatus = iota
	ReceiptConfirmed
)

func (s TxReceiptStatus) String() string {
	const unknown = "Unknown"

	switch s {
	case ReceiptFailed:
		return "Failed"
	case ReceiptConfirmed:
		return "Confirmed"
	}
	return unknown
}
