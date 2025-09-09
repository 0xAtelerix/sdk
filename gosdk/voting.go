package gosdk

import (
	"github.com/holiman/uint256"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type Voting[T apptypes.ExternalEntity] struct {
	totalVotingPower *uint256.Int
	threshold        *uint256.Int

	// map[ChainID][BlockNumber][BlockHash]votes
	voting map[uint64]map[uint64]map[[32]byte]*Entity[T]
}

type Entity[T apptypes.ExternalEntity] struct {
	voted  *uint256.Int
	entity *T
}

// NewVoting creates a new Voting accumulator.
// total must be non-nil. If you need to change total later, just set v.totalVotingPower.
func NewVoting[T apptypes.ExternalEntity](total *uint256.Int) *Voting[T] {
	if total == nil {
		total = uint256.NewInt(0)
	}

	// copy total so external mutations don't affect us
	cp := total.Clone()

	threshold := Threshold(total)

	return &Voting[T]{
		totalVotingPower: cp,
		threshold:        threshold,
		voting:           make(map[uint64]map[uint64]map[[32]byte]*Entity[T]),
	}
}

func NewVotingFromValidatorSet[T apptypes.ExternalEntity](validators *ValidatorSet) *Voting[T] {
	total := uint256.NewInt(0)

	if validators != nil {
		for _, stake := range validators.Set {
			if stake == 0 {
				continue
			}

			total.Add(total, uint256.NewInt(uint64(stake)))
		}
	}

	return NewVoting[T](total)
}

// AddVote adds `power` to the tally for (chainID, blockNumber, blockHash).
// A nil power is treated as 0 and is a no-op.
func (v *Voting[T]) AddVote(extBlock T, power *uint256.Int) {
	if power == nil || power.IsZero() {
		return
	}

	id := extBlock.GetEntityID()

	blockVoting, ok := v.voting[id.ChainID]
	if !ok {
		blockVoting = make(map[uint64]map[[32]byte]*Entity[T])
		v.voting[id.ChainID] = blockVoting
	}

	hashVoting, ok := blockVoting[id.BlockNumber]
	if !ok {
		hashVoting = make(map[[32]byte]*Entity[T])
		blockVoting[id.BlockNumber] = hashVoting
	}

	curr, ok := hashVoting[id.BlockHash]
	if !ok {
		curr = &Entity[T]{
			voted:  uint256.NewInt(0),
			entity: &extBlock,
		}
		hashVoting[id.BlockHash] = curr
	}

	curr.voted.Add(curr.voted, power)
}

// Finalized returns all blocks that have votes >= Threshold().
func (v *Voting[T]) Finalized() []*T {
	th := v.threshold

	if th.IsZero() {
		return nil
	}

	var out []*T

	for _, byBlockNum := range v.voting {
		for _, byHash := range byBlockNum {
			for _, ent := range byHash {
				if ent.voted.Cmp(th) >= 0 {
					out = append(out, ent.entity)
				}
			}
		}
	}

	return out
}

func (v *Voting[T]) GetVotes(block T) *uint256.Int {
	id := block.GetEntityID()

	if blockVoting, ok := v.voting[id.ChainID]; ok {
		if hashVoting, ok := blockVoting[id.BlockNumber]; ok {
			if ent, ok := hashVoting[id.BlockHash]; ok {
				return ent.voted.Clone()
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
