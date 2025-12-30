package receipt

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library/tests"
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
	t.Parallel()

	t.Run("successful store and retrieve", func(tr *testing.T) {
		tr.Parallel()

		db, cleanup := tests.TestDB(tr, ReceiptBucket)
		defer cleanup()

		receipt := createTestReceipt("test-receipt-1", "")

		caseErr := db.Update(tr.Context(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(tr, caseErr)

		// Retrieve the receipt
		var retrievedReceipt *TestReceipt

		caseErr = db.View(tr.Context(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()

			var getErr error

			retrievedReceipt, getErr = GetReceipt(tx, txHash[:], &TestReceipt{})

			return getErr
		})
		require.NoError(tr, caseErr)

		// Verify the retrieved receipt matches the original
		assert.Equal(tr, receipt.Hash, retrievedReceipt.Hash)
		assert.Equal(tr, receipt.Data, retrievedReceipt.Data)
		assert.Equal(tr, receipt.Error(), retrievedReceipt.Error())
		assert.Equal(tr, receipt.Status(), retrievedReceipt.Status())
	})

	t.Run("store multiple receipts", func(tr *testing.T) {
		tr.Parallel()

		db, cleanup := tests.TestDB(tr, ReceiptBucket)
		defer cleanup()

		receipts := []*TestReceipt{
			createTestReceipt("receipt-1", ""),
			createTestReceipt("receipt-2", "some error"),
			createTestReceipt("receipt-3", ""),
		}

		// Store all receipts
		caseErr := db.Update(tr.Context(), func(tx kv.RwTx) error {
			for _, receipt := range receipts {
				if dbErr := StoreReceipt(tx, receipt); dbErr != nil {
					return dbErr
				}
			}

			return nil
		})
		require.NoError(tr, caseErr)

		// Retrieve all receipts and verify
		caseErr = db.View(tr.Context(), func(tx kv.Tx) error {
			for _, originalReceipt := range receipts {
				var (
					retrievedReceipt *TestReceipt
					getErr           error
				)

				txHash := originalReceipt.TxHash()

				retrievedReceipt, getErr = GetReceipt(tx, txHash[:], &TestReceipt{})
				if getErr != nil {
					return getErr
				}

				assert.Equal(tr, originalReceipt.Hash, retrievedReceipt.Hash)
				assert.Equal(tr, originalReceipt.Data, retrievedReceipt.Data)
				assert.Equal(tr, originalReceipt.Error(), retrievedReceipt.Error())
			}

			return nil
		})

		require.NoError(tr, caseErr)
	})

	t.Run("get non-existent receipt", func(tr *testing.T) {
		tr.Parallel()

		db, cleanup := tests.TestDB(tr, ReceiptBucket)
		defer cleanup()

		nonExistentHash := sha256.Sum256([]byte("non-existent"))

		err := db.View(tr.Context(), func(tx kv.Tx) error {
			var receipt TestReceipt

			_, dbErr := GetReceipt(tx, nonExistentHash[:], &receipt)
			// Should return an error for non-existent receipt
			assert.Error(tr, dbErr)

			return nil
		})

		require.NoError(tr, err)
	})
}

func TestReceiptBatchOperations(t *testing.T) {
	t.Parallel()

	t.Run("batch store receipts", func(tr *testing.T) {
		tr.Parallel()

		db, cleanup := tests.TestDB(tr, ReceiptBucket)
		defer cleanup()

		receipts := make([]*TestReceipt, 100)

		for i := range 100 {
			errorMsg := ""
			if i%3 == 0 {
				errorMsg = fmt.Sprintf("error %d", i)
			}

			receipts[i] = createTestReceipt(
				//nolint:perfsprint // for test cases
				fmt.Sprint('a'+i%26)+fmt.Sprint('0'+i%10),
				errorMsg,
			)
		}

		// Store all receipts in a single transaction
		err := db.Update(tr.Context(), func(tx kv.RwTx) error {
			for _, receipt := range receipts {
				if dbErr := StoreReceipt(tx, receipt); dbErr != nil {
					return dbErr
				}
			}

			return nil
		})

		require.NoError(tr, err)

		// Verify all receipts can be retrieved
		err = db.View(tr.Context(), func(tx kv.Tx) error {
			for _, originalReceipt := range receipts {
				var (
					retrievedReceipt *TestReceipt
					getErr           error
				)

				txHash := originalReceipt.TxHash()

				retrievedReceipt, getErr = GetReceipt(tx, txHash[:], &TestReceipt{})
				if getErr != nil {
					return getErr
				}

				assert.Equal(tr, originalReceipt.Data, retrievedReceipt.Data)
				assert.Equal(tr, originalReceipt.Error(), retrievedReceipt.Error())
			}

			return nil
		})

		require.NoError(tr, err)
	})
}

func BenchmarkStoreReceipt(b *testing.B) {
	db := memdb.NewTestDB(b)
	defer db.Close()

	// Create tables
	err := db.Update(b.Context(), func(tx kv.RwTx) error {
		return tx.CreateBucket(ReceiptBucket)
	})
	require.NoError(b, err)

	b.ResetTimer()

	for i := range b.N {
		err = db.Update(b.Context(), func(tx kv.RwTx) error {
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
	err := db.Update(b.Context(), func(tx kv.RwTx) error {
		return tx.CreateBucket(ReceiptBucket)
	})
	require.NoError(b, err)

	receipt := createTestReceipt("benchmark-receipt", "")

	// Store the receipt first
	err = db.Update(b.Context(), func(tx kv.RwTx) error {
		return StoreReceipt(tx, receipt)
	})
	require.NoError(b, err)

	b.ResetTimer()

	for range b.N {
		err = db.View(b.Context(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()
			_, dbErr := GetReceipt(tx, txHash[:], &TestReceipt{})

			return dbErr
		})

		require.NoError(b, err)
	}
}

func TestReceiptErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("error message serialization", func(tr *testing.T) {
		tr.Parallel()

		db, cleanup := tests.TestDB(tr, ReceiptBucket)
		defer cleanup()

		// Test that error messages with special characters serialize correctly
		specialErrorMsg := "error with unicode: \u6d4b\u8bd5 and symbols: !@#$%^&*()[]{}|;':\",./<>?"
		receipt := createTestReceipt("unicode-error", specialErrorMsg)

		err := db.Update(tr.Context(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(tr, err)

		// Retrieve and verify special characters are preserved
		err = db.View(tr.Context(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()

			retrievedReceipt, getErr := GetReceipt(tx, txHash[:], &TestReceipt{})
			if getErr != nil {
				return getErr
			}

			assert.Equal(tr, specialErrorMsg, retrievedReceipt.Error())

			return nil
		})
		require.NoError(tr, err)
	})

	t.Run("long error message", func(tr *testing.T) {
		tr.Parallel()

		db, cleanup := tests.TestDB(tr, ReceiptBucket)
		defer cleanup()

		// Test with a very long error message
		longError := "this is a very long error message that simulates a detailed stack trace or " +
			"comprehensive error description that might occur in complex transaction processing scenarios " +
			"where multiple validation steps fail and detailed information needs to be preserved for debugging purposes"

		receipt := createTestReceipt("long-error", longError)

		err := db.Update(tr.Context(), func(tx kv.RwTx) error {
			return StoreReceipt(tx, receipt)
		})
		require.NoError(tr, err)

		// Retrieve and verify long error is preserved
		err = db.View(tr.Context(), func(tx kv.Tx) error {
			txHash := receipt.TxHash()

			retrievedReceipt, getErr := GetReceipt(tx, txHash[:], &TestReceipt{})
			if getErr != nil {
				return getErr
			}

			assert.Equal(tr, longError, retrievedReceipt.Error())
			assert.Greater(tr, len(retrievedReceipt.Error()), 200, "Error message should be long")

			return nil
		})
		require.NoError(tr, err)
	})
}

func TestErrorMethodContract(t *testing.T) {
	t.Parallel()

	t.Run("error method contract for success", func(tr *testing.T) {
		tr.Parallel()

		receipt := createTestReceipt("success", "")
		receipt.ErrorMsg = ""

		// For successful transactions, Error() should return empty string
		assert.Empty(tr, receipt.Error())
		assert.Equal(tr, apptypes.ReceiptConfirmed, receipt.Status())

		// Verify the contract: empty error means success
		assert.Empty(tr, receipt.Error(), "Successful receipt should have empty error")
	})

	t.Run("error method contract for failure", func(tr *testing.T) {
		tr.Parallel()

		receipt := createTestReceipt("failure", "transaction failed")

		// For failed transactions, Error() should return the error message
		assert.Equal(tr, "transaction failed", receipt.Error())
		assert.Equal(tr, apptypes.ReceiptFailed, receipt.Status())

		// Verify the contract: non-empty error indicates failure
		assert.NotEmpty(tr, receipt.Error(), "Failed receipt should have non-empty error")
	})

	t.Run("error and status consistency", func(tr *testing.T) {
		tr.Parallel()

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
			tr.Run(tc.name, func(tcr *testing.T) {
				tcr.Parallel()

				receipt := createTestReceipt(tc.name, tc.errorMsg)

				assert.Equal(tcr, tc.expectedStatus, receipt.Status())
				assert.Equal(tcr, tc.errorMsg, receipt.Error())
			})
		}
	})
}
