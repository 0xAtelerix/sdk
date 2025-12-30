package library

import (
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type EVMEvent struct {
	ChainID   apptypes.ChainType `cbor:"1,keyasint"`
	EventName string             `cbor:"2,keyasint"`
	Contract  EthereumAddress    `cbor:"3,keyasint"`
	ABI       abi.ABI            `cbor:"4,keyasint"`
}

func NewEVMEvent(
	chainID apptypes.ChainType,
	contract EthereumAddress,
	eventName string,
	contractABI abi.ABI,
) *EVMEvent {
	return &EVMEvent{
		ChainID:   chainID,
		Contract:  contract,
		EventName: eventName,
		ABI:       contractABI,
	}
}
