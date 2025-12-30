package tokens

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Event shapes (field names ≡ ABI names via `abi` tags)

type erc20Transfer struct {
	From  common.Address `abi:"from"`
	To    common.Address `abi:"to"`
	Value *big.Int       `abi:"value"`
}

type erc721Transfer struct {
	From    common.Address `abi:"from"`
	To      common.Address `abi:"to"`
	TokenID *big.Int       `abi:"tokenId"`
}

type erc1155Single struct {
	Operator common.Address `abi:"operator"`
	From     common.Address `abi:"from"`
	To       common.Address `abi:"to"`
	ID       *big.Int       `abi:"id"`
	Value    *big.Int       `abi:"value"`
}

type erc1155Batch struct {
	Operator common.Address `abi:"operator"`
	From     common.Address `abi:"from"`
	To       common.Address `abi:"to"`
	IDs      []*big.Int     `abi:"ids"`
	Values   []*big.Int     `abi:"values"`
}

// ExtractErcTransfers parses standard ERC-20/721/1155 logs from a receipt
// and returns generic EvmTransfer entries.
func ExtractErcTransfers(r *gethtypes.Receipt) []EvmTransfer {
	if r == nil || len(r.Logs) == 0 {
		return nil
	}

	out := make([]EvmTransfer, 0, len(r.Logs))

	for _, lg := range r.Logs {
		if len(lg.Topics) == 0 {
			continue
		}

		switch lg.Topics[0] {
		case SigTransfer:
			// Topic count disambiguates ERC-20 vs ERC-721
			if len(lg.Topics) == 4 {
				// Heuristic by topic count:
				// - 4 topics => ERC-721 (tokenId indexed)
				// - 3 topics => ERC-20 (value in data)
				ev, matched, err := DecodeEventInto[erc721Transfer](
					ERC721TransferABI,
					"Transfer",
					lg,
				)
				if !matched || err != nil {
					continue
				}

				out = append(out, EvmTransfer{
					Mint:      lg.Address.Hex(),
					FromOwner: ev.From.Hex(),
					ToOwner:   ev.To.Hex(),
					Amount:    big.NewInt(1), // NFT convention
					Decimals:  0,
					Balances: EthereumBalances{
						Standard: ERC721,
						TokenID:  new(big.Int).Set(ev.TokenID),
						TxHash:   r.TxHash,
						LogIndex: lg.Index,
					},
				})
			} else {
				// ERC-20: value in data
				ev, matched, err := DecodeEventInto[erc20Transfer](ERC20TransferABI, "Transfer", lg)
				if !matched || err != nil {
					continue
				}

				out = append(out, EvmTransfer{
					Mint:      lg.Address.Hex(),
					FromOwner: ev.From.Hex(),
					ToOwner:   ev.To.Hex(),
					Amount:    new(big.Int).Set(ev.Value),
					Decimals:  0, // can be enriched later from contract
					Balances: EthereumBalances{
						Standard: ERC20,
						TxHash:   r.TxHash,
						LogIndex: lg.Index,
					},
				})
			}

		case SigTransferSingle:
			ev, matched, err := DecodeEventInto[erc1155Single](ERC1155ABI, "TransferSingle", lg)
			if !matched || err != nil {
				continue
			}

			out = append(out, EvmTransfer{
				Mint:      lg.Address.Hex(),
				FromOwner: ev.From.Hex(),
				ToOwner:   ev.To.Hex(),
				Amount:    new(big.Int).Set(ev.Value),
				Decimals:  0,
				Balances: EthereumBalances{
					Standard: ERC1155,
					TokenID:  new(big.Int).Set(ev.ID),
					TxHash:   r.TxHash,
					LogIndex: lg.Index,
				},
			})

		case SigTransferBatch:
			ev, matched, err := DecodeEventInto[erc1155Batch](ERC1155ABI, "TransferBatch", lg)
			if !matched || err != nil {
				continue
			}
			// Emit one row per (id,value)
			for i := range ev.IDs {
				out = append(out, EvmTransfer{
					Mint:      lg.Address.Hex(),
					FromOwner: ev.From.Hex(),
					ToOwner:   ev.To.Hex(),
					Amount:    new(big.Int).Set(ev.Values[i]),
					Decimals:  0,
					Balances: EthereumBalances{
						Standard: ERC1155,
						TokenID:  new(big.Int).Set(ev.IDs[i]),
						IDs:      ev.IDs,    // keep full arrays if desired
						Values:   ev.Values, // keep full arrays if desired
						TxHash:   r.TxHash,
						LogIndex: lg.Index,
					},
				})
			}
		default:
			continue
		}
	}

	return out
}
