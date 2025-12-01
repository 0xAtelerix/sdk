package evmtypes

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goccy/go-json"
)

// Receipt contains standard EVM receipt fields.
type Receipt struct {
	Status            hexutil.Uint64  `json:"status"`
	CumulativeGasUsed hexutil.Uint64  `json:"cumulativeGasUsed"`
	Logs              []*types.Log    `json:"logs"`
	ContractAddress   *common.Address `json:"contractAddress"` // nil for non-contract-creation txs
	TxHash            common.Hash     `json:"transactionHash"`
	GasUsed           hexutil.Uint64  `json:"gasUsed"`
	EffectiveGasPrice *hexutil.Big    `json:"effectiveGasPrice"` // EIP-1559: baseFee + min(maxFee-baseFee, priorityFee)
	BlockNumber       hexutil.Uint64  `json:"blockNumber"`
	BlockHash         common.Hash     `json:"blockHash"`
	TransactionIndex  hexutil.Uint64  `json:"transactionIndex"`
	From              common.Address  `json:"from"`
	To                *common.Address `json:"to"`
	Type              hexutil.Uint64  `json:"type"`

	Raw json.RawMessage `json:"-"` // Set by fetcher for GetCustomField()
}

// NewReceipt creates a new Receipt with basic fields.
func NewReceipt(txHash common.Hash, status uint64, gasUsed uint64) *Receipt {
	return &Receipt{
		TxHash:  txHash,
		Status:  hexutil.Uint64(status),
		GasUsed: hexutil.Uint64(gasUsed),
	}
}

// GetCustomField extracts a chain-specific field from raw JSON.
func (r *Receipt) GetCustomField(fieldName string) (any, error) {
	return GetCustomFieldFromRaw(r.Raw, fieldName)
}
