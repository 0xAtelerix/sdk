package evmtypes

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
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
	if h.Raw == nil {
		return nil, ErrRawJSONNotAvailable
	}

	var data map[string]any
	if err := json.Unmarshal(h.Raw, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw header: %w", err)
	}

	value, exists := data[fieldName]
	if !exists {
		return nil, fmt.Errorf("%w: %s in header", ErrFieldNotFound, fieldName)
	}

	return value, nil
}
