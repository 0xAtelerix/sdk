package txpool

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/0xAtelerix/sdk/gosdk/utility"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"

	"github.com/0xAtelerix/sdk/gosdk/types"
)

// Определяем таблицы для хранения транзакций
const (
	txPoolBucket    = "txpool"
	txBatchesBucket = "txBatches"
)

// TxPool - generic пул транзакций с MDBX-хранилищем
type TxPool[T types.AppTransaction] struct {
	db kv.RwDB
}

// NewTxPool создает новый пул транзакций с MDBX-хранилищем
func NewTxPool[T types.AppTransaction](dbPath string) (*TxPool[T], error) {
	// Инициализация MDBX с логированием
	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(dbPath).
		WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				txPoolBucket: {},
			}
		}).
		Open()
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации MDBX: %w", err)
	}

	return &TxPool[T]{db: db}, nil
}

// AddTransaction добавляет транзакцию (generic)
func (p *TxPool[T]) AddTransaction(hash string, tx T) error {
	return p.db.Update(context.Background(), func(txn kv.RwTx) error {
		// Кодируем транзакцию в JSON
		data, err := json.Marshal(tx)
		if err != nil {
			return err
		}

		return txn.Put(txPoolBucket, []byte(hash), data)
	})
}

// GetTransaction возвращает транзакцию по хэшу
func (p *TxPool[T]) GetTransaction(hash string) (*T, error) {
	var txData []byte
	err := p.db.View(context.Background(), func(txn kv.Tx) error {
		var err error
		txData, err = txn.GetOne(txPoolBucket, []byte(hash))
		return err
	})
	if err != nil {
		return nil, err
	}

	// Декодируем JSON в объект T
	var tx T
	err = json.Unmarshal(txData, &tx)
	if err != nil {
		return nil, err
	}

	return &tx, nil
}

// RemoveTransaction удаляет транзакцию из пула
func (p *TxPool[T]) RemoveTransaction(hash string) error {
	return p.db.Update(context.TODO(), func(txn kv.RwTx) error {
		return txn.Delete(txPoolBucket, []byte(hash))
	})
}

// GetAllTransactions получает все транзакции (generic)
func (p *TxPool[T]) GetAllTransactions() ([]T, error) {
	var transactions []T

	err := p.db.View(context.TODO(), func(txn kv.Tx) error {
		it, err := txn.Cursor(txPoolBucket)
		if err != nil {
			return err
		}
		defer it.Close()

		for k, v, err := it.First(); k != nil && err == nil; k, v, err = it.Next() {
			var tx T
			err := json.Unmarshal(v, &tx)
			if err != nil {
				continue
			}
			transactions = append(transactions, tx)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

// GetAllTransactions получает все транзакции (generic)
func (p *TxPool[T]) CreateTransactionBatch() ([]byte, [][]byte, error) {
	var transactions [][]byte
	var batchHash []byte

	err := p.db.Update(context.TODO(), func(txn kv.RwTx) error {
		it, err := txn.Cursor(txPoolBucket)
		if err != nil {
			return err
		}
		defer it.Close()

		for k, v, err := it.First(); k != nil && err == nil; k, v, err = it.Next() {
			transactions = append(transactions, v)
			err = txn.Delete(txPoolBucket, k)
			if err != nil {
				return err
			}
		}

		txs := utility.Flatten(transactions)
		hash := sha256.New()
		hash.Write(txs)
		batchHash = hash.Sum(nil)

		return txn.Put(txBatchesBucket, batchHash, txs)
	})
	if err != nil {
		return nil, nil, err
	}

	return batchHash, transactions, nil
}

// Close закрывает MDBX
func (p *TxPool[T]) Close() error {
	p.db.Close()

	return nil
}
