package evmtypes

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goccy/go-json"
)

// Header contains standard EVM header fields plus raw JSON for chain-specific fields.
type Header struct {
	Number           *hexutil.Big     `json:"number"`
	Hash             common.Hash      `json:"hash"`
	ParentHash       common.Hash      `json:"parentHash"`
	Nonce            types.BlockNonce `json:"nonce"`
	Sha3Uncles       common.Hash      `json:"sha3Uncles"`
	LogsBloom        types.Bloom      `json:"logsBloom"`
	TransactionsRoot common.Hash      `json:"transactionsRoot"`
	StateRoot        common.Hash      `json:"stateRoot"`
	ReceiptsRoot     common.Hash      `json:"receiptsRoot"`
	Miner            common.Address   `json:"miner"`
	Difficulty       *hexutil.Big     `json:"difficulty"`
	ExtraData        hexutil.Bytes    `json:"extraData"`
	GasLimit         hexutil.Uint64   `json:"gasLimit"`
	GasUsed          hexutil.Uint64   `json:"gasUsed"`
	Time             hexutil.Uint64   `json:"timestamp"`
	MixHash          common.Hash      `json:"mixHash"`
	BaseFeePerGas    *hexutil.Big     `json:"baseFeePerGas,omitempty"` // EIP-1559

	Raw json.RawMessage `json:"-"` // Set by fetcher for GetCustomField()
}

// NewHeader creates a new Header with the given block number.
func NewHeader(number uint64) *Header {
	return &Header{
		Number: (*hexutil.Big)(new(big.Int).SetUint64(number)),
	}
}

// GetCustomField extracts a chain-specific field from raw JSON.
func (h *Header) GetCustomField(fieldName string) (any, error) {
	return GetCustomFieldFromRaw(h.Raw, fieldName)
}

// ComputeHash computes the block hash from the header fields using RLP encoding and Keccak256.
// This works for all standard EVM chains including:
// - Ethereum (pre/post London, Shanghai, Dencun)
// - Arbitrum (extra fields encoded in mixHash)
// - Optimism/Base (standard header structure post-Bedrock)
// - Polygon, BSC, and other EVM-compatible chains
func (h *Header) ComputeHash() common.Hash {
	gethHeader := h.toGethHeader()

	return gethHeader.Hash()
}

// VerifyHash checks if the stored hash matches the computed hash from header fields.
// Returns true if the hash is valid, false otherwise.
func (h *Header) VerifyHash() bool {
	return h.ComputeHash() == h.Hash
}

// toGethHeader converts evmtypes.Header to go-ethereum types.Header.
// This handles all standard EVM header fields including optional EIP fields.
func (h *Header) toGethHeader() *types.Header {
	gethHeader := &types.Header{
		ParentHash:  h.ParentHash,
		UncleHash:   h.Sha3Uncles,
		Coinbase:    h.Miner,
		Root:        h.StateRoot,
		TxHash:      h.TransactionsRoot,
		ReceiptHash: h.ReceiptsRoot,
		Bloom:       h.LogsBloom,
		GasLimit:    uint64(h.GasLimit),
		GasUsed:     uint64(h.GasUsed),
		Time:        uint64(h.Time),
		Extra:       h.ExtraData,
		MixDigest:   h.MixHash,
		Nonce:       h.Nonce,
	}

	// Handle Number
	if h.Number != nil {
		gethHeader.Number = h.Number.ToInt()
	} else {
		gethHeader.Number = big.NewInt(0)
	}

	// Handle Difficulty (can be nil for PoS chains)
	if h.Difficulty != nil {
		gethHeader.Difficulty = h.Difficulty.ToInt()
	} else {
		gethHeader.Difficulty = big.NewInt(0)
	}

	// Handle EIP-1559 BaseFee
	if h.BaseFeePerGas != nil {
		gethHeader.BaseFee = h.BaseFeePerGas.ToInt()
	}

	// Handle post-Shanghai/Dencun fields from Raw JSON if available
	if h.Raw != nil {
		// WithdrawalsRoot (EIP-4895)
		if val, err := h.GetCustomField("withdrawalsRoot"); err == nil && val != nil {
			if hashStr, ok := val.(string); ok {
				hash := common.HexToHash(hashStr)
				gethHeader.WithdrawalsHash = &hash
			}
		}

		// BlobGasUsed (EIP-4844)
		if val, err := h.GetCustomField("blobGasUsed"); err == nil && val != nil {
			if blobGasUsed, ok := parseHexUint64(val); ok {
				gethHeader.BlobGasUsed = &blobGasUsed
			}
		}

		// ExcessBlobGas (EIP-4844)
		if val, err := h.GetCustomField("excessBlobGas"); err == nil && val != nil {
			if excessBlobGas, ok := parseHexUint64(val); ok {
				gethHeader.ExcessBlobGas = &excessBlobGas
			}
		}

		// ParentBeaconBlockRoot (EIP-4788)
		if val, err := h.GetCustomField("parentBeaconBlockRoot"); err == nil && val != nil {
			if hashStr, ok := val.(string); ok {
				hash := common.HexToHash(hashStr)
				gethHeader.ParentBeaconRoot = &hash
			}
		}
	}

	return gethHeader
}

// parseHexUint64 parses a hex string (with or without 0x prefix) to uint64.
// Returns the value and true on success, 0 and false on failure.
func parseHexUint64(val any) (uint64, bool) {
	hexStr, ok := val.(string)
	if !ok || len(hexStr) < 2 {
		return 0, false
	}

	// Handle "0x0" case
	if hexStr == "0x0" || hexStr == "0X0" {
		return 0, true
	}

	// Strip 0x prefix
	if len(hexStr) > 2 && (hexStr[:2] == "0x" || hexStr[:2] == "0X") {
		hexStr = hexStr[2:]
	}

	bigInt, success := new(big.Int).SetString(hexStr, 16)
	if !success {
		return 0, false
	}

	return bigInt.Uint64(), true
}
