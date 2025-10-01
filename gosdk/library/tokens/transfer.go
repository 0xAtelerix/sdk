package tokens

import (
	"math/big"

	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/ethereum/go-ethereum/common"
)

// Transfer is a logical token movement between owners.
type Transfer[B Balances] struct {
	Mint      string   // mint address
	FromOwner string   // owner of source token account
	ToOwner   string   // owner of dest token account
	Amount    *big.Int // raw amount in base units (no decimals applied)
	Decimals  uint8
	Balances  B // post-transaction balances, optionally pre-transaction balances
}

type Balances interface {
	SolanaBalances | EthereumBalances
}

type EthereumBalances struct {
	Standard Standard

	// Optional per-standard metadata:
	TokenID *big.Int   `json:"tokenId,omitempty"` // ERC-721 / ERC-1155 single
	IDs     []*big.Int `json:"ids,omitempty"`     // ERC-1155 batch
	Values  []*big.Int `json:"values,omitempty"`  // ERC-1155 batch

	// Provenance:
	TxHash   common.Hash `json:"txHash"`
	LogIndex uint        `json:"logIndex"`
}

type SolanaBalances struct {
	PreTokenBalances  []rpc.TransactionMetaTokenBalance
	PostTokenBalances []rpc.TransactionMetaTokenBalance
}

type EvmTransfer Transfer[EthereumBalances]

// Ensure EvmTransfer satisfies AppEvent.
func (e EvmTransfer) Kind() string {
	// You can choose any convention you like for the kind string.
	// This includes the ERC standard for convenience.
	return "evm.transfer." + string(e.Balances.Standard)
}

type SolTransfer Transfer[SolanaBalances]

// Ensure SolTransfer satisfies AppEvent.
func (s SolTransfer) Kind() string {
	return "svm.transfer." + s.Mint
}
