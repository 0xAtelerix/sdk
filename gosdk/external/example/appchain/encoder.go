package appchain

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// Errors for ABI decoding.
var (
	ErrDataTooShort = errors.New("data too short")
)

// TransferData represents a token transfer operation
type TransferData struct {
	To     common.Address `abi:"to"`
	Amount *big.Int       `abi:"amount"`
	Token  common.Address `abi:"token"`
}

// SwapData represents a DeFi swap operation
type SwapData struct {
	TokenIn      common.Address `abi:"tokenIn"`
	TokenOut     common.Address `abi:"tokenOut"`
	AmountIn     *big.Int       `abi:"amountIn"`
	MinAmountOut *big.Int       `abi:"minAmountOut"`
	Deadline     *big.Int       `abi:"deadline"`
	Slippage     uint16         `abi:"slippage"`
}

// Encoder handles ABI-based encoding for appchain operations
type Encoder struct {
	transferABI abi.ABI
	swapABI     abi.ABI
}

// NewEncoder creates a new encoder with predefined ABIs
func NewEncoder() *Encoder {
	// Transfer ABI - matches AppchainReceiver.sol
	transferABI, err := abi.JSON(strings.NewReader(`[{
		"name": "processTransfer",
		"type": "function",
		"inputs": [
			{"name": "to", "type": "address"},
			{"name": "amount", "type": "uint256"},
			{"name": "token", "type": "address"}
		]
	}]`))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse transfer ABI: %v", err))
	}

	// Swap ABI - for DeFi operations
	swapABI, err := abi.JSON(strings.NewReader(`[{
		"name": "processSwap", 
		"type": "function",
		"inputs": [
			{"name": "tokenIn", "type": "address"},
			{"name": "tokenOut", "type": "address"},
			{"name": "amountIn", "type": "uint256"},
			{"name": "minAmountOut", "type": "uint256"},
			{"name": "deadline", "type": "uint256"},
			{"name": "slippage", "type": "uint16"}
		]
	}]`))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse swap ABI: %v", err))
	}

	return &Encoder{
		transferABI: transferABI,
		swapABI:     swapABI,
	}
}

// EncodeTransfer encodes transfer data using ABI
func (e *Encoder) EncodeTransfer(data TransferData) ([]byte, error) {
	return e.transferABI.Pack("processTransfer", data.To, data.Amount, data.Token)
}

// DecodeTransfer decodes transfer data using ABI
func (e *Encoder) DecodeTransfer(data []byte) (TransferData, error) {
	// Remove method selector (first 4 bytes)
	if len(data) < 4 {
		return TransferData{}, ErrDataTooShort
	}

	values, err := e.transferABI.Methods["processTransfer"].Inputs.Unpack(data[4:])
	if err != nil {
		return TransferData{}, err
	}

	return TransferData{
		To:     values[0].(common.Address),
		Amount: values[1].(*big.Int),
		Token:  values[2].(common.Address),
	}, nil
}

// EncodeSwap encodes swap data using ABI
func (e *Encoder) EncodeSwap(data SwapData) ([]byte, error) {
	return e.swapABI.Pack("processSwap",
		data.TokenIn, data.TokenOut, data.AmountIn,
		data.MinAmountOut, data.Deadline, data.Slippage)
}

// DecodeSwap decodes swap data using ABI
func (e *Encoder) DecodeSwap(data []byte) (SwapData, error) {
	// Remove method selector (first 4 bytes)
	if len(data) < 4 {
		return SwapData{}, ErrDataTooShort
	}

	values, err := e.swapABI.Methods["processSwap"].Inputs.Unpack(data[4:])
	if err != nil {
		return SwapData{}, err
	}

	return SwapData{
		TokenIn:      values[0].(common.Address),
		TokenOut:     values[1].(common.Address),
		AmountIn:     values[2].(*big.Int),
		MinAmountOut: values[3].(*big.Int),
		Deadline:     values[4].(*big.Int),
		Slippage:     values[5].(uint16),
	}, nil
}
