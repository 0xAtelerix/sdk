package apptypes

import (
	"bytes"
	"context"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type AppTransaction[R Receipt] interface {
	Hash() [32]byte
	Process(dbTx kv.RwTx) (R, []ExternalTransaction, error)
}

type Receipt interface {
	TxHash() [32]byte
	Status() TxReceiptStatus
	Error() string
}

// AppTransaction should be serializible
type Batch[appTx AppTransaction[R], R Receipt] struct {
	Atropos        [32]byte        `cbor:"1,keyasint"`
	Transactions   []appTx         `cbor:"2,keyasint"`
	ExternalBlocks []ExternalBlock `cbor:"3,keyasint"`
	// todo add crossappchain tx
	// ExternalTransactions [][]byte
	EndOffset   int64 `cbor:"4,keyasint"`
	TxEndOffset int64 `cbor:"5,keyasint"` // txReader.position после чтения
}

type ExternalBlock struct {
	ChainID     uint64   `cbor:"1,keyasint"`
	BlockNumber uint64   `cbor:"2,keyasint"`
	BlockHash   [32]byte `cbor:"3,keyasint"`
}

// For DAG it represents an interval between two Atroposes
type AppchainBlock interface {
	Number() uint64
	Hash() [32]byte
	StateRoot() [32]byte
	Bytes() []byte
}

type StoredAppchainBlock[appBlock AppchainBlock] struct {
	Root  [32]byte `cbor:"1,keyasint"`
	Block appBlock `cbor:"2,keyasint"`
}

type AppchainBlockConstructor[appTx AppTransaction[R], R Receipt, block AppchainBlock] func(
	blockNumber uint64,
	stateRoot [32]byte,
	previousBlockHash [32]byte,
	txsBatch Batch[appTx, R]) block

// Для подключенных L1/L2 мы должны  уметь анмаршалить поле tx.
// Для межапчейновых - вставляем, как есть.
type ExternalTransaction struct {
	ChainID uint64 `cbor:"1,keyasint"`
	Tx      []byte `cbor:"2,keyasint"`
}

//3) Calculate state root
//Мы определяем или кто-то другой? Возможно нужен какой-то интерфейс, который можно подменять
/*
	Вариант 1) одна табличка стейта и префиксы для разных модулей
	Вариант 2) много табличек стейта и указывать, какие из них буду участвовать в стейт руте?
*/
type RootCalculator interface {
	StateRootCalculator(tx kv.RwTx) ([32]byte, error)
}

// Батч хеш батча транзакций, который надо обработать
type AppchainTxPoolBatch struct {
	ChainID uint64   `cbor:"1,keyasint"`
	Hash    [32]byte `cbor:"2,keyasint"`
}

type DB interface {
	Write() // some changes
	Read()  // some read actions
	Commit(checkpoint Checkpoint) error
}

// TxPoolInterface определяет методы для работы с пулом транзакций
type TxPoolInterface[T AppTransaction[R], R Receipt] interface {
	// AddTransaction добавляет транзакцию в пул
	AddTransaction(ctx context.Context, tx T) error

	// GetTransaction получает транзакцию по хэшу
	GetTransaction(ctx context.Context, hash []byte) (T, error)

	// RemoveTransaction удаляет транзакцию из пула
	RemoveTransaction(ctx context.Context, hash []byte) error

	// GetPendingTransactions возвращает все транзакции
	GetPendingTransactions(ctx context.Context) ([]T, error)

	CreateTransactionBatch(ctx context.Context) ([]byte, [][]byte, error)

	GetTransactionStatus(ctx context.Context, hash []byte) (TxStatus, error)

	// Close закрывает хранилище транзакций
	Close() error
}

// финализация перезода состояния аппчейна
type Checkpoint struct {
	ChainID                  uint64   `json:"chainId"                  cbor:"1,keyasint"`
	BlockNumber              uint64   `json:"blockNumber"              cbor:"2,keyasint"`
	BlockHash                [32]byte `json:"blockHash"                cbor:"3,keyasint"`
	StateRoot                [32]byte `json:"stateRoot"                cbor:"4,keyasint"`
	ExternalTransactionsRoot [32]byte `json:"externalTransactionsRoot" cbor:"5,keyasint"`
}

type Event struct {
	// todo возможно тут должно быть MedianTime
	Base          BaseEvent `json:"base"          cbor:"1,keyasint"`
	CreationTime  uint64    `json:"creationTime"  cbor:"2,keyasint"`
	PrevEpochHash *[32]byte `json:"prevEpochHash" cbor:"3,keyasint"`

	// батчи транзакций, которые были уже переданы другим валидаторам и у нас есть подпись, что они получены
	TxPool []AppchainTxPoolBatch `json:"txPool"     cbor:"4,keyasint"`
	// обновления состояния аппчейна, какой новый стейт рут, блок и внешние транзакции
	Appchains []Checkpoint `json:"appchains"  cbor:"5,keyasint"`
	// внешние блоки
	BlockVotes []ExternalBlock `json:"blockVotes" cbor:"6,keyasint"`

	Signature [64]byte `json:"signature" cbor:"7,keyasint"`
}

func (e Event) Bytes() ([]byte, error) {
	var buf bytes.Buffer

	enc := cbor.NewEncoder(&buf)

	if err := enc.Encode(e); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type BaseEvent struct {
	ID      [32]byte   `cbor:"1,keyasint"`
	Epoch   uint32     `cbor:"2,keyasint"`
	Seq     uint32     `cbor:"3,keyasint"`
	Frame   uint32     `cbor:"4,keyasint"`
	Creator uint32     `cbor:"5,keyasint"`
	Lamport uint32     `cbor:"6,keyasint"`
	Parents [][32]byte `cbor:"7,keyasint"`
}

type AppchainAddresses struct {
	ChainID        uint32 `cbor:"1,keyasint"`
	EmitterAddress string `cbor:"2,keyasint"`
}
