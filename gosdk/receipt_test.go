package gosdk

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// TestReceipt is a mock implementation of apptypes.Receipt for testing
type TestReceipt struct {
	Hash   [32]byte
	Data   string
	Failed bool
}

func (r *TestReceipt) TxHash() [32]byte {
	return r.Hash
}

func (r *TestReceipt) Status() apptypes.TxReceiptStatus {
	if r.Failed {
		return apptypes.ReceiptFailed
	}
	return apptypes.ReceiptConfirmed
}

func (r *TestReceipt) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *TestReceipt) Unmarshal(data []byte) error {
	return json.Unmarshal(data, r)
}

// Helper function to create a test receipt
func createTestReceipt(data string, failed bool) *TestReceipt {
	hash := sha256.Sum256([]byte(data))
	return &TestReceipt{
		Hash:   hash,
		Data:   data,
		Failed: failed,
	}
}

func TestStoreAndGetReceipt(t *testing.T) {
	// Create in-memory database
	db := memdb.NewTestDB(t)
	defer db.Close()

	// Create tables
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		// Create the receipts bucket
		return tx.CreateBucket(ReceiptBucket)
	})
	require.NoError(t, err)

	t.Run("successful store and retrieve", func(t *testing.T) {
		receipt := createTestReceipt("test-receipt-1", false)

		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(t, err)

		// Retrieve the receipt
		var retrievedReceipt *TestReceipt
		err = db.View(context.Background(), func(tx kv.Tx) error {
			var err error
			txHash := receipt.TxHash()
			retrievedReceipt, err = GetReceipt(tx, txHash[:], &TestReceipt{})
			return err
		})
		require.NoError(t, err)

		// Verify the retrieved receipt matches the original
		assert.Equal(t, receipt.Hash, retrievedReceipt.Hash)
		assert.Equal(t, receipt.Data, retrievedReceipt.Data)
		assert.Equal(t, receipt.Failed, retrievedReceipt.Failed)
		assert.Equal(t, receipt.Status(), retrievedReceipt.Status())
	})

	t.Run("store multiple receipts", func(t *testing.T) {
		receipts := []*TestReceipt{
			createTestReceipt("receipt-1", false),
			createTestReceipt("receipt-2", true),
			createTestReceipt("receipt-3", false),
		}

		// Store all receipts
		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			for _, receipt := range receipts {
				if err := StoreReceipt(tx, receipt); err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		// Retrieve all receipts and verify
		err = db.View(context.Background(), func(tx kv.Tx) error {
			for _, originalReceipt := range receipts {
				var retrievedReceipt *TestReceipt
				var err error
				txHash := originalReceipt.TxHash()
				retrievedReceipt, err = GetReceipt(tx, txHash[:], &TestReceipt{})
				if err != nil {
					return err
				}

				assert.Equal(t, originalReceipt.Hash, retrievedReceipt.Hash)
				assert.Equal(t, originalReceipt.Data, retrievedReceipt.Data)
				assert.Equal(t, originalReceipt.Failed, retrievedReceipt.Failed)
			}
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("get non-existent receipt", func(t *testing.T) {
		nonExistentHash := sha256.Sum256([]byte("non-existent"))

		err := db.View(context.Background(), func(tx kv.Tx) error {
			var receipt TestReceipt
			_, err := GetReceipt(tx, nonExistentHash[:], &receipt)
			// Should return an error for non-existent receipt
			assert.Error(t, err)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("store receipt with marshal error", func(t *testing.T) {
		// Create a receipt that will fail to marshal
		badReceipt := &BadTestReceipt{
			Hash: sha256.Sum256([]byte("bad-receipt")),
		}

		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, badReceipt)
		})
		// Should return an error due to marshal failure
		assert.Error(t, err)
	})

	t.Run("get receipt with unmarshal error", func(t *testing.T) {
		// Store valid data but try to unmarshal into bad receipt
		receipt := createTestReceipt("valid-receipt", false)

		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(t, err)

		// Try to retrieve into a bad receipt type
		err = db.View(context.Background(), func(tx kv.Tx) error {
			var badReceipt BadTestReceipt
			txHash := receipt.TxHash()
			_, err := GetReceipt(tx, txHash[:], &badReceipt)
			// Should return an error due to unmarshal failure
			assert.Error(t, err)
			return nil
		})
		require.NoError(t, err)
	})
}

// BadTestReceipt is a receipt type that fails to marshal/unmarshal for testing error cases
type BadTestReceipt struct {
	Hash [32]byte
}

func (r *BadTestReceipt) TxHash() [32]byte {
	return r.Hash
}

func (r *BadTestReceipt) Status() apptypes.TxReceiptStatus {
	return apptypes.ReceiptFailed
}

func (r *BadTestReceipt) Marshal() ([]byte, error) {
	return nil, assert.AnError // Always return an error
}

func (r *BadTestReceipt) Unmarshal(data []byte) error {
	return assert.AnError // Always return an error
}

func TestReceiptBatchOperations(t *testing.T) {
	// Test storing multiple receipts in a single transaction (like in the real use case)
	db := memdb.NewTestDB(t)
	defer db.Close()

	// Create tables
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.CreateBucket(ReceiptBucket)
	})
	require.NoError(t, err)

	t.Run("batch store receipts", func(t *testing.T) {
		receipts := make([]*TestReceipt, 100)
		for i := 0; i < 100; i++ {
			receipts[i] = createTestReceipt(string(rune('a'+i%26))+string(rune('0'+i%10)), i%3 == 0)
		}

		// Store all receipts in a single transaction
		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			for _, receipt := range receipts {
				if err := StoreReceipt(tx, receipt); err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		// Verify all receipts can be retrieved
		err = db.View(context.Background(), func(tx kv.Tx) error {
			for _, originalReceipt := range receipts {
				var retrievedReceipt *TestReceipt
				var err error
				txHash := originalReceipt.TxHash()
				retrievedReceipt, err = GetReceipt(tx, txHash[:], &TestReceipt{})
				if err != nil {
					return err
				}

				assert.Equal(t, originalReceipt.Data, retrievedReceipt.Data)
				assert.Equal(t, originalReceipt.Failed, retrievedReceipt.Failed)
			}
			return nil
		})
		require.NoError(t, err)
	})
}

func BenchmarkStoreReceipt(b *testing.B) {
	db := memdb.NewTestDB(b)
	defer db.Close()

	// Create tables
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.CreateBucket(ReceiptBucket)
	})
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			// Create a unique receipt for each iteration
			testReceipt := createTestReceipt("benchmark-receipt-"+string(rune(i)), false)
			return StoreReceipt(tx, testReceipt)
		})
		require.NoError(b, err)
	}
}

func BenchmarkGetReceipt(b *testing.B) {
	db := memdb.NewTestDB(b)
	defer db.Close()

	// Create tables
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.CreateBucket(ReceiptBucket)
	})
	require.NoError(b, err)

	receipt := createTestReceipt("benchmark-receipt", false)

	// Store the receipt first
	err = db.Update(context.Background(), func(tx kv.RwTx) error {
		return StoreReceipt(tx, receipt)
	})
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := db.View(context.Background(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()
			_, err := GetReceipt(tx, txHash[:], &TestReceipt{})
			return err
		})
		require.NoError(b, err)
	}
}
