package txpool

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"
	"os"
	"pgregory.net/rapid"
	"strconv"
	"testing"
)

// CustomTransaction - тестовая структура транзакции
type CustomTransaction struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value int    `json:"value"`
}

func (c *CustomTransaction) Unmarshal(b []byte) error {
	return json.Unmarshal(b, c)
}
func (c *CustomTransaction) Marshal() ([]byte, error) {
	return json.Marshal(c)
}
func (c CustomTransaction) Hash() [32]byte {
	s := c.From + c.To + strconv.Itoa(c.Value)
	return sha256.Sum256([]byte(s))
}

// randomTransaction генерирует случайную транзакцию
func randomTransaction(t *rapid.T) CustomTransaction {
	return CustomTransaction{
		From:  rapid.StringN(1, 32, 32).Draw(t, "from"),
		To:    rapid.StringN(1, 32, 32).Draw(t, "to"),
		Value: rapid.IntRange(1, 10000).Draw(t, "value"),
	}
}

func TestTxPool_PropertyBased(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Путь к тестовой БД
		dbPath := "./test_txpool_db"
		defer os.RemoveAll(dbPath) // Удаляем базу после теста

		// Инициализация MDBX с логированием
		db, err := mdbx.NewMDBX(mdbxlog.New()).
			Path(dbPath).
			WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
				return TxPoolTables
			}).
			Open()
		require.NoError(t, err)

		// Создаем новый TxPool
		txPool := NewTxPool[CustomTransaction](db)
		defer txPool.Close()

		// Генерируем случайное количество транзакций (1-100)
		numTxs := rapid.IntRange(1, 100).Draw(t, "num_txs")
		txs := make(map[[32]byte]CustomTransaction)
		txHashes := make([][32]byte, 0, numTxs)

		// Добавляем транзакции
		for i := 0; i < numTxs; i++ {
			tx := randomTransaction(t)

			// Убеждаемся, что `Hash` уникален
			if _, exists := txs[tx.Hash()]; exists {
				continue
			}

			txs[tx.Hash()] = tx
			txHashes = append(txHashes, tx.Hash())

			if err := txPool.AddTransaction(tx); err != nil {
				t.Fatalf("Ошибка добавления транзакции: %v", err)
			}
		}

		// Проверяем, что все добавленные транзакции можно извлечь
		for hash, expectedTx := range txs {
			retrievedTx, err := txPool.GetTransaction(hash[:])
			if err != nil {
				t.Fatalf("Ошибка получения транзакции: %v", err)
			}
			if retrievedTx == nil {
				t.Fatalf("Ожидалась транзакция %v, но получен nil", expectedTx)
			}
			if *retrievedTx != expectedTx {
				t.Fatalf("Ожидалась %v, получена %v", expectedTx, *retrievedTx)
			}
		}

		// Проверяем, что GetPendingTransactions возвращает корректное количество
		allTxs, err := txPool.GetPendingTransactions()
		if err != nil {
			t.Fatalf("Ошибка получения всех транзакций: %v", err)
		}
		if len(allTxs) != len(txs) {
			t.Fatalf("Ожидалось %d транзакций, получено %d", len(txs), len(allTxs))
		}

		// Удаляем случайное количество транзакций (до половины)
		numDeletes := rapid.IntRange(1, len(txs)/2+1).Draw(t, "num_deletes")
		keysToDelete := make(map[[32]byte]struct{})
		for i := 0; i < numDeletes; i++ {
			key := rapid.SampledFrom(txHashes).Draw(t, "delete_key")
			keysToDelete[key] = struct{}{}
		}

		for hash := range keysToDelete {
			if err := txPool.RemoveTransaction(hash[:]); err != nil {
				t.Fatalf("Ошибка удаления транзакции: %v", err)
			}
			delete(txs, hash)
		}

		// Проверяем, что удаленные транзакции отсутствуют
		for hash := range keysToDelete {
			_, err := txPool.GetTransaction(hash[:])
			if err == nil {
				t.Fatalf("Ожидалась ошибка при получении удаленной транзакции %s", hash)
			}
		}

		// Проверяем, что оставшиеся транзакции присутствуют
		for hash, expectedTx := range txs {
			retrievedTx, err := txPool.GetTransaction(hash[:])
			if err != nil {
				t.Fatalf("Ошибка получения транзакции: %v", err)
			}
			if retrievedTx == nil || *retrievedTx != expectedTx {
				t.Fatalf("Ожидалась %v, получена %v", expectedTx, retrievedTx)
			}
		}

		// Проверяем, что GetPendingTransactions теперь возвращает уменьшенное количество
		allTxs, err = txPool.GetPendingTransactions()
		if err != nil {
			t.Fatalf("Ошибка получения всех транзакций: %v", err)
		}
		if len(allTxs) != len(txs) {
			t.Fatalf("Ожидалось %d транзакций, получено %d", len(txs), len(allTxs))
		}
	})
}
