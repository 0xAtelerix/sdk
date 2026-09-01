package txpool

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/rs/zerolog/log"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/utility"
)

// Определяем таблицы для хранения транзакций
const (
	txPoolBucket    = "txpool"
	txBatchedBucket = "txBatched" // txHash -> batch_hash
	artifactBucket  = "txArtifacts"
)

// ErrArtifactNotFound reports that no artifact exists for an opaque key.
var ErrArtifactNotFound = errors.New("txpool artifact not found")

var (
	errArtifactKeyEmpty = errors.New("txpool artifact key is empty")
	errArtifactEmpty    = errors.New("txpool artifact is empty")
)

func Tables() kv.TableCfg {
	return kv.TableCfg{
		txPoolBucket:    {},
		txBatchedBucket: {},
		artifactBucket:  {},
	}
}

// AddTransactionWithArtifact atomically adds a transaction and an opaque
// compiled artifact. The artifact key and value schema belong to the caller.
func (p *TxPool[T, R]) AddTransactionWithArtifact(
	ctx context.Context,
	tx T,
	artifactKey []byte,
	artifact []byte,
) error {
	data, err := cbor.Marshal(tx)
	if err != nil {
		return err
	}

	if len(artifactKey) == 0 {
		return errArtifactKeyEmpty
	}

	if len(artifact) == 0 {
		return errArtifactEmpty
	}

	hash := tx.Hash()

	return p.db.Update(ctx, func(txn kv.RwTx) error {
		if err := txn.Put(artifactBucket, artifactKey, artifact); err != nil {
			return fmt.Errorf("put txpool artifact: %w", err)
		}

		if err := txn.Put(txPoolBucket, hash[:], data); err != nil {
			return fmt.Errorf("put txpool transaction: %w", err)
		}

		return nil
	})
}

// GetArtifact returns an owned copy of opaque artifact bytes.
func (p *TxPool[T, R]) GetArtifact(ctx context.Context, artifactKey []byte) ([]byte, error) {
	var artifact []byte

	err := p.db.View(ctx, func(txn kv.Tx) error {
		stored, err := txn.GetOne(artifactBucket, artifactKey)
		if err != nil {
			return err
		}

		if len(stored) == 0 {
			return ErrArtifactNotFound
		}

		artifact = append([]byte(nil), stored...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return artifact, nil
}

// TxPool - generic пул транзакций с MDBX-хранилищем
type TxPool[T apptypes.AppTransaction[R], R apptypes.Receipt] struct {
	db kv.RwDB
}

// NewTxPool создает новый пул транзакций с MDBX-хранилищем
func NewTxPool[T apptypes.AppTransaction[R], R apptypes.Receipt](db kv.RwDB) *TxPool[T, R] {
	return &TxPool[T, R]{
		db: db,
	}
}

// AddTransaction добавляет транзакцию (generic)
func (p *TxPool[T, R]) AddTransaction(ctx context.Context, tx T) error {
	return p.db.Update(ctx, func(txn kv.RwTx) error {
		// Кодируем транзакцию в cbor
		data, err := cbor.Marshal(tx)
		if err != nil {
			return err
		}

		hash := tx.Hash()

		return txn.Put(txPoolBucket, hash[:], data)
	})
}

// GetTransaction возвращает транзакцию по хэшу
func (p *TxPool[T, R]) GetTransaction(ctx context.Context, hash []byte) (tx T, err error) {
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

	// Декодируем cbor в объект T
	err = cbor.Unmarshal(txData, &tx)
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
func (p *TxPool[T, R]) RemoveTransaction(ctx context.Context, hash []byte) error {
	return p.db.Update(ctx, func(txn kv.RwTx) error {
		return txn.Delete(txPoolBucket, hash)
	})
}

// GetPendingTransactions returns all pending transactions from the pool.
func (p *TxPool[T, R]) GetPendingTransactions(ctx context.Context) ([]T, error) {
	var transactions []T

	err := p.db.View(ctx, func(txn kv.Tx) error {
		it, err := txn.Cursor(txPoolBucket)
		if err != nil {
			return err
		}
		defer it.Close()

		for k, v, curErr := it.First(); k != nil && curErr == nil; k, v, curErr = it.Next() {
			var tx T

			curErr = cbor.Unmarshal(v, &tx)
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

// CreateTransactionBatch returns all transactions as a generic transaction batch.
func (p *TxPool[T, R]) CreateTransactionBatch(ctx context.Context) ([]byte, [][]byte, error) {
	log.Debug().Msg("creating transaction batch")

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
			// Values returned by MDBX cursor are memory-mapped and only valid until the next cursor op.
			// We must copy the value before we move/delete the cursor entry, otherwise data may be corrupted.
			copied := make([]byte, len(v))
			copy(copied, v)
			transactions = append(transactions, copied)

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

			err = cbor.Unmarshal(tx, &typedTx)
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

// GetTransactionStatus returns a transaction status by transaction hash.
func (p *TxPool[T, R]) GetTransactionStatus(
	ctx context.Context,
	hash []byte,
) (status apptypes.TxStatus, err error) {
	var (
		txData []byte
		dbErr  error
	)

	err = p.db.View(ctx, func(txn kv.Tx) error {
		txData, dbErr = txn.GetOne(txPoolBucket, hash)

		return dbErr
	})
	if err == nil && len(txData) != 0 {
		return apptypes.Pending, nil
	}

	err = p.db.View(ctx, func(txn kv.Tx) error {
		txData, dbErr = txn.GetOne(txBatchedBucket, hash)

		return dbErr
	})
	if err == nil && len(txData) != 0 {
		return apptypes.Batched, nil
	}

	return status, nil
}

// Close закрывает MDBX
func (p *TxPool[T, R]) Close() error {
	p.db.Close()

	return nil
}
