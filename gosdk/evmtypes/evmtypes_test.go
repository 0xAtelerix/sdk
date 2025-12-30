package evmtypes

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeader_GetCustomField(t *testing.T) {
	t.Parallel()

	t.Run("returns field when present", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(
			`{"number":"0x100","hash":"0x1234","customField":"customValue","l1BlockNumber":"0x999"}`,
		)
		header := &Header[json.RawMessage]{
			Number: (*hexutil.Big)(big.NewInt(256)),
			Raw:    raw,
		}

		val, err := header.GetCustomField("customField")
		require.NoError(t, err)
		assert.Equal(t, "customValue", val)

		// Test chain-specific field (like Arbitrum's l1BlockNumber)
		val, err = header.GetCustomField("l1BlockNumber")
		require.NoError(t, err)
		assert.Equal(t, "0x999", val)
	})

	t.Run("returns error when field not found", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"number":"0x100"}`)
		header := &Header[json.RawMessage]{Raw: raw}

		_, err := header.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		t.Parallel()

		header := &Header[json.RawMessage]{}

		_, err := header.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrEmptyCustomField)
	})
}

func TestBlock_GetCustomField(t *testing.T) {
	t.Parallel()

	t.Run("returns field when present", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"withdrawalsRoot":"0xabc123","blobGasUsed":"0x20000"}`)
		block := &Block[json.RawMessage]{Raw: raw}

		val, err := block.GetCustomField("withdrawalsRoot")
		require.NoError(t, err)
		assert.Equal(t, "0xabc123", val)

		val, err = block.GetCustomField("blobGasUsed")
		require.NoError(t, err)
		assert.Equal(t, "0x20000", val)
	})

	t.Run("returns error when field not found", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"number":"0x100"}`)
		block := &Block[json.RawMessage]{Raw: raw}

		_, err := block.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		t.Parallel()

		block := &Block[json.RawMessage]{}

		_, err := block.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrEmptyCustomField)
	})
}

func TestReceipt_GetCustomField(t *testing.T) {
	t.Parallel()

	t.Run("returns field when present", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"effectiveGasPrice":"0x3b9aca00","l1Fee":"0x12345"}`)
		receipt := &Receipt[json.RawMessage]{Raw: raw}

		val, err := receipt.GetCustomField("effectiveGasPrice")
		require.NoError(t, err)
		assert.Equal(t, "0x3b9aca00", val)

		// Test L2-specific field (like Optimism's l1Fee)
		val, err = receipt.GetCustomField("l1Fee")
		require.NoError(t, err)
		assert.Equal(t, "0x12345", val)
	})

	t.Run("returns error when field not found", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"status":"0x1"}`)
		receipt := &Receipt[json.RawMessage]{Raw: raw}

		_, err := receipt.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		t.Parallel()

		receipt := &Receipt[json.RawMessage]{}

		_, err := receipt.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrEmptyCustomField)
	})
}

func TestTransaction_GetCustomField(t *testing.T) {
	t.Parallel()

	t.Run("returns field when present", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"maxFeePerBlobGas":"0x100","accessList":[]}`)
		tx := &Transaction[json.RawMessage]{Raw: raw}

		val, err := tx.GetCustomField("maxFeePerBlobGas")
		require.NoError(t, err)
		assert.Equal(t, "0x100", val)

		val, err = tx.GetCustomField("accessList")
		require.NoError(t, err)
		assert.Equal(t, []any{}, val)
	})

	t.Run("returns error when field not found", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"hash":"0x123"}`)
		tx := &Transaction[json.RawMessage]{Raw: raw}

		_, err := tx.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		t.Parallel()

		tx := &Transaction[json.RawMessage]{}

		_, err := tx.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrEmptyCustomField)
	})
}

func TestNewHeader(t *testing.T) {
	t.Parallel()

	header := NewHeader[json.RawMessage](12345)

	require.NotNil(t, header.Number)
	assert.Equal(t, uint64(12345), header.Number.ToInt().Uint64())
}

func TestNewBlock(t *testing.T) {
	t.Parallel()

	t.Run("with header and transactions", func(t *testing.T) {
		t.Parallel()

		header := NewHeader[json.RawMessage](100)
		txs := []Transaction[json.RawMessage]{
			{Hash: common.HexToHash("0x123")},
			{Hash: common.HexToHash("0x456")},
		}

		block := NewBlock[json.RawMessage](header, txs)

		assert.Equal(t, uint64(100), block.Number.ToInt().Uint64())
		assert.Len(t, block.Transactions, 2)
	})

	t.Run("with nil header", func(t *testing.T) {
		t.Parallel()

		block := NewBlock[json.RawMessage](nil, nil)

		assert.NotNil(t, block)
		assert.Empty(t, block.Transactions)
	})

	t.Run("with nil transactions", func(t *testing.T) {
		t.Parallel()

		header := NewHeader[json.RawMessage](50)
		block := NewBlock(header, nil)

		assert.NotNil(t, block.Transactions)
		assert.Empty(t, block.Transactions)
	})
}

func TestNewReceipt(t *testing.T) {
	t.Parallel()

	txHash := common.HexToHash("0xabc123")
	receipt := NewReceipt[json.RawMessage](txHash, 1, 21000)

	assert.Equal(t, txHash, receipt.TxHash)
	assert.Equal(t, hexutil.Uint64(1), receipt.Status)
	assert.Equal(t, hexutil.Uint64(21000), receipt.GasUsed)
}

func TestNewTransaction(t *testing.T) {
	t.Parallel()

	hash := common.HexToHash("0xdef456")
	from := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f")

	tx := NewTransaction[json.RawMessage](hash, from)

	assert.Equal(t, hash, tx.Hash)
	assert.Equal(t, from, tx.From)
}

func TestHeader_ComputeHash(t *testing.T) {
	t.Parallel()

	// Test constants for empty trie hashes
	emptyUnclesHash := common.HexToHash(
		"0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
	)
	emptyTrieRoot := common.HexToHash(
		"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
	)

	t.Run("computes correct hash for basic header", func(t *testing.T) {
		t.Parallel()

		// Create a header and compute its hash
		header := NewHeader[json.RawMessage](12345)
		header.ParentHash = common.HexToHash(
			"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		)
		header.Sha3Uncles = emptyUnclesHash
		header.StateRoot = common.HexToHash("0xabcdef")
		header.TransactionsRoot = emptyTrieRoot
		header.ReceiptsRoot = emptyTrieRoot
		header.GasLimit = 30000000
		header.GasUsed = 21000
		header.Time = 1700000000
		header.Difficulty = (*hexutil.Big)(big.NewInt(1))

		computedHash := header.ComputeHash()

		// Hash should be deterministic
		assert.Equal(t, computedHash, header.ComputeHash())

		// Hash should not be zero
		assert.NotEqual(t, common.Hash{}, computedHash)
	})

	t.Run("verify hash returns true for matching hash", func(t *testing.T) {
		t.Parallel()

		header := NewHeader[json.RawMessage](100)
		header.ParentHash = common.HexToHash("0x1234")
		header.Sha3Uncles = emptyUnclesHash
		header.StateRoot = common.HexToHash("0xabcd")
		header.TransactionsRoot = emptyTrieRoot
		header.ReceiptsRoot = emptyTrieRoot
		header.Difficulty = (*hexutil.Big)(big.NewInt(0))

		// Set the hash to the computed value
		header.Hash = header.ComputeHash()

		assert.True(t, header.VerifyHash())
	})

	t.Run("verify hash returns false for mismatched hash", func(t *testing.T) {
		t.Parallel()

		header := NewHeader[json.RawMessage](100)
		header.ParentHash = common.HexToHash("0x1234")
		header.Sha3Uncles = emptyUnclesHash
		header.StateRoot = common.HexToHash("0xabcd")
		header.TransactionsRoot = emptyTrieRoot
		header.ReceiptsRoot = emptyTrieRoot
		header.Difficulty = (*hexutil.Big)(big.NewInt(0))

		// Set a wrong hash
		header.Hash = common.HexToHash("0xbadbadbadbadbadbadbadbadbadbadbad")

		assert.False(t, header.VerifyHash())
	})

	t.Run("handles EIP-1559 baseFee", func(t *testing.T) {
		t.Parallel()

		header := NewHeader[json.RawMessage](15000000)
		header.ParentHash = common.HexToHash("0x1234")
		header.Sha3Uncles = emptyUnclesHash
		header.StateRoot = common.HexToHash("0xabcd")
		header.TransactionsRoot = emptyTrieRoot
		header.ReceiptsRoot = emptyTrieRoot
		header.Difficulty = (*hexutil.Big)(big.NewInt(0))
		header.BaseFeePerGas = (*hexutil.Big)(big.NewInt(1000000000)) // 1 gwei

		computedHash := header.ComputeHash()
		assert.NotEqual(t, common.Hash{}, computedHash)

		// Changing baseFee should change hash
		header.BaseFeePerGas = (*hexutil.Big)(big.NewInt(2000000000))
		newHash := header.ComputeHash()
		assert.NotEqual(t, computedHash, newHash)
	})

	t.Run("handles EIP-7685 requestsHash (Pectra)", func(t *testing.T) {
		t.Parallel()

		header := NewHeader[json.RawMessage](20000000)
		header.ParentHash = common.HexToHash("0x1234")
		header.Sha3Uncles = emptyUnclesHash
		header.StateRoot = common.HexToHash("0xabcd")
		header.TransactionsRoot = emptyTrieRoot
		header.ReceiptsRoot = emptyTrieRoot
		header.Difficulty = (*hexutil.Big)(big.NewInt(0))
		header.BaseFeePerGas = (*hexutil.Big)(big.NewInt(1000000000))

		// Set Raw JSON with requestsHash field (simulates post-Pectra block)
		header.Raw = json.RawMessage(`{
			"withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
			"blobGasUsed": "0x0",
			"excessBlobGas": "0x0",
			"parentBeaconBlockRoot": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			"requestsHash": "0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		}`)

		computedHash := header.ComputeHash()
		assert.NotEqual(t, common.Hash{}, computedHash)

		// Changing requestsHash should change the computed hash
		header.Raw = json.RawMessage(`{
			"withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
			"blobGasUsed": "0x0",
			"excessBlobGas": "0x0",
			"parentBeaconBlockRoot": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			"requestsHash": "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
		}`)

		newHash := header.ComputeHash()
		assert.NotEqual(t, computedHash, newHash, "requestsHash should affect computed hash")
	})
}
