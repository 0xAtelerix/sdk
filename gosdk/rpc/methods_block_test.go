package rpc

// This file contains UNIT TESTS for block query RPC methods.
// These tests focus on testing the method logic in isolation without HTTP overhead.
// Includes tests for getBlock method with various scenarios including edge cases.
//
// For INTEGRATION TESTS that verify these methods work through the HTTP/JSON-RPC stack,
// see rpc_test.go (TestStandardRPCServer_* functions).

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk"
)

// setupBlockTestEnvironment creates a test environment for block methods
func setupBlockTestEnvironment(t *testing.T) (
	methods *BlockMethods[TestTransaction[TestReceipt], TestReceipt, TestBlock],
	appchainDB kv.RwDB,
	cleanup func(),
) {
	t.Helper()

	appchainDBPath := t.TempDir()

	appchainDB, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(appchainDBPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				gosdk.BlocksBucket: {},
				gosdk.ConfigBucket: {},
			}
		}).
		Open()
	require.NoError(t, err)

	methods = NewBlockMethods[TestTransaction[TestReceipt], TestReceipt, TestBlock](appchainDB)

	cleanup = func() {
		appchainDB.Close()
	}

	return methods, appchainDB, cleanup
}

func TestBlockMethods_GetBlock_Success(t *testing.T) {
	methods, appchainDB, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Create and store a test block
	blockNumber := uint64(10)
	testBlock := TestBlock{
		BlockNumber: blockNumber,
		BlockHash:   sha256.Sum256([]byte("test-block-10")),
		Root:        sha256.Sum256([]byte("test-root-10")),
	}

	// Store block in database using CBOR marshaling
	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		blockBytes, marshalErr := cbor.Marshal(testBlock)
		if marshalErr != nil {
			return marshalErr
		}

		return gosdk.WriteBlock(tx, blockNumber, blockBytes)
	})
	require.NoError(t, err)

	// Get the block
	result, err := methods.GetBlock(context.Background(), []any{float64(blockNumber)})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedBlock, ok := result.(TestBlock)
	require.True(t, ok)
	assert.Equal(t, testBlock.BlockNumber, returnedBlock.BlockNumber)
	assert.Equal(t, testBlock.BlockHash, returnedBlock.BlockHash)
	assert.Equal(t, testBlock.Root, returnedBlock.Root)
}

func TestBlockMethods_GetBlock_NotFound(t *testing.T) {
	methods, _, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Try to get a non-existent block
	result, err := methods.GetBlock(context.Background(), []any{float64(999)})

	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrBlockNotFound)
}

func TestBlockMethods_GetBlock_WrongParamsCount(t *testing.T) {
	methods, _, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Too many parameters
	result, err := methods.GetBlock(context.Background(), []any{float64(1), float64(2)})
	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrWrongParamsCount)
}

func TestBlockMethods_GetBlock_InvalidBlockNumber(t *testing.T) {
	methods, _, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name        string
		blockNumber any
		wantErr     bool
	}{
		{
			name:        "string instead of number",
			blockNumber: "not-a-number",
			wantErr:     true,
		},
		{
			name:        "negative number",
			blockNumber: float64(-1),
			wantErr:     true,
		},
		{
			name:        "nil",
			blockNumber: nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := methods.GetBlock(context.Background(), []any{tt.blockNumber})
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBlockMethods_GetBlock_MultipleBlocks(t *testing.T) {
	methods, appchainDB, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Create and store multiple blocks
	blocks := []TestBlock{
		{
			BlockNumber: 1,
			BlockHash:   sha256.Sum256([]byte("block-1")),
			Root:        sha256.Sum256([]byte("root-1")),
		},
		{
			BlockNumber: 2,
			BlockHash:   sha256.Sum256([]byte("block-2")),
			Root:        sha256.Sum256([]byte("root-2")),
		},
		{
			BlockNumber: 3,
			BlockHash:   sha256.Sum256([]byte("block-3")),
			Root:        sha256.Sum256([]byte("root-3")),
		},
	}

	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		for _, block := range blocks {
			blockBytes, marshalErr := cbor.Marshal(block)
			if marshalErr != nil {
				return marshalErr
			}

			if writeErr := gosdk.WriteBlock(tx, block.BlockNumber, blockBytes); writeErr != nil {
				return writeErr
			}
		}

		return nil
	})
	require.NoError(t, err)

	// Retrieve each block and verify
	for _, expectedBlock := range blocks {
		result, err := methods.GetBlock(
			context.Background(),
			[]any{float64(expectedBlock.BlockNumber)},
		)
		require.NoError(t, err)
		require.NotNil(t, result)

		returnedBlock, ok := result.(TestBlock)
		require.True(t, ok)
		assert.Equal(t, expectedBlock.BlockNumber, returnedBlock.BlockNumber)
		assert.Equal(t, expectedBlock.BlockHash, returnedBlock.BlockHash)
		assert.Equal(t, expectedBlock.Root, returnedBlock.Root)
	}
}

func TestBlockMethods_GetBlock_ZeroBlock(t *testing.T) {
	methods, appchainDB, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Create and store block 0 (genesis)
	genesisBlock := TestBlock{
		BlockNumber: 0,
		BlockHash:   sha256.Sum256([]byte("genesis")),
		Root:        sha256.Sum256([]byte("genesis-root")),
	}

	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		blockBytes, marshalErr := cbor.Marshal(genesisBlock)
		if marshalErr != nil {
			return marshalErr
		}

		return gosdk.WriteBlock(tx, 0, blockBytes)
	})
	require.NoError(t, err)

	// Get block 0
	result, err := methods.GetBlock(context.Background(), []any{float64(0)})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedBlock, ok := result.(TestBlock)
	require.True(t, ok)
	assert.Equal(t, uint64(0), returnedBlock.BlockNumber)
	assert.Equal(t, genesisBlock.BlockHash, returnedBlock.BlockHash)
}

func TestBlockMethods_GetBlock_LargeBlockNumber(t *testing.T) {
	methods, appchainDB, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Test with a very large block number
	largeBlockNumber := uint64(1000000)
	largeBlock := TestBlock{
		BlockNumber: largeBlockNumber,
		BlockHash:   sha256.Sum256([]byte("large-block")),
		Root:        sha256.Sum256([]byte("large-root")),
	}

	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		blockBytes, marshalErr := cbor.Marshal(largeBlock)
		if marshalErr != nil {
			return marshalErr
		}

		return gosdk.WriteBlock(tx, largeBlockNumber, blockBytes)
	})
	require.NoError(t, err)

	// Get the large block
	result, err := methods.GetBlock(context.Background(), []any{float64(largeBlockNumber)})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedBlock, ok := result.(TestBlock)
	require.True(t, ok)
	assert.Equal(t, largeBlockNumber, returnedBlock.BlockNumber)
}

func TestBlockMethods_AddBlockMethods(t *testing.T) {
	_, appchainDB, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	server := NewStandardRPCServer(nil)

	// Add block methods to server
	AddBlockMethods[TestTransaction[TestReceipt], TestReceipt, TestBlock](
		server,
		appchainDB,
	)

	// Verify that the method was registered
	_, exists := server.methods["getBlock"]
	assert.True(t, exists, "getBlock method should be registered")
}

func TestBlockMethods_GetBlock_CorruptedData(t *testing.T) {
	methods, appchainDB, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	blockNumber := uint64(50)

	// Store corrupted (non-CBOR) data
	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		corruptedData := []byte{0xFF, 0xFE, 0xFD, 0xFC}

		return gosdk.WriteBlock(tx, blockNumber, corruptedData)
	})
	require.NoError(t, err)

	// Try to get the block - should fail to unmarshal
	result, err := methods.GetBlock(context.Background(), []any{float64(blockNumber)})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestBlockMethods_GetBlock_LatestBlock(t *testing.T) {
	methods, appchainDB, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Create and store multiple blocks
	blocks := []TestBlock{
		{
			BlockNumber: 1,
			BlockHash:   sha256.Sum256([]byte("block-1")),
			Root:        sha256.Sum256([]byte("root-1")),
		},
		{
			BlockNumber: 5,
			BlockHash:   sha256.Sum256([]byte("block-5")),
			Root:        sha256.Sum256([]byte("root-5")),
		},
		{
			BlockNumber: 10,
			BlockHash:   sha256.Sum256([]byte("block-10")),
			Root:        sha256.Sum256([]byte("root-10")),
		},
	}

	// Store blocks and update last block pointer
	err := appchainDB.Update(context.Background(), func(tx kv.RwTx) error {
		for _, block := range blocks {
			blockBytes, marshalErr := cbor.Marshal(block)
			if marshalErr != nil {
				return marshalErr
			}

			if writeErr := gosdk.WriteBlock(tx, block.BlockNumber, blockBytes); writeErr != nil {
				return writeErr
			}
		}
		// Set the last block to block 10
		return gosdk.WriteLastBlock(tx, 10, blocks[2].BlockHash)
	})
	require.NoError(t, err)

	// Get latest block (no params)
	result, err := methods.GetBlock(context.Background(), []any{})

	require.NoError(t, err)
	require.NotNil(t, result)

	returnedBlock, ok := result.(TestBlock)
	require.True(t, ok)
	assert.Equal(t, uint64(10), returnedBlock.BlockNumber)
	assert.Equal(t, blocks[2].BlockHash, returnedBlock.BlockHash)
	assert.Equal(t, blocks[2].Root, returnedBlock.Root)
}

func TestBlockMethods_GetBlock_LatestBlock_NoBlocks(t *testing.T) {
	methods, _, cleanup := setupBlockTestEnvironment(t)
	defer cleanup()

	// Try to get latest block when no blocks exist
	result, err := methods.GetBlock(context.Background(), []any{})

	require.Error(t, err)
	assert.Nil(t, result)
	require.ErrorIs(t, err, ErrBlockNotFound)
}
