package tokens

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Standard string

const (
	ERC20   Standard = "ERC20"
	ERC721  Standard = "ERC721"
	ERC1155 Standard = "ERC1155"
)

const (
	sigERC20ERC721TransferABI = "Transfer(address,address,uint256)"
	sigERC1155SingleABI       = "TransferSingle(address,address,address,uint256,uint256)"
	sigERC1155BatchABI        = "TransferBatch(address,address,address,uint256[],uint256[])"

	transferSigABI = "Transfer(address,address,uint256)"

	// ERC-1155
	transferSingleSigABI = "TransferSingle(address,address,address,uint256,uint256)"
	transferBatchSigABI  = "TransferBatch(address,address,address,uint256[],uint256[])"
)

func sigERC20ERC721Transfer() common.Hash {
	return crypto.Keccak256Hash([]byte(sigERC20ERC721TransferABI))
}

func sigERC1155Single() common.Hash {
	return crypto.Keccak256Hash([]byte(sigERC1155SingleABI))
}

func sigERC1155Batch() common.Hash {
	return crypto.Keccak256Hash([]byte(sigERC1155BatchABI))
}

func transferSig() common.Hash {
	return crypto.Keccak256Hash([]byte(transferSigABI))
}

func transferSingleSig() common.Hash {
	return crypto.Keccak256Hash([]byte(transferSingleSigABI))
}

func transferBatchSig() common.Hash {
	return crypto.Keccak256Hash([]byte(transferBatchSigABI))
}

// ABI arg decoders for non-indexed data payloads
//
//nolint:gochecknoglobals // private globals
var (
	uint256T, _  = abi.NewType("uint256", "", nil)
	uint256sT, _ = abi.NewType("uint256[]", "", nil)

	erc20Data    = abi.Arguments{{Type: uint256T}}                     // value
	erc1155DataS = abi.Arguments{{Type: uint256T}, {Type: uint256T}}   // id, value
	erc1155DataB = abi.Arguments{{Type: uint256sT}, {Type: uint256sT}} // ids[], values[]
)
