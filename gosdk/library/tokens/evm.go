package tokens

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type EvmTokenTransfer struct {
	Token    common.Address // contract that emitted the event
	Standard Standard

	From common.Address
	To   common.Address

	// ERC-20: Amount set, TokenID nil
	// ERC-721: TokenID set, Amount = 1 (convention), or nil if you prefer
	// ERC-1155 single: TokenID and Amount set
	// ERC-1155 batch: IDs and Values set (From/To are same for all rows)
	Amount  *big.Int
	TokenID *big.Int

	// For ERC-1155 TransferBatch
	IDs    []*big.Int
	Values []*big.Int

	TxHash   common.Hash
	LogIndex uint
}
