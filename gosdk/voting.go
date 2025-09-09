package gosdk

import (
	"github.com/holiman/uint256"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type Voting struct {
	totalVotingPower *uint256.Int
	threshold        *uint256.Int

	// map[ChainID][BlockNumber][BlockHash]votes
	voting map[uint64]map[uint64]map[[32]byte]*uint256.Int
}

// NewVoting creates a new Voting accumulator.
// total must be non-nil. If you need to change total later, just set v.totalVotingPower.
func NewVoting(total *uint256.Int) *Voting {
	if total == nil {
		total = uint256.NewInt(0)
	}

	// copy total so external mutations don't affect us
	cp := total.Clone()

	threshold := Threshold(total)

	return &Voting{
		totalVotingPower: cp,
		threshold:        threshold,
		voting:           make(map[uint64]map[uint64]map[[32]byte]*uint256.Int),
	}
}

func NewVotingFromValidatorSet(validators *ValidatorSet) *Voting {
	total := uint256.NewInt(0)

	if validators != nil {
		for _, stake := range validators.Set {
			if stake == 0 {
				continue
			}

			total.Add(total, uint256.NewInt(uint64(stake)))
		}
	}

	return NewVoting(total)
}

// AddVote adds `power` to the tally for (chainID, blockNumber, blockHash).
// A nil power is treated as 0 and is a no-op.
func (v *Voting) AddVote(extBlock apptypes.ExternalBlock, power *uint256.Int) {
	if power == nil || power.IsZero() {
		return
	}

	blockVoting, ok := v.voting[extBlock.ChainID]
	if !ok {
		blockVoting = make(map[uint64]map[[32]byte]*uint256.Int)
		v.voting[extBlock.ChainID] = blockVoting
	}

	hashVoting, ok := blockVoting[extBlock.BlockNumber]
	if !ok {
		hashVoting = make(map[[32]byte]*uint256.Int)
		blockVoting[extBlock.BlockNumber] = hashVoting
	}

	curr, ok := hashVoting[extBlock.BlockHash]
	if !ok {
		curr = uint256.NewInt(0)
		hashVoting[extBlock.BlockHash] = curr
	}

	curr.Add(curr, power)
}

// FinalizedBlocks returns all blocks that have votes >= Threshold().
func (v *Voting) FinalizedBlocks() []apptypes.ExternalBlock {
	th := v.threshold

	if th.IsZero() {
		return nil
	}

	var out []apptypes.ExternalBlock

	for chainID, byBlockNum := range v.voting {
		for blockNum, byHash := range byBlockNum {
			for h, votes := range byHash {
				if votes.Cmp(th) >= 0 {
					out = append(out, apptypes.ExternalBlock{
						ChainID:     chainID,
						BlockNumber: blockNum,
						BlockHash:   h,
					})
				}
			}
		}
	}

	return out
}

func (v *Voting) GetVotes(block apptypes.ExternalBlock) *uint256.Int {
	if blockVoting, ok := v.voting[block.ChainID]; ok {
		if hashVoting, ok := blockVoting[block.BlockNumber]; ok {
			if votes, ok := hashVoting[block.BlockHash]; ok {
				return votes.Clone()
			}
		}
	}

	return uint256.NewInt(0)
}

// Threshold returns 2/3 of totalVotingPower, rounded up, plus 1.
// If totalVotingPower == 0, returns 0 (so nothing can finalize spuriously).
func Threshold(totalVotingPower *uint256.Int) *uint256.Int {
	if totalVotingPower == nil || totalVotingPower.IsZero() {
		return uint256.NewInt(0)
	}

	// floor(total*2/3) + 1
	two := uint256.NewInt(2)
	three := uint256.NewInt(3)

	// numerator = 2*total + (3 - 1)
	numer := new(uint256.Int).Mul(totalVotingPower, two)
	numer.Add(numer, uint256.NewInt(2)) // add (y - 1), where y=3

	// ceilDiv = numerator / 3
	ceilDiv := new(uint256.Int).Div(numer, three)

	// +1 for the strict “>⅔” condition
	return new(uint256.Int).Add(ceilDiv, uint256.NewInt(1))
}
