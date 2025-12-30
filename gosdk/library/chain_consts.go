package library

import (
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

const (
	EthereumAddressLength = 20
	SolanaAddressLength   = 32
)

type EthereumAddress [EthereumAddressLength]byte

type EthereumTopic [32]byte

func EmptyEthereumTopic() EthereumTopic {
	return EthereumTopic{}
}

type SolanaAddress [SolanaAddressLength]byte

type Address interface {
	~[EthereumAddressLength]byte | ~[SolanaAddressLength]byte
}

type AddressMap interface {
	~map[EthereumAddress]struct{} | ~map[SolanaAddress]struct{}
}

type ChainAddresses[T Address] struct {
	ChainID   apptypes.ChainType
	Addresses []T
}

type ChainTopicAddresses struct {
	Address EthereumAddress `cbor:"1,keyasint"`
	Topics  []EthereumTopic `cbor:"2,keyasint"`
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
