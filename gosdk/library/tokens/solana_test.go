package tokens

import (
	"math/big"
	"testing"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/stretchr/testify/require"
)

// tiny helper
func tb(owner, mint, amount string, decimals uint8) rpc.TransactionMetaTokenBalance {
	return rpc.TransactionMetaTokenBalance{
		Owner: owner,
		Mint:  mint,
		UITokenAmount: rpc.TokenAccountBalance{
			Amount:   amount,   // raw base units as decimal string
			Decimals: decimals, // 0..9+
		},
	}
}

func bi(s string) *big.Int {
	z, _ := new(big.Int).SetString(s, 10)

	return z
}

func TestSingleTransfer_SameMint(t *testing.T) {
	t.Parallel()

	// A sends 100 to B (USDC mint, 6 decimals)
	const (
		mint = "So11111111111111111111111111111111111111112"
		A    = "A_owner"
		B    = "B_owner"
	)

	tx := client.BlockTransaction{
		Meta: &client.TransactionMeta{
			PreTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(A, mint, "1000", 6), // A had 1000
				tb(B, mint, "200", 6),  // B had 200
			},
			PostTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(A, mint, "900", 6), // A -100
				tb(B, mint, "300", 6), // B +100
			},
		},
	}

	out := ExtractSplTransfers(tx)
	require.Len(t, out, 1)

	x := out[0]
	require.Equal(t, mint, x.Mint)
	require.Equal(t, A, x.FromOwner)
	require.Equal(t, B, x.ToOwner)
	require.Equal(t, uint8(6), x.Decimals)
	require.Equal(t, 0, x.Amount.Cmp(bi("100")))
}

func TestTwoTransfers_SameMint_DifferentOwners(t *testing.T) {
	t.Parallel()

	const (
		mint = "Tokenkeg1111111111111111111111111111111"
		A    = "A_owner"
		B    = "B_owner"
		C    = "C_owner"
	)

	// A -> B : 60 ; C -> B : 40  (total B +100)
	tx := client.BlockTransaction{
		Meta: &client.TransactionMeta{
			PreTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(A, mint, "500", 6),
				tb(B, mint, "1000", 6),
				tb(C, mint, "400", 6),
			},
			PostTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(A, mint, "440", 6),  // -60
				tb(B, mint, "1100", 6), // +100
				tb(C, mint, "360", 6),  // -40
			},
		},
	}

	out := ExtractSplTransfers(tx)
	require.Len(t, out, 2)

	// Order isn’t guaranteed; bucket by (from,to,amt)
	type key struct{ f, to, amt string }

	set := map[key]bool{}
	for _, x := range out {
		set[key{x.FromOwner, x.ToOwner, x.Amount.String()}] = true
		require.Equal(t, mint, x.Mint)
		require.Equal(t, uint8(6), x.Decimals)
	}

	require.True(t, set[key{A, B, "60"}])
	require.True(t, set[key{C, B, "40"}])
}

func TestDifferentMints_ParallelTransfers(t *testing.T) {
	t.Parallel()

	const (
		usdc = "USDC11111111111111111111111111111111111111"
		solx = "So2122222222222222222222222222222222222222"
		A    = "A_owner"
		B    = "B_owner"
	)

	tx := client.BlockTransaction{
		Meta: &client.TransactionMeta{
			PreTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(A, usdc, "1000", 6),
				tb(B, usdc, "0", 6),
				tb(A, solx, "50", 9),
				tb(B, solx, "10", 9),
			},
			PostTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(A, usdc, "900", 6), // -100
				tb(B, usdc, "100", 6), // +100
				tb(A, solx, "40", 9),  // -10
				tb(B, solx, "20", 9),  // +10
			},
		},
	}

	out := ExtractSplTransfers(tx)
	require.Len(t, out, 2)

	// group by mint
	var usdcX, solxX *SolTransfer

	for i := range out {
		if out[i].Mint == usdc {
			usdcX = &out[i]
		}

		if out[i].Mint == solx {
			solxX = &out[i]
		}
	}

	require.NotNil(t, usdcX)
	require.NotNil(t, solxX)

	require.Equal(t, A, usdcX.FromOwner)
	require.Equal(t, B, usdcX.ToOwner)
	require.Equal(t, 0, usdcX.Amount.Cmp(bi("100")))
	require.Equal(t, uint8(6), usdcX.Decimals)

	require.Equal(t, A, solxX.FromOwner)
	require.Equal(t, B, solxX.ToOwner)
	require.Equal(t, 0, solxX.Amount.Cmp(bi("10")))
	require.Equal(t, uint8(9), solxX.Decimals)
}

func TestMintOrBurn_UnmatchedDeltasIgnored(t *testing.T) {
	t.Parallel()

	// A gets +50 new tokens (mint), nobody decreased.
	const (
		mint = "MintMintMintMintMintMintMintMintMintMint"
		A    = "A_owner"
	)

	tx := client.BlockTransaction{
		Meta: &client.TransactionMeta{
			PreTokenBalances:  []rpc.TransactionMetaTokenBalance{tb(A, mint, "0", 6)},
			PostTokenBalances: []rpc.TransactionMetaTokenBalance{tb(A, mint, "50", 6)}, // +50
		},
	}

	// Extractor matches negatives to positives per mint; unmatched are mints/burns => ignored.
	out := ExtractSplTransfers(tx)
	require.Empty(t, out)
}

func TestSelfTransferSameOwner_NetsToZero(t *testing.T) {
	t.Parallel()

	// If you diff by (owner, mint), moving between token accounts owned by same owner nets to 0,
	// which is intended for "logical" owner transfers; switch to account-level diff if needed.
	const (
		mint  = "TokenzQd1111111111111111111111111111111111"
		owner = "OwnerX"
	)

	tx := client.BlockTransaction{
		Meta: &client.TransactionMeta{
			PreTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(owner, mint, "70", 6),
			},
			PostTokenBalances: []rpc.TransactionMetaTokenBalance{
				tb(owner, mint, "70", 6),
			},
		},
	}

	out := ExtractSplTransfers(tx)
	require.Empty(t, out)
}
