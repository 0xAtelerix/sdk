package tokens

import (
	"math/big"

	"github.com/blocto/solana-go-sdk/rpc"
)

type (
	SolTransfer = Transfer[SolanaBalances]
	EvmTransfer = Transfer[EthereumBalances]
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

type EthereumBalances struct{}

type SolanaBalances struct {
	PreTokenBalances  []rpc.TransactionMetaTokenBalance
	PostTokenBalances []rpc.TransactionMetaTokenBalance
}
