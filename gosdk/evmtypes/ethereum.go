package evmtypes

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type EthereumCustomFields struct {
	WithdrawalsRoot       *common.Hash `json:"withdrawalsRoot,omitempty" rlp:"optional"`
	BlobGasUsed           *uint64      `json:"blobGasUsed,omitempty" rlp:"optional"`
	ExcessBlobGas         *uint64      `json:"excessBlobGas,omitempty" rlp:"optional"`
	ParentBeaconBlockRoot *common.Hash `json:"parentBeaconBlockRoot,omitempty" rlp:"optional"`
	RequestsHash          *common.Hash `json:"requestsHash,omitempty" rlp:"optional"`
}

func (e *EthereumCustomFields) GetField(name string) (any, error) {
	if e == nil {
		return nil, errors.New("nil receiver")
	}

	f := reflect.ValueOf(e).
		Elem().
		FieldByName(name)

	if !f.IsValid() {
		return nil, fmt.Errorf("unknown field %q", name)
	}

	if f.Kind() == reflect.Ptr && f.IsNil() {
		return nil, nil
	}

	return f.Interface(), nil
}

func NewFromEthereumHeader(h *types.Header) *Header[EthereumCustomFields] {
	evmHeader := &Header[EthereumCustomFields]{
		ParentHash:       h.ParentHash,
		Sha3Uncles:       h.UncleHash,
		Miner:            h.Coinbase,
		StateRoot:        h.Root,
		TransactionsRoot: h.TxHash,
		ReceiptsRoot:     h.ReceiptHash,
		LogsBloom:        h.Bloom,
		GasLimit:         hexutil.Uint64(h.GasLimit),
		GasUsed:          hexutil.Uint64(h.GasUsed),
		Time:             hexutil.Uint64(h.Time),
		ExtraData:        h.Extra,
		MixHash:          h.MixDigest,
		Nonce:            h.Nonce,
	}

	// Handle Number
	if h.Number != nil {
		evmHeader.Number = (*hexutil.Big)(big.NewInt(0).Set(h.Number))
	} else {
		evmHeader.Number = (*hexutil.Big)(big.NewInt(0))
	}

	// Handle Difficulty (can be nil for PoS chains)
	if h.Difficulty != nil {
		evmHeader.Difficulty = (*hexutil.Big)(big.NewInt(0).Set(h.Difficulty))
	} else {
		evmHeader.Difficulty = (*hexutil.Big)(big.NewInt(0))
	}

	// Handle EIP-1559 BaseFee
	if h.BaseFee != nil {
		evmHeader.BaseFeePerGas = (*hexutil.Big)(big.NewInt(0).Set(h.BaseFee))
	}

	// Handle post-Shanghai/Dencun fields from Raw JSON if available
	// WithdrawalsRoot (EIP-4895)
	if h.WithdrawalsHash != nil {
		hash := *h.WithdrawalsHash
		evmHeader.Raw.WithdrawalsRoot = &hash
	}

	// BlobGasUsed (EIP-4844)
	if h.BlobGasUsed != nil {
		hash := *h.BlobGasUsed
		evmHeader.Raw.BlobGasUsed = &hash
	}

	// ExcessBlobGas (EIP-4844)
	if h.ExcessBlobGas != nil {
		hash := *h.ExcessBlobGas
		evmHeader.Raw.ExcessBlobGas = &hash
	}

	// ParentBeaconBlockRoot (EIP-4788)
	if h.ParentBeaconRoot != nil {
		hash := *h.ParentBeaconRoot
		evmHeader.Raw.ParentBeaconBlockRoot = &hash
	}

	// RequestsHash (EIP-7685 - Pectra upgrade)
	if h.RequestsHash != nil {
		hash := *h.RequestsHash
		evmHeader.Raw.RequestsHash = &hash
	}

	return evmHeader
}
