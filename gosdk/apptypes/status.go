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
	case Unknown:
		return unknown
	default:
		return unknown
	}
}

type TxReceiptStatus uint8

const (
	ReceiptUnknown TxReceiptStatus = iota
	ReceiptFailed
	ReceiptConfirmed
)

func (s TxReceiptStatus) String() string {
	const unknown = "Unknown"

	switch s {
	case ReceiptUnknown:
		return unknown
	case ReceiptFailed:
		return "Failed"
	case ReceiptConfirmed:
		return "Confirmed"
	default:
		return unknown
	}
}
