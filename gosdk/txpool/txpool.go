package txpool

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

// Определяем таблицы для хранения транзакций
const (
	txPoolBucket    = "txpool"
	txBatchesBucket = "txBatches"
)

func Tables() kv.TableCfg {
	return kv.TableCfg{
		txPoolBucket:    {},
		txBatchesBucket: {},
	}
}

// TxPool - generic пул транзакций с MDBX-хранилищем
type TxPool[T apptypes.AppTransaction, B apptypes.AppTransactionBuilder[T]] struct {
	db                    kv.RwDB
	appTransactionBuilder B
}

// NewTxPool создает новый пул транзакций с MDBX-хранилищем
func NewTxPool[T apptypes.AppTransaction, B apptypes.AppTransactionBuilder[T]](
	db kv.RwDB,
	txBuilder B,
) *TxPool[T, B] {
	return &TxPool[T, B]{
		db:                    db,
		appTransactionBuilder: txBuilder,
	}
}

// AddTransaction добавляет транзакцию (generic)
func (p *TxPool[T, B]) AddTransaction(ctx context.Context, tx T) error {
	return p.db.Update(ctx, func(txn kv.RwTx) error {
		// Кодируем транзакцию в JSON
		data, err := json.Marshal(tx)
		if err != nil {
			return err
		}

		hash := tx.Hash()

		return txn.Put(txPoolBucket, hash[:], data)
	})
}

// GetTransaction возвращает транзакцию по хэшу
func (p *TxPool[T, B]) GetTransaction(ctx context.Context, hash []byte) (T, error) {
	var txData []byte

	err := p.db.View(ctx, func(txn kv.Tx) error {
		var err error

		txData, err = txn.GetOne(txPoolBucket, hash)

		return err
	})
	if err != nil {
		return p.appTransactionBuilder.Make(), err
	}

	// Декодируем JSON в объект T
	var tx T

	err = json.Unmarshal(txData, &tx)
	if err != nil {
		return p.appTransactionBuilder.Make(), err
	}

	return tx, nil
}

// RemoveTransaction удаляет транзакцию из пула
func (p *TxPool[T, B]) RemoveTransaction(ctx context.Context, hash []byte) error {
	return p.db.Update(ctx, func(txn kv.RwTx) error {
		return txn.Delete(txPoolBucket, hash)
	})
}

// GetAllTransactions получает все транзакции (generic)
func (p *TxPool[T, B]) GetPendingTransactions(ctx context.Context) ([]T, error) {
	var transactions []T

	err := p.db.View(ctx, func(txn kv.Tx) error {
		it, err := txn.Cursor(txPoolBucket)
		if err != nil {
			return err
		}
		defer it.Close()

		for k, v, curErr := it.First(); k != nil && curErr == nil; k, v, curErr = it.Next() {
			var tx T

			curErr = json.Unmarshal(v, &tx)
			if curErr != nil {
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
func (p *TxPool[T, B]) CreateTransactionBatch(ctx context.Context) ([]byte, [][]byte, error) {
	var (
		transactions [][]byte
		batchHash    []byte
	)

	err := p.db.Update(ctx, func(txn kv.RwTx) error {
		it, err := txn.Cursor(txPoolBucket)
		if err != nil {
			return err
		}
		defer it.Close()

		for k, v, curErr := it.First(); k != nil && curErr == nil; k, v, curErr = it.Next() {
			transactions = append(transactions, v)

			curErr = txn.Delete(txPoolBucket, k)
			if curErr != nil {
				return curErr
			}
		}

		txs, err := utility.Flatten(transactions)
		if err != nil {
			return err
		}

		hash := sha256.New()

		_, err = hash.Write(txs)
		if err != nil {
			return err
		}

		batchHash = hash.Sum(nil)

		return txn.Put(txBatchesBucket, batchHash, txs)
	})
	if err != nil {
		return nil, nil, err
	}

	return batchHash, transactions, nil
}

// Close закрывает MDBX
func (p *TxPool[T, B]) Close() error {
	p.db.Close()

	return nil
}
