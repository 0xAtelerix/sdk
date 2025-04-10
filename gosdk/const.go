package gosdk

const (
	EthereumChainID  = 1     // Ethereum Mainnet
	PolygonChainID   = 137   // Polygon (PoS)
	BNBChainID       = 56    // BNB Smart Chain (BSC)
	AvalancheChainID = 43114 // Avalanche C-Chain
	GnosisChainID    = 100   // Gnosis Chain (xDai)
	FantomChainID    = 250   // Fantom Opera
	BaseChainID      = 8453  // Base Mainnet

	//testnets
	EthereumSepoliaChainID = 11155111 // Ethereum Sepolia Testnet
	PolygonMumbaiChainID   = 80001    // Polygon Mumbai Testnet
	BNBTestnetChainID      = 97       // BNB Smart Chain Testnet
	AvalancheFujiChainID   = 43113    // Avalanche Fuji Testnet
	GnosisChiadoChainID    = 10200    // Gnosis Chiado Testnet
	FantomTestnetChainID   = 4002     // Fantom Testnet

	SolanaDevnetChainID  = 123231
	SolanaTestnetChainID = 123234
	SolanaChainID        = 1232342
)

var EvmChains = map[uint32]struct{}{
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

var SolanaChains = map[uint32]struct{}{
	SolanaTestnetChainID: {},
	SolanaDevnetChainID:  {},
	SolanaChainID:        {},
}

func IsEvmChain(chainID uint32) bool {
	_, ok := EvmChains[chainID]
	return ok
}

func IsSolanaChain(chainID uint32) bool {
	_, ok := SolanaChains[chainID]
	return ok
}
