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
