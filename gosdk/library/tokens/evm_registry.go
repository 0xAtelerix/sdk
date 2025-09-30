// tokens/evm_registry.go

package tokens

import (
	"math/big"
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

	DefaultEvmTransfers = BuildDefaultEvmTransferRegistry()
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

func BuildDefaultEvmTransferRegistry() *Registry[EvmTransfer] {
	reg := NewRegistry[EvmTransfer]()

	// ERC-20 Transfer: 3 topics (sig, from, to), value in data
	Register[erc20Transfer](reg, SigTransfer, ERC20TransferABI, "Transfer",
		func(ev erc20Transfer, m Meta) ([]EvmTransfer, error) {
			return []EvmTransfer{{
				Mint:      m.Contract.Hex(),
				FromOwner: ev.From.Hex(),
				ToOwner:   ev.To.Hex(),
				Amount:    new(big.Int).Set(ev.Value),
				Decimals:  0, // enrich later via decimals()
				Balances: EthereumBalances{
					Standard: ERC20,
					TxHash:   m.TxHash,
					LogIndex: m.LogIndex,
				},
			}}, nil
		},
	)

	// ERC-721 Transfer: 4 topics (sig, from, to, tokenId), no data
	Register[erc721Transfer](reg, SigTransfer, ERC721TransferABI, "Transfer",
		func(ev erc721Transfer, m Meta) ([]EvmTransfer, error) {
			return []EvmTransfer{{
				Mint:      m.Contract.Hex(),
				FromOwner: ev.From.Hex(),
				ToOwner:   ev.To.Hex(),
				Amount:    big.NewInt(1), // NFT convention
				Decimals:  0,
				Balances: EthereumBalances{
					Standard: ERC721,
					TokenID:  new(big.Int).Set(ev.TokenID),
					TxHash:   m.TxHash,
					LogIndex: m.LogIndex,
				},
			}}, nil
		},
	)

	// ERC-1155 TransferSingle
	Register[erc1155Single](reg, SigTransferSingle, ERC1155ABI, "TransferSingle",
		func(ev erc1155Single, m Meta) ([]EvmTransfer, error) {
			return []EvmTransfer{{
				Mint:      m.Contract.Hex(),
				FromOwner: ev.From.Hex(),
				ToOwner:   ev.To.Hex(),
				Amount:    new(big.Int).Set(ev.Value),
				Decimals:  0,
				Balances: EthereumBalances{
					Standard: ERC1155,
					TokenID:  new(big.Int).Set(ev.ID),
					TxHash:   m.TxHash,
					LogIndex: m.LogIndex,
				},
			}}, nil
		},
	)

	// ERC-1155 TransferBatch → one row per (id,value)
	Register[erc1155Batch](reg, SigTransferBatch, ERC1155ABI, "TransferBatch",
		func(ev erc1155Batch, m Meta) ([]EvmTransfer, error) {
			out := make([]EvmTransfer, 0, len(ev.IDs))
			for i := range ev.IDs {
				out = append(out, EvmTransfer{
					Mint:      m.Contract.Hex(),
					FromOwner: ev.From.Hex(),
					ToOwner:   ev.To.Hex(),
					Amount:    new(big.Int).Set(ev.Values[i]),
					Decimals:  0,
					Balances: EthereumBalances{
						Standard: ERC1155,
						TokenID:  new(big.Int).Set(ev.IDs[i]),
						IDs:      ev.IDs,    // keep full arrays if you like
						Values:   ev.Values, // keep full arrays if you like
						TxHash:   m.TxHash,
						LogIndex: m.LogIndex,
					},
				})
			}

			return out, nil
		},
	)

	return reg
}
