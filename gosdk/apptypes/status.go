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
	case Unknown:
		return unknownTxStatusString()
	case Pending:
		return "Pending"
	case Batched:
		return "Batched"
	case Processed:
		return "Processed"
	case Failed:
		return failed
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
		return unknownTxStatusString()
	case ReceiptFailed:
		return failed
	case ReceiptConfirmed:
		return confirmed
	default:
		return unknown
	}
}

func unknownTxStatusString() string {
	return unknown
}
