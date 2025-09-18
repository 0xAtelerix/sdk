package tokens

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
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

//nolint:gochecknoglobals // private globals
var (
	sigERC20ERC721Transfer = crypto.Keccak256Hash([]byte(sigERC20ERC721TransferABI))

	sigERC1155Single = crypto.Keccak256Hash([]byte(sigERC1155SingleABI))
	sigERC1155Batch  = crypto.Keccak256Hash([]byte(sigERC1155BatchABI))

	transferSig       = crypto.Keccak256Hash([]byte(transferSigABI))
	transferSingleSig = crypto.Keccak256Hash([]byte(transferSingleSigABI))
	transferBatchSig  = crypto.Keccak256Hash([]byte(transferBatchSigABI))
)

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
