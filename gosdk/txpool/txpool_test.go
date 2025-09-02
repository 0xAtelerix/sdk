package txpool

import (
	"crypto/sha256"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// CustomTransaction - тестовая структура транзакции
//

type CustomTransaction[R Receipt] struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value int    `json:"value"`
}

func (c *CustomTransaction[R]) Unmarshal(b []byte) error {
	return json.Unmarshal(b, c)
}

func (c CustomTransaction[R]) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

func (c CustomTransaction[R]) Hash() [32]byte {
	s := c.From + c.To + strconv.Itoa(c.Value)

	return sha256.Sum256([]byte(s))
}

func (CustomTransaction[R]) Process(
	_ kv.RwTx,
) (r Receipt, txs []apptypes.ExternalTransaction, err error) {
	return
}

type Receipt struct{}

// randomTransaction генерирует случайную транзакцию
func randomTransaction[R Receipt]() *rapid.Generator[CustomTransaction[R]] {
	return rapid.Custom(func(t *rapid.T) CustomTransaction[R] {
		return CustomTransaction[R]{
			From:  rapid.StringN(1, 32, 32).Draw(t, "from"),
			To:    rapid.StringN(1, 32, 32).Draw(t, "to"),
			Value: rapid.IntRange(1, 10_000).Draw(t, "value"),
		}
	})
}

type CustomTransactionBuilder struct{}

func TestTxPool_PropertyBased(t *testing.T) {
	rapid.Check(t, func(tr *rapid.T) {
		t.Run(tr.Name(), func(t *testing.T) {
			dbPath := t.TempDir()

			// Инициализация MDBX с логированием
			db, err := mdbx.NewMDBX(mdbxlog.New()).
				Path(dbPath).
				WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
					return Tables()
				}).
				Open()
			require.NoError(tr, err)

			// Создаем новый TxPool
			txPool := NewTxPool[CustomTransaction[Receipt]](db)

			defer func() {
				poolErr := txPool.Close()
				require.NoError(tr, poolErr)
			}()

			// Генерируем случайное количество транзакций (1-100)
			txsSlice := rapid.SliceOfNDistinct(
				randomTransaction(), 1, 100,
				func(tx CustomTransaction[Receipt]) [32]byte { return tx.Hash() },
			).Draw(tr, "txs_distinct")

			txs := make(map[[32]byte]*CustomTransaction[Receipt])
			txHashes := make([][32]byte, 0, len(txsSlice))

			// Добавляем транзакции
			for _, tx := range txsSlice {
				txs[tx.Hash()] = &tx
				txHashes = append(txHashes, tx.Hash())

				if err = txPool.AddTransaction(t.Context(), tx); err != nil {
					tr.Fatalf("Ошибка добавления транзакции: %v", err)
				}
			}

			// Проверяем, что все добавленные транзакции можно извлечь
			for hash, expectedTx := range txs {
				var retrievedTx CustomTransaction[Receipt]

				retrievedTx, err = txPool.GetTransaction(t.Context(), hash[:])
				if err != nil {
					tr.Fatalf("Ошибка получения транзакции: %v", err)
				}

				if retrievedTx != *expectedTx {
					tr.Fatalf("Ожидалась %v, получена %v", *expectedTx, retrievedTx)
				}
			}

			// Проверяем, что GetPendingTransactions возвращает корректное количество
			allTxs, err := txPool.GetPendingTransactions(t.Context())
			if err != nil {
				tr.Fatalf("Ошибка получения всех транзакций: %v", err)
			}

			if len(allTxs) != len(txs) {
				tr.Fatalf("Ожидалось %d транзакций, получено %d", len(txs), len(allTxs))
			}

			// Удаляем случайное количество транзакций (до половины)
			numDeletes := rapid.IntRange(1, len(txs)/2+1).Draw(tr, "num_deletes")
			keysToDelete := make(map[[32]byte]struct{})

			for range numDeletes {
				key := rapid.SampledFrom(txHashes).Draw(tr, "delete_key")
				keysToDelete[key] = struct{}{}
			}

			for hash := range keysToDelete {
				if err = txPool.RemoveTransaction(t.Context(), hash[:]); err != nil {
					tr.Fatalf("Ошибка удаления транзакции: %v", err)
				}

				delete(txs, hash)
			}

			// Проверяем, что удаленные транзакции отсутствуют
			for hash := range keysToDelete {
				_, err = txPool.GetTransaction(t.Context(), hash[:])
				if err == nil {
					tr.Fatalf("Ожидалась ошибка при получении удаленной транзакции %s", hash)
				}
			}

			// Проверяем, что оставшиеся транзакции присутствуют
			for hash, expectedTx := range txs {
				var retrievedTx CustomTransaction[Receipt]

				retrievedTx, err = txPool.GetTransaction(t.Context(), hash[:])
				if err != nil {
					tr.Fatalf("Ошибка получения транзакции: %v", err)
				}

				if retrievedTx != *expectedTx {
					tr.Fatalf("Ожидалась %v, получена %v", *expectedTx, retrievedTx)
				}
			}

			// Проверяем, что GetPendingTransactions теперь возвращает уменьшенное количество
			allTxs, err = txPool.GetPendingTransactions(t.Context())
			if err != nil {
				tr.Fatalf("Ошибка получения всех транзакций: %v", err)
			}

			if len(allTxs) != len(txs) {
				tr.Fatalf("Ожидалось %d транзакций, получено %d", len(txs), len(allTxs))
			}
		})
	})
}
