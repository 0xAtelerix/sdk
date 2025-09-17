package apptypes

type TxStatus int

const (
	unknown   = "Unknown"
	failed    = "Failed"
	confirmed = "Confirmed"
)

const (
	Unknown TxStatus = iota
	Pending
	Batched
	Processed
	Failed
)

func (s TxStatus) String() string {
	switch s {
	case Pending:
		return "Pending"
	case Batched:
		return "Batched"
	case Processed:
		return "Processed"
	case Failed:
		return failed
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
	switch s {
	case ReceiptUnknown:
		return unknown
	case ReceiptFailed:
		return failed
	case ReceiptConfirmed:
		return confirmed
	default:
		return unknown
	}
}
