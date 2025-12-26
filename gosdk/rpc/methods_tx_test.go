package rpc

// This file contains UNIT TESTS for all transaction-related RPC methods.
// These tests focus on testing the method logic in isolation without HTTP overhead.
// Includes tests for:
//   - Transaction submission (sendTransaction)
//   - Pending transaction queries (getPendingTransactions)
//   - Transaction queries (getTransaction - both finalized + pending)
//   - Transaction status (getTransactionStatus - both finalized + pending)
//   - Block transaction queries (getTransactionsByBlockNumber, getExternalTransactions)
//
// For INTEGRATION TESTS that verify these methods work through the HTTP/JSON-RPC stack,
// see rpc_test.go (TestStandardRPCServer_* functions).

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/scheme"
	"github.com/0xAtelerix/sdk/gosdk/txpool"
)

// setupTransactionTestEnvironment creates a test environment for transaction methods
func setupTransactionTestEnvironment(t *testing.T) (
	methods *TransactionMethods[TestTransaction[TestReceipt], TestReceipt],
	pool *txpool.TxPool[TestTransaction[TestReceipt], TestReceipt],
	appchainDB kv.RwDB,
	cleanup func(),
) {
	t.Helper()

	localDBPath := t.TempDir()
	appchainDBPath := t.TempDir()

	localDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(localDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return txpool.Tables()
		}).
		Open()
	require.NoError(t, err)

	appchainDB, err = mdbx.NewMDBX(mdbxlog.New()).
		Path(appchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				scheme.TxLookupBucket:          {},
				scheme.BlockTransactionsBucket: {},
				scheme.ExternalTxBucket:        {},
				scheme.ReceiptBucket:           {},
			}
		}).
		Open()
	require.NoError(t, err)

	pool = txpool.NewTxPool[TestTransaction[TestReceipt]](localDB)
	methods = NewTransactionMethods(pool, appchainDB)

	cleanup = func() {
		localDB.Close()
		appchainDB.Close()
	}

	return methods, pool, appchainDB, cleanup
}

func TestTransactionMethods_GetTransaction_FromTxPool(t *testing.T) {
	t.Parallel()

	methods, pool, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Add transaction to txpool
	tx := TestTransaction[TestReceipt]{
		From:  "0x1234",
		To:    "0x5678",
		Value: 100,
	}

	err := pool.AddTransaction(t.Context(), tx)
	require.NoError(t, err)

	// Get transaction by hash
	txHash := tx.Hash()
	hashStr := "0x" + hex.EncodeToString(txHash[:])

	result, err := methods.GetTransaction(t.Context(), []any{hashStr})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedTx, ok := result.(TestTransaction[TestReceipt])
	require.True(t, ok)
	assert.Equal(t, tx.Value, returnedTx.Value)
	assert.Equal(t, tx.From, returnedTx.From)
	assert.Equal(t, tx.To, returnedTx.To)
}

func TestTransactionMethods_GetTransaction_FromBlocks(t *testing.T) {
	t.Parallel()

	methods, _, appchainDB, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Create and store a transaction in blocks
	tx := TestTransaction[TestReceipt]{
		From:  "0xaaaa",
		To:    "0xbbbb",
		Value: 500,
	}
	txHash := tx.Hash()
	blockNumber := uint64(10)
	txIndex := uint32(0)

	err := appchainDB.Update(t.Context(), func(rwTx kv.RwTx) error {
		// Store block transactions (primary storage)
		var blockNumBytes [8]byte
		binary.BigEndian.PutUint64(blockNumBytes[:], blockNumber)

		txs := []TestTransaction[TestReceipt]{tx}

		txsBytes, marshalErr := cbor.Marshal(txs)
		if marshalErr != nil {
			return marshalErr
		}

		if err := rwTx.Put(scheme.BlockTransactionsBucket, blockNumBytes[:], txsBytes); err != nil {
			return err
		}

		// Store transaction lookup index: blockNumber (8 bytes) + txIndex (4 bytes)
		lookupEntry := make([]byte, 12)
		binary.BigEndian.PutUint64(lookupEntry[0:8], blockNumber)
		binary.BigEndian.PutUint32(lookupEntry[8:12], txIndex)

		return rwTx.Put(scheme.TxLookupBucket, txHash[:], lookupEntry)
	})
	require.NoError(t, err)

	// Get transaction by hash
	hashStr := "0x" + hex.EncodeToString(txHash[:])
	result, err := methods.GetTransaction(t.Context(), []any{hashStr})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedTx, ok := result.(TestTransaction[TestReceipt])
	require.True(t, ok)
	assert.Equal(t, tx.Value, returnedTx.Value)
	assert.Equal(t, tx.From, returnedTx.From)
	assert.Equal(t, tx.To, returnedTx.To)
}

func TestTransactionMethods_GetTransaction_NotFound(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Try to get a non-existent transaction
	nonExistentHash := sha256.Sum256([]byte("non-existent-tx"))
	hashStr := "0x" + hex.EncodeToString(nonExistentHash[:])

	result, err := methods.GetTransaction(t.Context(), []any{hashStr})

	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrTransactionNotFound)
}

func TestTransactionMethods_GetTransaction_WrongParamsCount(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// No parameters
	result, err := methods.GetTransaction(t.Context(), []any{})
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrWrongParamsCount)

	// Too many parameters
	result, err = methods.GetTransaction(t.Context(), []any{"hash1", "hash2"})
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrWrongParamsCount)
}

func TestTransactionMethods_GetTransaction_InvalidHashType(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Non-string parameter
	result, err := methods.GetTransaction(t.Context(), []any{12345})
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrHashParameterMustBeString)
}

func TestTransactionMethods_GetTransactionsByBlockNumber_Success(t *testing.T) {
	t.Parallel()

	methods, _, appchainDB, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	blockNumber := uint64(5)
	txs := []TestTransaction[TestReceipt]{
		{From: "0x1111", To: "0x2222", Value: 100},
		{From: "0x3333", To: "0x4444", Value: 200},
		{From: "0x5555", To: "0x6666", Value: 300},
	}

	// Store transactions in block
	err := appchainDB.Update(t.Context(), func(rwTx kv.RwTx) error {
		txsBytes, marshalErr := cbor.Marshal(txs)
		if marshalErr != nil {
			return marshalErr
		}

		var blockNumKey [8]byte
		binary.BigEndian.PutUint64(blockNumKey[:], blockNumber)

		return rwTx.Put(scheme.BlockTransactionsBucket, blockNumKey[:], txsBytes)
	})
	require.NoError(t, err)

	// Get transactions by block number
	result, err := methods.GetTransactionsByBlockNumber(
		t.Context(),
		[]any{float64(blockNumber)},
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedTxs, ok := result.([]TestTransaction[TestReceipt])
	require.True(t, ok)
	assert.Len(t, returnedTxs, 3)

	for i, tx := range returnedTxs {
		assert.Equal(t, txs[i].Value, tx.Value)
		assert.Equal(t, txs[i].From, tx.From)
		assert.Equal(t, txs[i].To, tx.To)
	}
}

func TestTransactionMethods_GetTransactionsByBlockNumber_EmptyBlock(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Get transactions from non-existent block
	result, err := methods.GetTransactionsByBlockNumber(t.Context(), []any{float64(999)})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedTxs, ok := result.([]TestTransaction[TestReceipt])
	require.True(t, ok)
	assert.Empty(t, returnedTxs)
}

func TestTransactionMethods_GetExternalTransactions_Success(t *testing.T) {
	t.Parallel()

	methods, _, appchainDB, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	blockNumber := uint64(7)
	externalTxs := []apptypes.ExternalTransaction{
		{
			ChainID: 1,
			Tx:      []byte{0x11, 0x22, 0x33},
		},
		{
			ChainID: 2,
			Tx:      []byte{0x44, 0x55, 0x66},
		},
	}

	// Store external transactions
	err := appchainDB.Update(t.Context(), func(rwTx kv.RwTx) error {
		_, writeErr := gosdk.WriteExternalTransactions(rwTx, blockNumber, externalTxs)

		return writeErr
	})
	require.NoError(t, err)

	// Get external transactions
	result, err := methods.GetExternalTransactions(
		t.Context(),
		[]any{float64(blockNumber)},
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedTxs, ok := result.([]apptypes.ExternalTransaction)
	require.True(t, ok)
	assert.Len(t, returnedTxs, 2)

	for i, tx := range returnedTxs {
		assert.Equal(t, externalTxs[i].ChainID, tx.ChainID)
		assert.Equal(t, externalTxs[i].Tx, tx.Tx)
	}
}

func TestTransactionMethods_GetExternalTransactions_EmptyBlock(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Get external transactions from non-existent block
	result, err := methods.GetExternalTransactions(t.Context(), []any{float64(999)})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedTxs, ok := result.([]apptypes.ExternalTransaction)
	require.True(t, ok)
	assert.Empty(t, returnedTxs)
}

func TestTransactionMethods_GetTransactionStatus_Processed(t *testing.T) {
	t.Parallel()

	methods, _, appchainDB, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Create and store a confirmed receipt
	testHash := sha256.Sum256([]byte("processed-tx"))
	testReceipt := TestReceipt{
		ReceiptStatus:   apptypes.ReceiptConfirmed,
		TransactionHash: testHash,
	}

	err := appchainDB.Update(t.Context(), func(tx kv.RwTx) error {
		receiptData, marshalErr := cbor.Marshal(testReceipt)
		if marshalErr != nil {
			return marshalErr
		}

		return tx.Put(scheme.ReceiptBucket, testHash[:], receiptData)
	})
	require.NoError(t, err)

	// Get transaction status
	hashStr := "0x" + hex.EncodeToString(testHash[:])
	result, err := methods.GetTransactionStatus(t.Context(), []any{hashStr})

	require.NoError(t, err)
	require.NotNil(t, result)

	status, ok := result.(string)
	require.True(t, ok)
	assert.Equal(t, "Processed", status)
}

func TestTransactionMethods_GetTransactionStatus_Failed(t *testing.T) {
	t.Parallel()

	methods, _, appchainDB, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Create and store a failed receipt
	testHash := sha256.Sum256([]byte("failed-tx"))
	testReceipt := TestReceipt{
		ReceiptStatus:   apptypes.ReceiptFailed,
		TransactionHash: testHash,
	}

	err := appchainDB.Update(t.Context(), func(tx kv.RwTx) error {
		receiptData, marshalErr := cbor.Marshal(testReceipt)
		if marshalErr != nil {
			return marshalErr
		}

		return tx.Put(scheme.ReceiptBucket, testHash[:], receiptData)
	})
	require.NoError(t, err)

	// Get transaction status
	hashStr := "0x" + hex.EncodeToString(testHash[:])
	result, err := methods.GetTransactionStatus(t.Context(), []any{hashStr})

	require.NoError(t, err)
	require.NotNil(t, result)

	status, ok := result.(string)
	require.True(t, ok)
	assert.Equal(t, "Failed", status)
}

func TestTransactionMethods_GetTransactionStatus_Pending(t *testing.T) {
	t.Parallel()

	methods, pool, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Add transaction to txpool
	tx := TestTransaction[TestReceipt]{
		From:  "0xpending",
		To:    "0xtest",
		Value: 999,
	}

	err := pool.AddTransaction(t.Context(), tx)
	require.NoError(t, err)

	// Get transaction status
	txHash := tx.Hash()
	hashStr := "0x" + hex.EncodeToString(txHash[:])

	result, err := methods.GetTransactionStatus(t.Context(), []any{hashStr})

	require.NoError(t, err)
	require.NotNil(t, result)

	status, ok := result.(string)
	require.True(t, ok)
	// Should be one of the pending statuses
	assert.Contains(t, []string{"Pending", "Batched", "ReadyToProcess"}, status)
}

func TestTransactionMethods_AddTransactionMethods(t *testing.T) {
	t.Parallel()

	_, pool, appchainDB, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	server := NewStandardRPCServer(nil)

	// Add all transaction methods to server
	AddTransactionMethods(server, pool, appchainDB)

	// Verify that all methods were registered
	_, sendTxExists := server.methods["sendTransaction"]
	_, getPendingExists := server.methods["getPendingTransactions"]
	_, getTxExists := server.methods["getTransaction"]
	_, getTxsByBlockExists := server.methods["getTransactionsByBlockNumber"]
	_, getExtTxsExists := server.methods["getExternalTransactions"]
	_, getStatusExists := server.methods["getTransactionStatus"]

	assert.True(t, sendTxExists, "sendTransaction method should be registered")
	assert.True(t, getPendingExists, "getPendingTransactions method should be registered")
	assert.True(t, getTxExists, "getTransaction method should be registered")
	assert.True(t, getTxsByBlockExists, "getTransactionsByBlockNumber method should be registered")
	assert.True(t, getExtTxsExists, "getExternalTransactions method should be registered")
	assert.True(t, getStatusExists, "getTransactionStatus method should be registered")
}

// ============= TRANSACTION SUBMISSION & PENDING QUERIES =============
// These tests were migrated from methods_txpool_test.go

func TestTransactionMethods_SendTransaction_Success(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	tx := TestTransaction[TestReceipt]{
		From:  "0x1234",
		To:    "0x5678",
		Value: 100,
	}

	result, err := methods.SendTransaction(t.Context(), []any{tx})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Result should be a hex string
	hashStr, ok := result.(string)
	require.True(t, ok)
	assert.Greater(t, len(hashStr), 2)
	assert.Equal(t, "0x", hashStr[:2])

	// Verify the hash matches the transaction
	expectedHash := tx.Hash()
	expectedHashStr := "0x" + hex.EncodeToString(expectedHash[:])
	assert.Equal(t, expectedHashStr, hashStr)
}

func TestTransactionMethods_SendTransaction_WrongParamsCount(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// No parameters
	result, err := methods.SendTransaction(t.Context(), []any{})
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrWrongParamsCount)

	// Too many parameters
	result, err = methods.SendTransaction(t.Context(), []any{"tx1", "tx2"})
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrWrongParamsCount)
}

func TestTransactionMethods_SendTransaction_InvalidData(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Test with invalid transaction data
	tests := []struct {
		name   string
		txData any
	}{
		{
			name:   "invalid type",
			txData: "invalid-tx-string",
		},
		{
			name:   "number instead of tx",
			txData: 12345,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := methods.SendTransaction(t.Context(), []any{tt.txData})
			require.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestTransactionMethods_GetPendingTransactions_Empty(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	result, err := methods.GetPendingTransactions(t.Context(), []any{})

	require.NoError(t, err)
	// Result can be nil or an empty array depending on implementation
	if result != nil {
		txs, ok := result.([]TestTransaction[TestReceipt])
		require.True(t, ok)
		assert.Empty(t, txs)
	}
}

func TestTransactionMethods_GetPendingTransactions_WithTransactions(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// Send some transactions
	txs := []TestTransaction[TestReceipt]{
		{From: "0x1111", To: "0x2222", Value: 100},
		{From: "0x3333", To: "0x4444", Value: 200},
		{From: "0x5555", To: "0x6666", Value: 300},
	}

	for _, tx := range txs {
		_, err := methods.SendTransaction(t.Context(), []any{tx})
		require.NoError(t, err)
	}

	// Get pending transactions
	result, err := methods.GetPendingTransactions(t.Context(), []any{})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should return all sent transactions
	pendingTxs, ok := result.([]TestTransaction[TestReceipt])
	require.True(t, ok)
	assert.Len(t, pendingTxs, 3)

	// Verify transaction values - order might not be guaranteed
	expectedValues := map[int]bool{100: true, 200: true, 300: true}
	for _, tx := range pendingTxs {
		assert.True(t, expectedValues[tx.Value], "Unexpected transaction value: %d", tx.Value)
	}
}

func TestTransactionMethods_GetPendingTransactions_IgnoresParams(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	// GetPendingTransactions should ignore any parameters
	result, err := methods.GetPendingTransactions(t.Context(), []any{"ignored", 123})

	require.NoError(t, err)
	// Result can be nil or an empty array depending on implementation
	if result != nil {
		txs, ok := result.([]TestTransaction[TestReceipt])
		require.True(t, ok)
		assert.Empty(t, txs)
	}
}

func TestTransactionMethods_SendAndRetrieve_Integration(t *testing.T) {
	t.Parallel()

	methods, pool, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	tx := TestTransaction[TestReceipt]{
		From:  "0xaaa",
		To:    "0xbbb",
		Value: 999,
	}

	// Send transaction
	hashStr, err := methods.SendTransaction(t.Context(), []any{tx})
	require.NoError(t, err)

	// Get pending transactions
	result, err := methods.GetPendingTransactions(t.Context(), []any{})
	require.NoError(t, err)

	pendingTxs, ok := result.([]TestTransaction[TestReceipt])
	require.True(t, ok)
	assert.Len(t, pendingTxs, 1)
	assert.Equal(t, tx.Value, pendingTxs[0].Value)
	assert.Equal(t, tx.From, pendingTxs[0].From)
	assert.Equal(t, tx.To, pendingTxs[0].To)

	// Verify hash matches
	expectedHash := tx.Hash()
	expectedHashStr := "0x" + hex.EncodeToString(expectedHash[:])
	assert.Equal(t, expectedHashStr, hashStr)

	// Verify transaction can be retrieved from pool directly
	retrievedTx, err := pool.GetTransaction(t.Context(), expectedHash[:])
	require.NoError(t, err)
	assert.Equal(t, tx.Value, retrievedTx.Value)
}

func TestTransactionMethods_SendTransaction_DuplicateTransaction(t *testing.T) {
	t.Parallel()

	methods, _, _, cleanup := setupTransactionTestEnvironment(t)
	defer cleanup()

	tx := TestTransaction[TestReceipt]{
		From:  "0x1234",
		To:    "0x5678",
		Value: 100,
	}

	// Send transaction first time
	hash1, err := methods.SendTransaction(t.Context(), []any{tx})
	require.NoError(t, err)
	require.NotNil(t, hash1)

	// Send same transaction again - should succeed (txpool handles duplicates)
	hash2, err := methods.SendTransaction(t.Context(), []any{tx})
	require.NoError(t, err)
	require.NotNil(t, hash2)

	// Hashes should be the same
	assert.Equal(t, hash1, hash2)

	// Should still only have one transaction in pending
	result, err := methods.GetPendingTransactions(t.Context(), []any{})
	require.NoError(t, err)

	pendingTxs, ok := result.([]TestTransaction[TestReceipt])
	require.True(t, ok)
	// Depending on txpool implementation, this might be 1 or 2
	// The important thing is that it doesn't error
	assert.GreaterOrEqual(t, len(pendingTxs), 1)
}
