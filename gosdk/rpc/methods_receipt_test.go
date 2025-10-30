package rpc

// This file contains UNIT TESTS for receipt-related RPC methods.
// These tests focus on testing the method logic in isolation without HTTP overhead.
//
// For INTEGRATION TESTS that verify these methods work through the HTTP/JSON-RPC stack,
// see rpc_test.go (TestStandardRPCServer_* functions).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/receipt"
)

// setupReceiptTestEnvironment creates a test environment for receipt methods
func setupReceiptTestEnvironment(t *testing.T) (methods *ReceiptMethods[TestReceipt], appchainDB kv.RwDB, cleanup func()) {
	t.Helper()

	appchainDBPath := t.TempDir()

	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(appchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				receipt.ReceiptBucket: {},
			}
		}).
		Open()
	require.NoError(t, err)

	methods = NewReceiptMethods[TestReceipt](appchainDB)

	cleanup = func() {
		appchainDB.Close()
	}

	return methods, appchainDB, cleanup
}

func TestReceiptMethods_GetTransactionReceipt_Success(t *testing.T) {
	methods, appchainDB, cleanup := setupReceiptTestEnvironment(t)
	defer cleanup()

	// Create a test receipt
	testHash := sha256.Sum256([]byte("test-receipt-1"))
	testReceipt := TestReceipt{
		ReceiptStatus:   apptypes.ReceiptConfirmed,
		TransactionHash: testHash,
	}

	// Store receipt in database
	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		receiptData, marshalErr := cbor.Marshal(testReceipt)
		if marshalErr != nil {
			return marshalErr
		}
		return tx.Put(receipt.ReceiptBucket, testHash[:], receiptData)
	})
	require.NoError(t, err)

	// Test GetTransactionReceipt
	hashStr := "0x" + hex.EncodeToString(testHash[:])
	result, err := methods.GetTransactionReceipt(context.Background(), []any{hashStr})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedReceipt, ok := result.(TestReceipt)
	require.True(t, ok)
	assert.Equal(t, testReceipt.ReceiptStatus, returnedReceipt.ReceiptStatus)
	assert.Equal(t, testReceipt.TransactionHash, returnedReceipt.TransactionHash)
}

func TestReceiptMethods_GetTransactionReceipt_NotFound(t *testing.T) {
	methods, _, cleanup := setupReceiptTestEnvironment(t)
	defer cleanup()

	// Try to get a non-existent receipt
	nonExistentHash := sha256.Sum256([]byte("non-existent"))
	hashStr := "0x" + hex.EncodeToString(nonExistentHash[:])

	result, err := methods.GetTransactionReceipt(context.Background(), []any{hashStr})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrReceiptNotFound)
}

func TestReceiptMethods_GetTransactionReceipt_WrongParamsCount(t *testing.T) {
	methods, _, cleanup := setupReceiptTestEnvironment(t)
	defer cleanup()

	// Test with no parameters
	result, err := methods.GetTransactionReceipt(context.Background(), []any{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrGetTransactionReceiptRequires1Param)

	// Test with too many parameters
	result, err = methods.GetTransactionReceipt(context.Background(), []any{"hash1", "hash2"})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrGetTransactionReceiptRequires1Param)
}

func TestReceiptMethods_GetTransactionReceipt_InvalidHashType(t *testing.T) {
	methods, _, cleanup := setupReceiptTestEnvironment(t)
	defer cleanup()

	// Test with non-string parameter
	result, err := methods.GetTransactionReceipt(context.Background(), []any{12345})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrHashParameterMustBeString)
}

func TestReceiptMethods_GetTransactionReceipt_InvalidHashFormat(t *testing.T) {
	methods, _, cleanup := setupReceiptTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name     string
		hashStr  string
		wantErr  bool
		errorMsg string
	}{
		{
			name:     "missing 0x prefix",
			hashStr:  "1234567890abcdef",
			wantErr:  true,
			errorMsg: "invalid hash format",
		},
		{
			name:     "invalid hex characters",
			hashStr:  "0xZZZZ",
			wantErr:  true,
			errorMsg: "invalid hash format",
		},
		{
			name:     "empty hash",
			hashStr:  "",
			wantErr:  true,
			errorMsg: "invalid hash format",
		},
		{
			name:     "wrong length",
			hashStr:  "0x1234",
			wantErr:  true,
			errorMsg: "invalid hash format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := methods.GetTransactionReceipt(context.Background(), []any{tt.hashStr})
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.ErrorIs(t, err, ErrInvalidHashFormat)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReceiptMethods_GetTransactionReceipt_FailedStatus(t *testing.T) {
	methods, appchainDB, cleanup := setupReceiptTestEnvironment(t)
	defer cleanup()

	// Create a test receipt with failed status
	testHash := sha256.Sum256([]byte("test-receipt-failed"))
	testReceipt := TestReceipt{
		ReceiptStatus:   apptypes.ReceiptFailed,
		TransactionHash: testHash,
	}

	// Store receipt in database
	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		receiptData, marshalErr := cbor.Marshal(testReceipt)
		if marshalErr != nil {
			return marshalErr
		}
		return tx.Put(receipt.ReceiptBucket, testHash[:], receiptData)
	})
	require.NoError(t, err)

	// Test GetTransactionReceipt
	hashStr := "0x" + hex.EncodeToString(testHash[:])
	result, err := methods.GetTransactionReceipt(context.Background(), []any{hashStr})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedReceipt, ok := result.(TestReceipt)
	require.True(t, ok)
	assert.Equal(t, apptypes.ReceiptFailed, returnedReceipt.ReceiptStatus)
	assert.Equal(t, "transaction failed", returnedReceipt.Error())
}

func TestReceiptMethods_AddReceiptMethods(t *testing.T) {
	_, appchainDB, cleanup := setupReceiptTestEnvironment(t)
	defer cleanup()

	server := NewStandardRPCServer(nil)

	// Add receipt methods to server
	AddReceiptMethods[TestReceipt](server, appchainDB)

	// Verify that the method was registered
	_, exists := server.methods["getTransactionReceipt"]
	assert.True(t, exists, "getTransactionReceipt method should be registered")
}
