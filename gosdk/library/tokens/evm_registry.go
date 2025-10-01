// tokens/evm_registry.go

package tokens

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

// Token standards we emit
type Standard string

const (
	ERC20   Standard = "ERC20"
	ERC721  Standard = "ERC721"
	ERC1155 Standard = "ERC1155"
)

// Canonical event signatures (topic[0])
//
//nolint:gochecknoglobals // there's no way to do the same with functions
var (
	SigTransfer       = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	SigTransferSingle = crypto.Keccak256Hash(
		[]byte("TransferSingle(address,address,address,uint256,uint256)"),
	)
	SigTransferBatch = crypto.Keccak256Hash(
		[]byte("TransferBatch(address,address,address,uint256[],uint256[])"),
	)
)

// Separate ABIs so both ERC-20 and ERC-721 can be named "Transfer"
//
//nolint:gochecknoglobals // there's no way to do the same with functions
var (
	ERC20TransferABI  abi.ABI // Transfer(from indexed, to indexed, value non-indexed)
	ERC721TransferABI abi.ABI // Transfer(from indexed, to indexed, tokenId indexed)
	ERC1155ABI        abi.ABI // TransferSingle, TransferBatch
)

func mustABI(json string) abi.ABI {
	a, err := abi.JSON(strings.NewReader(json))
	if err != nil {
		panic(err)
	}

	return a
}

func init() {
	ERC20TransferABI = mustABI(`[
	  { "type":"event","name":"Transfer",
	    "inputs":[
	      {"indexed":true,"name":"from","type":"address"},
	      {"indexed":true,"name":"to","type":"address"},
	      {"indexed":false,"name":"value","type":"uint256"}
	    ]
	  }
	]`)

	ERC721TransferABI = mustABI(`[
	  { "type":"event","name":"Transfer",
	    "inputs":[
	      {"indexed":true,"name":"from","type":"address"},
	      {"indexed":true,"name":"to","type":"address"},
	      {"indexed":true,"name":"tokenId","type":"uint256"}
	    ]
	  }
	]`)

	ERC1155ABI = mustABI(`[
	  { "type":"event","name":"TransferSingle",
	    "inputs":[
	      {"indexed":true,"name":"operator","type":"address"},
	      {"indexed":true,"name":"from","type":"address"},
	      {"indexed":true,"name":"to","type":"address"},
	      {"indexed":false,"name":"id","type":"uint256"},
	      {"indexed":false,"name":"value","type":"uint256"}
	    ]
	  },
	  { "type":"event","name":"TransferBatch",
	    "inputs":[
	      {"indexed":true,"name":"operator","type":"address"},
	      {"indexed":true,"name":"from","type":"address"},
	      {"indexed":true,"name":"to","type":"address"},
	      {"indexed":false,"name":"ids","type":"uint256[]"},
	      {"indexed":false,"name":"values","type":"uint256[]"}
	    ]
	  }
	]`)
}
