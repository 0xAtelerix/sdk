package tokens

import "math/big"

// RegisterDefaultEvmTransfers registers the canonical ERC-20/721/1155
// transfer events into the provided *Registry[EvmTransfer].
func RegisterDefaultEvmTransfers(reg *Registry[AppEvent]) {
	// ERC-20 Transfer: 3 topics (sig, from, to), value in data
	Register[erc20Transfer](reg, SigTransfer, ERC20TransferABI, "Transfer",
		func(ev erc20Transfer, m Meta) ([]AppEvent, error) {
			evm := EvmTransfer{
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
			}
			return []AppEvent{evm}, nil
		},
	)

	// ERC-721 Transfer: 4 topics (sig, from, to, tokenId), no data
	Register[erc721Transfer](reg, SigTransfer, ERC721TransferABI, "Transfer",
		func(ev erc721Transfer, m Meta) ([]AppEvent, error) {
			evm := EvmTransfer{
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
			}
			return []AppEvent{evm}, nil
		},
	)

	// ERC-1155 TransferSingle
	Register[erc1155Single](reg, SigTransferSingle, ERC1155ABI, "TransferSingle",
		func(ev erc1155Single, m Meta) ([]AppEvent, error) {
			evm := EvmTransfer{
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
			}
			return []AppEvent{evm}, nil
		},
	)

	// ERC-1155 TransferBatch → one row per (id,value)
	Register[erc1155Batch](reg, SigTransferBatch, ERC1155ABI, "TransferBatch",
		func(ev erc1155Batch, m Meta) ([]AppEvent, error) {
			out := make([]AppEvent, 0, len(ev.IDs))
			for i := range ev.IDs {
				evm := EvmTransfer{
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
				}
				out = append(out, evm)
			}

			return out, nil
		},
	)
}

// EnsureDefaultEvmTransfers registers defaults into the global EVMEventRegistry.
// If a registry is passed, load that; otherwise, load the global.
func EnsureDefaultEvmTransfersInto(regs ...*Registry[AppEvent]) {
	target := EVMEventRegistry
	if len(regs) > 0 && regs[0] != nil {
		target = regs[0]
	}
	RegisterDefaultEvmTransfers(target)
}

// NewDefaultEvmTransferRegistry returns a fresh registry preloaded with defaults.
func NewDefaultEvmTransferRegistry() *Registry[AppEvent] {
	reg := NewRegistry[AppEvent]()
	RegisterDefaultEvmTransfers(reg)
	return reg
}
