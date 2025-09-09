package gosdk

import "slices"

const (
	EthereumAddressLength = 20
	SolanaAddressLength   = 32
)

type EthereumAddress [EthereumAddressLength]byte

type SolanaAddress [SolanaAddressLength]byte

type Address interface {
	~[EthereumAddressLength]byte | ~[SolanaAddressLength]byte
}

type AddressMap interface {
	~map[EthereumAddress]struct{} | ~map[SolanaAddress]struct{}
}

type ChainType uint32

const (
	EthereumChainID  = ChainType(1)     // Ethereum Mainnet
	PolygonChainID   = ChainType(137)   // Polygon (PoS)
	BNBChainID       = ChainType(56)    // BNB Smart Chain (BSC)
	AvalancheChainID = ChainType(43114) // Avalanche C-Chain
	GnosisChainID    = ChainType(100)   // Gnosis Chain (xDai)
	FantomChainID    = ChainType(250)   // Fantom Opera
	BaseChainID      = ChainType(8453)  // Base Mainnet

	// testnets
	EthereumSepoliaChainID = ChainType(11155111) // Ethereum Sepolia Testnet
	PolygonMumbaiChainID   = ChainType(80001)    // Polygon Mumbai Testnet
	BNBTestnetChainID      = ChainType(97)       // BNB Smart Chain Testnet
	AvalancheFujiChainID   = ChainType(43113)    // Avalanche Fuji Testnet
	GnosisChiadoChainID    = ChainType(10200)    // Gnosis Chiado Testnet
	FantomTestnetChainID   = ChainType(4002)     // Fantom Testnet

	SolanaDevnetChainID  = ChainType(123231)
	SolanaTestnetChainID = ChainType(123234)
	SolanaChainID        = ChainType(1232342)
)

func EVMChains() map[ChainType]struct{} {
	//nolint:exhaustive // it's a config map, it should have only EVM chains, not all chains
	return map[ChainType]struct{}{
		EthereumChainID:        {},
		PolygonChainID:         {},
		BNBChainID:             {},
		AvalancheChainID:       {},
		GnosisChainID:          {},
		FantomChainID:          {},
		BaseChainID:            {},
		EthereumSepoliaChainID: {},
		PolygonMumbaiChainID:   {},
		BNBTestnetChainID:      {},
		AvalancheFujiChainID:   {},
		GnosisChiadoChainID:    {},
		FantomTestnetChainID:   {},
	}
}

func SolanaChains() map[ChainType]struct{} {
	//nolint:exhaustive // it's a config map, it should have only SVM chains, not all chains
	return map[ChainType]struct{}{
		SolanaTestnetChainID: {},
		SolanaDevnetChainID:  {},
		SolanaChainID:        {},
	}
}

func IsEvmChain(chainID ChainType) bool {
	_, ok := EVMChains()[chainID]

	return ok
}

func IsSolanaChain(chainID ChainType) bool {
	_, ok := SolanaChains()[chainID]

	return ok
}

type ChainAddresses[T Address] struct {
	chainID   ChainType
	addresses []T
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
		case a.chainID < b.chainID:
			return -1
		case a.chainID > b.chainID:
			return 1
		default:
			return 0
		}
	})

	for i := range items {
		slices.SortFunc(items[i].addresses, CmpAddr[T])
	}
}

func CollectChainAddresses[T Address](m map[ChainType]map[T]struct{}) []ChainAddresses[T] {
	out := make([]ChainAddresses[T], 0, len(m))

	for chainID, set := range m {
		ca := ChainAddresses[T]{
			chainID:   chainID,
			addresses: make([]T, 0, len(set)),
		}

		for addr := range set {
			ca.addresses = append(ca.addresses, addr)
		}

		out = append(out, ca)
	}

	SortChainAddresses(out)

	return out
}
