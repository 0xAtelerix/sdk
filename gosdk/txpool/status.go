package txpool

type TxStatus int

const (
	Unknown TxStatus = iota
	Pending
	Batched
	ReadyToProcess
	Processed
)

func (s TxStatus) String() string {
	switch s {
	case Pending:
		return "Pending"
	case Batched:
		return "Batched"
	case ReadyToProcess:
		return "ReadyToProcess"
	case Processed:
		return "Processed"
	default:
		return "Unknown"
	}
}
