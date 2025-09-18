package tokens

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
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

// ExtractErcTransfers parses a single receipt and returns logical token transfers.
// It covers standard-compliant ERC-20, ERC-721, and ERC-1155 logs (including CPIs inside the same tx).
func ExtractErcTransfers(r gethtypes.Receipt) []EvmTokenTransfer {
	out := make([]EvmTokenTransfer, 0, len(r.Logs))

	for _, lg := range r.Logs {
		if len(lg.Topics) == 0 {
			continue
		}

		sig := lg.Topics[0]

		switch sig {
		case transferSig():
			// ERC-20 and ERC-721 share the same signature.
			// Heuristic: topic count
			// - ERC-20 Transfer(address indexed from, address indexed to, uint256 value)
			//     topics: [sig, from, to], data: value
			// - ERC-721 Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
			//     topics: [sig, from, to, tokenId], data: empty
			switch len(lg.Topics) {
			case 3: // ERC-20
				from := topicToAddress(lg.Topics[1])
				to := topicToAddress(lg.Topics[2])

				vals, err := erc20Data.Unpack(lg.Data) // []interface{}{*big.Int}
				if err != nil || len(vals) != 1 {
					continue
				}

				amount, ok := vals[0].(*big.Int)
				if !ok {
					continue
				}

				out = append(out, EvmTokenTransfer{
					Token:    lg.Address,
					Standard: ERC20,
					From:     from,
					To:       to,
					Amount:   new(big.Int).Set(amount),
					TxHash:   r.TxHash,
					LogIndex: lg.Index,
				})

			case 4: // ERC-721
				from := topicToAddress(lg.Topics[1])
				to := topicToAddress(lg.Topics[2])
				tokenID := lg.Topics[3].Big()

				out = append(out, EvmTokenTransfer{
					Token:    lg.Address,
					Standard: ERC721,
					From:     from,
					To:       to,
					TokenID:  new(big.Int).Set(tokenID),
					// Optionally: Amount = big.NewInt(1) for an NFT
					TxHash:   r.TxHash,
					LogIndex: lg.Index,
				})
			default:
				continue
			}

		case transferSingleSig():
			// topics: [sig, operator, from, to]
			if len(lg.Topics) != 4 {
				continue
			}

			from := topicToAddress(lg.Topics[2])
			to := topicToAddress(lg.Topics[3])

			vals, err := erc1155DataS.Unpack(lg.Data) // [id, value]
			if err != nil || len(vals) != 2 {
				continue
			}

			id, ok := vals[0].(*big.Int)
			if !ok {
				continue
			}

			value, ok := vals[1].(*big.Int)
			if !ok {
				continue
			}

			out = append(out, EvmTokenTransfer{
				Token:    lg.Address,
				Standard: ERC1155,
				From:     from,
				To:       to,
				TokenID:  new(big.Int).Set(id),
				Amount:   new(big.Int).Set(value),
				TxHash:   r.TxHash,
				LogIndex: lg.Index,
			})

		case transferBatchSig():
			// topics: [sig, operator, from, to]
			if len(lg.Topics) != 4 {
				continue
			}

			from := topicToAddress(lg.Topics[2])
			to := topicToAddress(lg.Topics[3])

			vals, err := erc1155DataB.Unpack(lg.Data) // [ids[], values[]]
			if err != nil || len(vals) != 2 {
				continue
			}

			ids := toBigIntSlice(vals[0])
			values := toBigIntSlice(vals[1])

			out = append(out, EvmTokenTransfer{
				Token:    lg.Address,
				Standard: ERC1155,
				From:     from,
				To:       to,
				IDs:      ids,
				Values:   values,
				TxHash:   r.TxHash,
				LogIndex: lg.Index,
			})
		default:
			continue
		}
	}

	return out
}

// Helpers

func topicToAddress(t common.Hash) common.Address {
	// address is right-padded 32-byte topic; take last 20 bytes
	return common.BytesToAddress(t[12:])
}

func toBigIntSlice(v any) []*big.Int {
	switch xs := v.(type) {
	case []*big.Int:
		return xs
	case []any:
		out := make([]*big.Int, 0, len(xs))
		for _, it := range xs {
			b, ok := it.(*big.Int)
			if ok {
				out = append(out, b)
			}
		}

		return out
	default:
		return nil
	}
}
