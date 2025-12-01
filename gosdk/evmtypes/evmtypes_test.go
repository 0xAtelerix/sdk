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
	t.Run("returns field when present", func(t *testing.T) {
		raw := json.RawMessage(
			`{"number":"0x100","hash":"0x1234","customField":"customValue","l1BlockNumber":"0x999"}`,
		)
		header := &Header{
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
		raw := json.RawMessage(`{"number":"0x100"}`)
		header := &Header{Raw: raw}

		_, err := header.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		header := &Header{}

		_, err := header.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrRawJSONNotAvailable)
	})
}

func TestBlock_GetCustomField(t *testing.T) {
	t.Run("returns field when present", func(t *testing.T) {
		raw := json.RawMessage(`{"withdrawalsRoot":"0xabc123","blobGasUsed":"0x20000"}`)
		block := &Block{Raw: raw}

		val, err := block.GetCustomField("withdrawalsRoot")
		require.NoError(t, err)
		assert.Equal(t, "0xabc123", val)

		val, err = block.GetCustomField("blobGasUsed")
		require.NoError(t, err)
		assert.Equal(t, "0x20000", val)
	})

	t.Run("returns error when field not found", func(t *testing.T) {
		raw := json.RawMessage(`{"number":"0x100"}`)
		block := &Block{Raw: raw}

		_, err := block.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		block := &Block{}

		_, err := block.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrRawJSONNotAvailable)
	})
}

func TestReceipt_GetCustomField(t *testing.T) {
	t.Run("returns field when present", func(t *testing.T) {
		raw := json.RawMessage(`{"effectiveGasPrice":"0x3b9aca00","l1Fee":"0x12345"}`)
		receipt := &Receipt{Raw: raw}

		val, err := receipt.GetCustomField("effectiveGasPrice")
		require.NoError(t, err)
		assert.Equal(t, "0x3b9aca00", val)

		// Test L2-specific field (like Optimism's l1Fee)
		val, err = receipt.GetCustomField("l1Fee")
		require.NoError(t, err)
		assert.Equal(t, "0x12345", val)
	})

	t.Run("returns error when field not found", func(t *testing.T) {
		raw := json.RawMessage(`{"status":"0x1"}`)
		receipt := &Receipt{Raw: raw}

		_, err := receipt.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		receipt := &Receipt{}

		_, err := receipt.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrRawJSONNotAvailable)
	})
}

func TestTransaction_GetCustomField(t *testing.T) {
	t.Run("returns field when present", func(t *testing.T) {
		raw := json.RawMessage(`{"maxFeePerBlobGas":"0x100","accessList":[]}`)
		tx := &Transaction{Raw: raw}

		val, err := tx.GetCustomField("maxFeePerBlobGas")
		require.NoError(t, err)
		assert.Equal(t, "0x100", val)

		val, err = tx.GetCustomField("accessList")
		require.NoError(t, err)
		assert.Equal(t, []any{}, val)
	})

	t.Run("returns error when field not found", func(t *testing.T) {
		raw := json.RawMessage(`{"hash":"0x123"}`)
		tx := &Transaction{Raw: raw}

		_, err := tx.GetCustomField("nonexistent")
		assert.ErrorIs(t, err, ErrFieldNotFound)
	})

	t.Run("returns error when raw is nil", func(t *testing.T) {
		tx := &Transaction{}

		_, err := tx.GetCustomField("anyField")
		assert.ErrorIs(t, err, ErrRawJSONNotAvailable)
	})
}

func TestNewHeader(t *testing.T) {
	header := NewHeader(12345)

	require.NotNil(t, header.Number)
	assert.Equal(t, uint64(12345), header.Number.ToInt().Uint64())
}

func TestNewBlock(t *testing.T) {
	t.Run("with header and transactions", func(t *testing.T) {
		header := NewHeader(100)
		txs := []Transaction{
			{Hash: common.HexToHash("0x123")},
			{Hash: common.HexToHash("0x456")},
		}

		block := NewBlock(header, txs)

		assert.Equal(t, uint64(100), block.Number.ToInt().Uint64())
		assert.Len(t, block.Transactions, 2)
	})

	t.Run("with nil header", func(t *testing.T) {
		block := NewBlock(nil, nil)

		assert.NotNil(t, block)
		assert.Empty(t, block.Transactions)
	})

	t.Run("with nil transactions", func(t *testing.T) {
		header := NewHeader(50)
		block := NewBlock(header, nil)

		assert.NotNil(t, block.Transactions)
		assert.Empty(t, block.Transactions)
	})
}

func TestNewReceipt(t *testing.T) {
	txHash := common.HexToHash("0xabc123")
	receipt := NewReceipt(txHash, 1, 21000)

	assert.Equal(t, txHash, receipt.TxHash)
	assert.Equal(t, hexutil.Uint64(1), receipt.Status)
	assert.Equal(t, hexutil.Uint64(21000), receipt.GasUsed)
}

func TestNewTransaction(t *testing.T) {
	hash := common.HexToHash("0xdef456")
	from := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f")

	tx := NewTransaction(hash, from)

	assert.Equal(t, hash, tx.Hash)
	assert.Equal(t, from, tx.From)
}
