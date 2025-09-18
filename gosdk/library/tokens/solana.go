package tokens

import (
	"math/big"
	"sort"

	"github.com/blocto/solana-go-sdk/client"
)

// ExtractSplTransfers returns logical token transfers for a BlockTransaction.
// It uses meta pre/post token balances, so it covers CPI token movements too.
func ExtractSplTransfers(tx client.BlockTransaction) []SolTransfer {
	meta := tx.Meta
	if meta == nil {
		return nil
	}

	type key struct {
		owner string
		mint  string
	}

	// Gather pre/post by (owner, mint). Use raw "amount" (string) => big.Int.
	pre := map[key]*big.Int{}
	post := map[key]*big.Int{}
	decimals := map[string]uint8{} // mint -> decimals

	// helper to parse token amount strings safely
	parse := func(s string) *big.Int {
		if s == "" {
			return big.NewInt(0)
		}

		z, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return big.NewInt(0)
		}

		return z
	}

	// PreTokenBalances
	for _, tb := range meta.PreTokenBalances {
		k := key{owner: tb.Owner, mint: tb.Mint}
		pre[k] = new(big.Int).Add(parse(tb.UITokenAmount.Amount), big.NewInt(0))
		decimals[tb.Mint] = tb.UITokenAmount.Decimals
	}
	// PostTokenBalances
	for _, tb := range meta.PostTokenBalances {
		k := key{owner: tb.Owner, mint: tb.Mint}
		post[k] = new(big.Int).Add(parse(tb.UITokenAmount.Amount), big.NewInt(0))
		decimals[tb.Mint] = tb.UITokenAmount.Decimals
	}

	// Compute deltas per (owner, mint)
	type delta struct {
		owner  string
		mint   string
		amount *big.Int // positive = received, negative = sent
		decim  uint8
	}

	var deltas []delta

	seen := map[key]struct{}{}
	for k, v := range pre {
		seen[k] = struct{}{}

		p := post[k]
		if p == nil {
			p = big.NewInt(0)
		}

		amt := new(big.Int).Sub(p, v) // post - pre
		if amt.Sign() != 0 {
			deltas = append(deltas, delta{k.owner, k.mint, amt, decimals[k.mint]})
		}
	}

	for k, p := range post { // keys that only exist in post
		if _, ok := seen[k]; ok {
			continue
		}

		amt := new(big.Int).Set(p) // 0 -> post
		if amt.Sign() != 0 {
			deltas = append(deltas, delta{k.owner, k.mint, amt, decimals[k.mint]})
		}
	}

	// Split per mint into outs/ins and greedily match equal amounts.
	type item struct {
		owner  string
		amount *big.Int
	}

	transfers := make([]SolTransfer, 0)

	// group by mint
	byMint := map[string][]delta{}
	for _, d := range deltas {
		byMint[d.mint] = append(byMint[d.mint], d)
	}

	for mint, list := range byMint {
		dec := decimals[mint]

		var outs, ins []item

		for _, d := range list {
			if d.amount.Sign() < 0 {
				outs = append(outs, item{owner: d.owner, amount: new(big.Int).Neg(d.amount)})
			} else if d.amount.Sign() > 0 {
				ins = append(ins, item{owner: d.owner, amount: new(big.Int).Set(d.amount)})
			}
		}

		if len(outs) == 0 || len(ins) == 0 {
			continue // mints/burns or pure receive/send without counterpart
		}

		// Sort for deterministic pairing (largest first helps reduce fragmentation)
		sort.Slice(outs, func(i, j int) bool { return outs[i].amount.Cmp(outs[j].amount) > 0 })
		sort.Slice(ins, func(i, j int) bool { return ins[i].amount.Cmp(ins[j].amount) > 0 })

		// Greedy matching
		for oi := 0; oi < len(outs); oi++ {
			o := outs[oi]
			if o.amount.Sign() == 0 {
				continue
			}

			for ii := 0; ii < len(ins) && o.amount.Sign() > 0; ii++ {
				i := ins[ii]
				if i.amount.Sign() == 0 {
					continue
				}

				amt := new(big.Int)
				if o.amount.Cmp(i.amount) <= 0 {
					amt.Set(o.amount)
				} else {
					amt.Set(i.amount)
				}

				if amt.Sign() > 0 {
					transfers = append(transfers, SolTransfer{
						Mint:      mint,
						FromOwner: o.owner,
						ToOwner:   i.owner,
						Amount:    new(big.Int).Set(amt),
						Decimals:  dec,
						Balances: SolanaBalances{
							PreTokenBalances:  meta.PreTokenBalances,
							PostTokenBalances: meta.PostTokenBalances,
						},
					})
					// reduce both sides
					o.amount.Sub(o.amount, amt)
					outs[oi] = o

					ins[ii].amount.Sub(ins[ii].amount, amt)
				}
			}
		}
	}

	return transfers
}
