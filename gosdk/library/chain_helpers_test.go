package library

import (
	"testing"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func TestChainSetAccessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chainID  apptypes.ChainType
		set      func() map[apptypes.ChainType]struct{}
		contains func(apptypes.ChainType) bool
	}{
		{
			name:     "evm",
			chainID:  EthereumChainID,
			set:      EVMChains,
			contains: IsEvmChain,
		},
		{
			name:     "solana",
			chainID:  SolanaChainID,
			set:      SolanaChains,
			contains: IsSolanaChain,
		},
		{
			name:     "midnight",
			chainID:  MidnightPreviewChainID,
			set:      MidnightChains,
			contains: IsMidnightChain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chains := tt.set()
			if _, ok := chains[tt.chainID]; !ok {
				t.Fatalf("expected %s set to contain %d", tt.name, tt.chainID)
			}

			if !tt.contains(tt.chainID) {
				t.Fatalf("expected %s predicate to accept %d", tt.name, tt.chainID)
			}

			if _, ok := tt.set()[tt.chainID]; !ok {
				t.Fatalf("%s accessor should contain %d", tt.name, tt.chainID)
			}
		})
	}
}

func BenchmarkIsEvmChain(b *testing.B) {
	for b.Loop() {
		if !IsEvmChain(EthereumChainID) {
			b.Fatal("expected EVM chain")
		}
	}
}
