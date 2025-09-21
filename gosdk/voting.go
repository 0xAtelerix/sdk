package gosdk

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"slices"
	"sort"

	"github.com/fxamacker/cbor/v2"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Voting is not thread-safe
type Voting[T apptypes.ExternalEntity] struct {
	totalVotingPower *uint256.Int
	threshold        *uint256.Int

	// map[ChainID][BlockNumber][BlockHash]votes
	voting map[uint64]map[uint64]map[[32]byte]*Entity[T]
}

type Entity[T apptypes.ExternalEntity] struct {
	entityID *T           `cbor:"-"` //nolint:revive // it's cbor, it works differently
	Voted    *uint256.Int `cbor:"1,keyasint"`
	Signers  []Signer     `cbor:"2,keyasint"` // sorted
}

// epoch + creator identifies a signer
type Signer struct {
	Epoch   uint32 `cbor:"1,keyasint"`
	Creator uint32 `cbor:"2,keyasint"`
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
	total := getTotalVoting(validators)

	return NewVoting[T](total)
}

func (v *Voting[T]) SetValset(validators *ValidatorSet) {
	total := getTotalVoting(validators)
	v.totalVotingPower.Set(total)
	v.threshold.Set(Threshold(total))
}

// NewVotingFromStorage restores voting state of the given entity type from DB.
// It expects entries stored by Voting[T].StoreProgress.
// `validators` is optional: used to set the total voting power / threshold.
func NewVotingFromStorage[T apptypes.ExternalEntity](
	tx kv.Tx,
	constr func(chainID uint64, blockNum uint64, hash [32]byte) T,
	validators *ValidatorSet,
) (*Voting[T], error) {
	// Decide bucket
	var (
		zero   T
		bucket string
	)

	switch any(zero).(type) {
	case apptypes.ExternalBlock:
		bucket = ExternalBlockVotingBucket
	case apptypes.Checkpoint:
		bucket = CheckpointVotingBucket
	default:
		return nil, fmt.Errorf("%w %T", ErrUnsupportedEntityType, zero)
	}

	total := getTotalVoting(validators)
	v := NewVoting[T](total)

	err := tx.ForEach(bucket, nil, func(k, val []byte) error {
		if len(k) != 8+8+32 {
			return fmt.Errorf("%w %q %v", ErrMalformedKey, bucket, k)
		}

		chainID := binary.BigEndian.Uint64(k[0:8])
		blockNum := binary.BigEndian.Uint64(k[8:16])

		var hash [32]byte
		copy(hash[:], k[16:])

		// Unmarshal Entity
		var ent Entity[T]
		if err := cbor.Unmarshal(val, &ent); err != nil {
			return fmt.Errorf("unmarshal voting entity: %w", err)
		}

		restored := constr(chainID, blockNum, hash)
		ent.entityID = &restored

		// Put back into map
		byBlockNum, ok := v.voting[chainID]
		if !ok {
			byBlockNum = make(map[uint64]map[[32]byte]*Entity[T])
			v.voting[chainID] = byBlockNum
		}

		byHash, ok := byBlockNum[blockNum]
		if !ok {
			byHash = make(map[[32]byte]*Entity[T])
			byBlockNum[blockNum] = byHash
		}

		byHash[hash] = &ent

		return nil
	})
	if err != nil {
		return nil, err
	}

	return v, nil
}

// AddVote adds `power` to the tally for (chainID, blockNumber, blockHash).
// A nil power is treated as 0 and is a no-op.
func (v *Voting[T]) AddVote(extBlock T, power *uint256.Int, epoch uint32, creator uint32) {
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
			Voted:    uint256.NewInt(0),
			entityID: &extBlock,
		}

		hashVoting[id.BlockHash] = curr
	}

	if len(curr.Signers) > 0 && ok {
		voteIdx := sort.Search(len(curr.Signers), func(i int) bool {
			return curr.Signers[i].Creator >= creator
		})

		if voteIdx < len(curr.Signers) && curr.Signers[voteIdx].Creator == creator {
			return
		}
	}

	curr.Voted.Add(curr.Voted, power)

	curr.Signers = append(curr.Signers, Signer{
		Epoch:   epoch,
		Creator: creator,
	})

	slices.SortFunc(curr.Signers, cmpSigners)
}

func (v *Voting[T]) Finalized() []*T {
	return v.finalized(false)
}

// PopFinalized returns all entities that meet the threshold
// and removes them from the Voting accumulator.
func (v *Voting[T]) PopFinalized() []*T {
	return v.finalized(true)
}

// Finalized returns all blocks that have votes >= Threshold().
//
//nolint:revive // with a flat it is simpler
func (v *Voting[T]) finalized(remove bool) []*T {
	th := v.threshold

	if th.IsZero() {
		return nil
	}

	var out []*T

	for chainID, byBlockNum := range v.voting {
		for blockNum, byHash := range byBlockNum {
			for h, ent := range byHash {
				if ent.Voted.Cmp(th) >= 0 {
					out = append(out, ent.entityID)

					if remove {
						// delete this hash entry
						delete(byHash, h)
					}
				}
			}

			// prune empty hash map
			if remove && len(byHash) == 0 {
				delete(byBlockNum, blockNum)
			}
		}

		// prune empty block-number map
		if remove && len(byBlockNum) == 0 {
			delete(v.voting, chainID)
		}
	}

	return out
}

func (v *Voting[T]) GetVotes(block T) *uint256.Int {
	id := block.GetEntityID()

	if blockVoting, ok := v.voting[id.ChainID]; ok {
		if hashVoting, ok := blockVoting[id.BlockNumber]; ok {
			if ent, ok := hashVoting[id.BlockHash]; ok {
				return ent.Voted.Clone()
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

func getTotalVoting(validators *ValidatorSet) *uint256.Int {
	total := uint256.NewInt(0)

	if validators != nil {
		for _, stake := range validators.Set {
			if stake == 0 {
				continue
			}

			total.Add(total, uint256.NewInt(uint64(stake)))
		}
	}

	return total
}

// StoreProgress writes all current vote tallies for this Voting[T] into the
// corresponding bucket. Idempotent: re-puts overwrite previous snapshots.
func (v *Voting[T]) StoreProgress(tx kv.RwTx) error {
	if v == nil {
		return nil
	}

	// Pick bucket by T
	var (
		zero   T
		bucket string
	)

	switch any(zero).(type) {
	case apptypes.ExternalBlock:
		bucket = ExternalBlockVotingBucket
	case apptypes.Checkpoint:
		bucket = CheckpointVotingBucket
	default:
		return fmt.Errorf("%w %T", ErrUnsupportedEntityType, zero)
	}

	err := tx.ClearBucket(bucket)
	if err != nil {
		return err
	}

	// Walk the map and persist each entry.
	for chainID, byBlockNum := range v.voting {
		for blockNum, byHash := range byBlockNum {
			for h, ent := range byHash {
				key := VotingKey(chainID, blockNum, h)

				val, err := cbor.Marshal(ent)
				if err != nil {
					return fmt.Errorf("marshal voting entry c=%d n=%d: %w", chainID, blockNum, err)
				}

				if err = tx.Put(bucket, key[:], val); err != nil {
					return fmt.Errorf("put voting entry c=%d n=%d: %w", chainID, blockNum, err)
				}
			}
		}
	}

	return nil
}

func cmpSigners(s1, s2 Signer) int {
	return cmp.Compare(s1.Creator, s2.Creator)
}

func VotingKey(chainID uint64, blockNum uint64, blockHash [32]byte) []byte {
	// Key: 8 bytes chainID | 8 bytes blockNum | 32 bytes hash
	var key [8 + 8 + 32]byte
	binary.BigEndian.PutUint64(key[0:8], chainID)
	binary.BigEndian.PutUint64(key[8:16], blockNum)
	copy(key[16:], blockHash[:])

	return key[:]
}
