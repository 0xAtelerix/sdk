package receipt

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// TestReceipt is a mock implementation of apptypes.Receipt for testing
type TestReceipt struct {
	Hash     [32]byte `json:"hash"                cbor:"1,keyasint"`
	Data     string   `json:"data"                cbor:"2,keyasint"`
	ErrorMsg string   `json:"error_msg,omitempty" cbor:"3,keyasint,omitempty"`
}

func (r *TestReceipt) TxHash() [32]byte {
	return r.Hash
}

func (r *TestReceipt) Status() apptypes.TxReceiptStatus {
	if r.Error() != "" {
		return apptypes.ReceiptFailed
	}

	return apptypes.ReceiptConfirmed
}

func (r *TestReceipt) Error() string {
	return r.ErrorMsg
}

// Helper function to create a test receipt
func createTestReceipt(data string, errorMsg string) *TestReceipt {
	hash := sha256.Sum256([]byte(data))

	return &TestReceipt{
		Hash:     hash,
		Data:     data,
		ErrorMsg: errorMsg,
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
		receipt := createTestReceipt("test-receipt-1", "")

		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(t, err)

		// Retrieve the receipt
		var retrievedReceipt *TestReceipt

		err = db.View(context.Background(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()

			var getErr error

			retrievedReceipt, getErr = getReceipt(tx, txHash[:], &TestReceipt{})

			return getErr
		})
		require.NoError(t, err)

		// Verify the retrieved receipt matches the original
		assert.Equal(t, receipt.Hash, retrievedReceipt.Hash)
		assert.Equal(t, receipt.Data, retrievedReceipt.Data)
		assert.Equal(t, receipt.Error(), retrievedReceipt.Error())
		assert.Equal(t, receipt.Status(), retrievedReceipt.Status())
	})

	t.Run("store multiple receipts", func(t *testing.T) {
		receipts := []*TestReceipt{
			createTestReceipt("receipt-1", ""),
			createTestReceipt("receipt-2", "some error"),
			createTestReceipt("receipt-3", ""),
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
				var (
					retrievedReceipt *TestReceipt
					getErr           error
				)

				txHash := originalReceipt.TxHash()

				retrievedReceipt, getErr = getReceipt(tx, txHash[:], &TestReceipt{})
				if getErr != nil {
					return getErr
				}

				assert.Equal(t, originalReceipt.Hash, retrievedReceipt.Hash)
				assert.Equal(t, originalReceipt.Data, retrievedReceipt.Data)
				assert.Equal(t, originalReceipt.Error(), retrievedReceipt.Error())
			}

			return nil
		})
		require.NoError(t, err)
	})

	t.Run("get non-existent receipt", func(t *testing.T) {
		nonExistentHash := sha256.Sum256([]byte("non-existent"))

		err := db.View(context.Background(), func(tx kv.Tx) error {
			var receipt TestReceipt

			_, err := getReceipt(tx, nonExistentHash[:], &receipt)
			// Should return an error for non-existent receipt
			assert.Error(t, err)

			return nil
		})
		require.NoError(t, err)
	})
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

		for i := range 100 {
			errorMsg := ""
			if i%3 == 0 {
				errorMsg = fmt.Sprintf("error %d", i)
			}

			receipts[i] = createTestReceipt(string(rune('a'+i%26))+string(rune('0'+i%10)), errorMsg)
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
				var (
					retrievedReceipt *TestReceipt
					getErr           error
				)

				txHash := originalReceipt.TxHash()

				retrievedReceipt, getErr = getReceipt(tx, txHash[:], &TestReceipt{})
				if getErr != nil {
					return getErr
				}

				assert.Equal(t, originalReceipt.Data, retrievedReceipt.Data)
				assert.Equal(t, originalReceipt.Error(), retrievedReceipt.Error())
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

	for i := range b.N {
		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			// Create a unique receipt for each iteration
			testReceipt := createTestReceipt("benchmark-receipt-"+string(rune(i)), "")

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

	receipt := createTestReceipt("benchmark-receipt", "")

	// Store the receipt first
	err = db.Update(context.Background(), func(tx kv.RwTx) error {
		return StoreReceipt(tx, receipt)
	})
	require.NoError(b, err)

	b.ResetTimer()

	for range b.N {
		err := db.View(context.Background(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()
			_, err := getReceipt(tx, txHash[:], &TestReceipt{})

			return err
		})
		require.NoError(b, err)
	}
}

func TestReceiptErrorHandling(t *testing.T) {
	// Create in-memory database
	db := memdb.NewTestDB(t)
	defer db.Close()

	// Create tables
	err := db.Update(context.Background(), func(tx kv.RwTx) error {
		return tx.CreateBucket(ReceiptBucket)
	})
	require.NoError(t, err)

	t.Run("error message serialization", func(t *testing.T) {
		// Test that error messages with special characters serialize correctly
		specialErrorMsg := "error with unicode: \u6d4b\u8bd5 and symbols: !@#$%^&*()[]{}|;':\",./<>?"
		receipt := createTestReceipt("unicode-error", specialErrorMsg)

		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(t, err)

		// Retrieve and verify special characters are preserved
		err = db.View(context.Background(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()

			retrievedReceipt, getErr := getReceipt(tx, txHash[:], &TestReceipt{})
			if getErr != nil {
				return getErr
			}

			assert.Equal(t, specialErrorMsg, retrievedReceipt.Error())

			return nil
		})
		require.NoError(t, err)
	})

	t.Run("long error message", func(t *testing.T) {
		// Test with a very long error message
		longError := "this is a very long error message that simulates a detailed stack trace or " +
			"comprehensive error description that might occur in complex transaction processing scenarios " +
			"where multiple validation steps fail and detailed information needs to be preserved for debugging purposes"

		receipt := createTestReceipt("long-error", longError)

		err := db.Update(context.Background(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(t, err)

		// Retrieve and verify long error is preserved
		err = db.View(context.Background(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()

			retrievedReceipt, getErr := getReceipt(tx, txHash[:], &TestReceipt{})
			if getErr != nil {
				return getErr
			}

			assert.Equal(t, longError, retrievedReceipt.Error())
			assert.Greater(t, len(retrievedReceipt.Error()), 200, "Error message should be long")

			return nil
		})
		require.NoError(t, err)
	})
}

func TestErrorMethodContract(t *testing.T) {
	t.Run("error method contract for success", func(t *testing.T) {
		receipt := createTestReceipt("success", "")
		receipt.ErrorMsg = ""

		// For successful transactions, Error() should return empty string
		assert.Empty(t, receipt.Error())
		assert.Equal(t, apptypes.ReceiptConfirmed, receipt.Status())

		// Verify the contract: empty error means success
		assert.Empty(t, receipt.Error(), "Successful receipt should have empty error")
	})

	t.Run("error method contract for failure", func(t *testing.T) {
		receipt := createTestReceipt("failure", "transaction failed")

		// For failed transactions, Error() should return the error message
		assert.Equal(t, "transaction failed", receipt.Error())
		assert.Equal(t, apptypes.ReceiptFailed, receipt.Status())

		// Verify the contract: non-empty error indicates failure
		assert.NotEmpty(t, receipt.Error(), "Failed receipt should have non-empty error")
	})

	t.Run("error and status consistency", func(t *testing.T) {
		testCases := []struct {
			name           string
			errorMsg       string
			expectedStatus apptypes.TxReceiptStatus
		}{
			{"success_no_error", "", apptypes.ReceiptConfirmed},
			{"success_with_empty_error", "", apptypes.ReceiptConfirmed},
			{"failed_with_error", "some error", apptypes.ReceiptFailed},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				receipt := createTestReceipt(tc.name, tc.errorMsg)

				assert.Equal(t, tc.expectedStatus, receipt.Status())
				assert.Equal(t, tc.errorMsg, receipt.Error())
			})
		}
	})
}
