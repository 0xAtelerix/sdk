package gosdk

import (
	"slices"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

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

const (
	EthereumChainID  = apptypes.ChainType(1)     // Ethereum Mainnet
	PolygonChainID   = apptypes.ChainType(137)   // Polygon (PoS)
	BNBChainID       = apptypes.ChainType(56)    // BNB Smart Chain (BSC)
	AvalancheChainID = apptypes.ChainType(43114) // Avalanche C-Chain
	GnosisChainID    = apptypes.ChainType(100)   // Gnosis Chain (xDai)
	FantomChainID    = apptypes.ChainType(250)   // Fantom Opera
	BaseChainID      = apptypes.ChainType(8453)  // Base Mainnet

	// testnets
	EthereumSepoliaChainID = apptypes.ChainType(11155111) // Ethereum Sepolia Testnet
	PolygonMumbaiChainID   = apptypes.ChainType(80001)    // Polygon Mumbai Testnet
	PolygonAmoyChainID     = apptypes.ChainType(80002)    // Polygon Amoy Testnet
	BNBTestnetChainID      = apptypes.ChainType(97)       // BNB Smart Chain Testnet
	AvalancheFujiChainID   = apptypes.ChainType(43113)    // Avalanche Fuji Testnet
	GnosisChiadoChainID    = apptypes.ChainType(10200)    // Gnosis Chiado Testnet
	FantomTestnetChainID   = apptypes.ChainType(4002)     // Fantom Testnet

	StavangerTestnetChainID = apptypes.ChainType(50591822) // Stavanger Testnet

	SolanaDevnetChainID  = apptypes.ChainType(123231)
	SolanaTestnetChainID = apptypes.ChainType(123234)
	SolanaChainID        = apptypes.ChainType(1232342)
)

func EVMChains() map[apptypes.ChainType]struct{} {
	return map[apptypes.ChainType]struct{}{
		EthereumChainID:         {},
		PolygonChainID:          {},
		BNBChainID:              {},
		AvalancheChainID:        {},
		GnosisChainID:           {},
		FantomChainID:           {},
		BaseChainID:             {},
		EthereumSepoliaChainID:  {},
		PolygonMumbaiChainID:    {},
		PolygonAmoyChainID:      {},
		BNBTestnetChainID:       {},
		AvalancheFujiChainID:    {},
		GnosisChiadoChainID:     {},
		FantomTestnetChainID:    {},
		StavangerTestnetChainID: {},
	}
}

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

type ChainAddresses[T Address] struct {
	chainID   apptypes.ChainType
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

func CollectChainAddresses[T Address](m map[apptypes.ChainType]map[T]struct{}) []ChainAddresses[T] {
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
