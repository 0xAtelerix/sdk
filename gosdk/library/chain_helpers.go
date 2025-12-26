package library

import (
	"slices"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func SolanaChains() map[apptypes.ChainType]struct{} {
	return map[apptypes.ChainType]struct{}{
		SolanaTestnetChainID: {},
		SolanaDevnetChainID:  {},
		SolanaChainID:        {},
	}
}

func IsEvmChain(chainID apptypes.ChainType) bool {
	_, ok := EVMChains()[chainID]

	return ok
}

func IsSolanaChain(chainID apptypes.ChainType) bool {
	_, ok := SolanaChains()[chainID]

	return ok
}

func CmpAddr[T Address](a, b T) int {
	for i := range len(a) {
		if a[i] < b[i] {
			return -1
		}

		if a[i] > b[i] {
			return 1
		}
	}

	return 0
}

func SortChainAddresses[T Address](items []ChainAddresses[T]) {
	slices.SortFunc(items, func(a, b ChainAddresses[T]) int {
		switch {
		case a.ChainID < b.ChainID:
			return -1
		case a.ChainID > b.ChainID:
			return 1
		default:
			return 0
		}
	})

	for i := range items {
		slices.SortFunc(items[i].Addresses, CmpAddr[T])
	}
}

func CollectChainAddresses[T Address](m map[apptypes.ChainType]map[T]struct{}) []ChainAddresses[T] {
	out := make([]ChainAddresses[T], 0, len(m))

	for chainID, set := range m {
		ca := ChainAddresses[T]{
			ChainID:   chainID,
			Addresses: make([]T, 0, len(set)),
		}

		for addr := range set {
			ca.Addresses = append(ca.Addresses, addr)
		}

		out = append(out, ca)
	}

	SortChainAddresses(out)

	return out
}

// CollectChainTopicsGrouped converts nested maps into a stable slice per chain.
func CollectChainTopicsGrouped(
	src map[apptypes.ChainType]map[EthereumAddress]map[EthereumTopic]struct{},
) map[apptypes.ChainType][]ChainTopicAddresses {
	out := make(map[apptypes.ChainType][]ChainTopicAddresses)

	for chainID, addrMap := range src {
		for addr, topics := range addrMap {
			item := ChainTopicAddresses{
				Address: addr,
				Topics:  make([]EthereumTopic, 0, len(topics)),
			}

			for t := range topics {
				item.Topics = append(item.Topics, t)
			}

			out[chainID] = append(out[chainID], item)
		}
	}

	return out
}
