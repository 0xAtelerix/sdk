package txpool

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

// Определяем таблицы для хранения транзакций
const (
	txPoolBucket    = "txpool"
	txBatchedBucket = "txBatched" // txHash -> batch_hash
)

func Tables() kv.TableCfg {
	return kv.TableCfg{
		txPoolBucket:    {},
		txBatchedBucket: {},
	}
}

// TxPool - generic пул транзакций с MDBX-хранилищем
type TxPool[T apptypes.AppTransaction] struct {
	db kv.RwDB
}

// NewTxPool создает новый пул транзакций с MDBX-хранилищем
func NewTxPool[T apptypes.AppTransaction](db kv.RwDB) *TxPool[T] {
	return &TxPool[T]{
		db: db,
	}
}

// AddTransaction добавляет транзакцию (generic)
func (p *TxPool[T]) AddTransaction(ctx context.Context, tx T) error {
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
func (p *TxPool[T]) GetTransaction(ctx context.Context, hash []byte) (tx T, err error) {
	var (
		txData []byte
		dbErr  error
	)

	err = p.db.View(ctx, func(txn kv.Tx) error {
		txData, dbErr = txn.GetOne(txPoolBucket, hash)

		return dbErr
	})
	if err != nil {
		return tx, err
	}

	// Декодируем JSON в объект T
	err = json.Unmarshal(txData, &tx)
	if err != nil {
		return tx, fmt.Errorf(
			"error while unmarshal getTx result: %w, %d - %q, %T",
			err,
			len(txData),
			string(txData),
			&tx,
		)
	}

	return tx, nil
}

// RemoveTransaction удаляет транзакцию из пула
func (p *TxPool[T]) RemoveTransaction(ctx context.Context, hash []byte) error {
	return p.db.Update(ctx, func(txn kv.RwTx) error {
		return txn.Delete(txPoolBucket, hash)
	})
}

// GetAllTransactions получает все транзакции
func (p *TxPool[T]) GetPendingTransactions(ctx context.Context) ([]T, error) {
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
func (p *TxPool[T]) CreateTransactionBatch(ctx context.Context) ([]byte, [][]byte, error) {
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

			// TODO: for data consistency we need to get a fetcher response on successful tx save first and only then delete from txpool
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

		for _, tx := range transactions {
			var typedTx T
			err = json.Unmarshal(tx, &typedTx)
			if err != nil {
				return fmt.Errorf("can't serialize tx from txpool: %w", err)
			}

			txHash := typedTx.Hash()

			err = txn.Put(txBatchedBucket, txHash[:], batchHash)
			if err != nil {
				return fmt.Errorf("can't put a batched tx to txpool: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return batchHash, transactions, nil
}

// GetTransaction возвращает транзакцию по хэшу
func (p *TxPool[T]) GetTransactionStatus(ctx context.Context, hash []byte) (status TxStatus, err error) {
	var (
		txData []byte
		dbErr  error
	)

	err = p.db.View(ctx, func(txn kv.Tx) error {
		txData, dbErr = txn.GetOne(txPoolBucket, hash)

		return dbErr
	})
	if err == nil && len(txData) != 0 {
		return Pending, nil
	}

	err = p.db.View(ctx, func(txn kv.Tx) error {
		txData, dbErr = txn.GetOne(txBatchedBucket, hash)

		return dbErr
	})
	if err == nil && len(txData) != 0 {
		return Batched, nil
	}

	// TODO: handle: ReadyToProcess and Processed

	return status, nil
}

// Close закрывает MDBX
func (p *TxPool[T]) Close() error {
	p.db.Close()

	return nil
}
