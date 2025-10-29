package gosdk

import (
	"encoding/binary"
	"testing"

	cbor "github.com/fxamacker/cbor/v2"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func block(c, n uint64, h0 byte) apptypes.ExternalBlock {
	var h [32]byte

	h[0] = h0

	return apptypes.ExternalBlock{ChainID: c, BlockNumber: n, BlockHash: h}
}

func TestThreshold_ZeroTotal_ReturnsZero(t *testing.T) {
	t.Parallel()

	got := Threshold(nil)
	require.True(t, got.IsZero())

	got = Threshold(uint256.NewInt(0))
	require.True(t, got.IsZero())
}

func TestThreshold_FloorTwoThirdsPlusOne(t *testing.T) {
	t.Parallel()

	// table: total -> floor(2*total/3) + 1
	cases := []struct {
		total uint64
		exp   uint64
	}{
		{1, 1},  // floor(0.66)=0 -> +1=1
		{2, 2},  // floor(1.33)=1 -> +1=2
		{3, 3},  // floor(2)=2     -> +1=3
		{4, 3},  // floor(2.66)=2  -> +1=3
		{5, 4},  // floor(3.33)=3  -> +1=4
		{6, 5},  // floor(4)=4     -> +1=5
		{7, 5},  // floor(4.66)=4  -> +1=5
		{8, 6},  // floor(5.33)=5  -> +1=6
		{9, 7},  // floor(6)=6     -> +1=7
		{10, 7}, // floor(6.66)=6  -> +1=7
		{11, 8}, // floor(7.33)=7  -> +1=8
		{12, 9}, // floor(8)=8     -> +1=9
	}

	for _, tc := range cases {
		got := Threshold(uint256.NewInt(tc.total))
		exp := uint256.NewInt(tc.exp)
		require.Zerof(t, got.Cmp(exp), "total=%d got=%s exp=%s", tc.total, got.String(), exp.String())
	}
}

// --- NewVoting / NewVotingFromValidatorSet ---

func TestNewVoting_StoresCloneAndThreshold(t *testing.T) {
	t.Parallel()

	total := uint256.NewInt(10)
	v := NewVoting[apptypes.ExternalBlock](total)

	// internal copy made
	require.NotSame(t, total, v.totalVotingPower)
	require.Equal(t, total, v.totalVotingPower)

	// threshold matches helper
	require.Equal(t, Threshold(total), v.threshold)

	// mutating input must not change internal total
	total.Add(total, uint256.NewInt(5))
	require.Equal(t, uint256.NewInt(10), v.totalVotingPower)
}

func TestNewVotingFromValidatorSet_SumsStakes(t *testing.T) {
	t.Parallel()

	vs := &ValidatorSet{
		Set: map[ValidatorID]Stake{
			1: 100,
			2: 0,   // zero stakes ignored (has no effect)
			3: 250, // non-zero
		},
	}
	v := NewVotingFromValidatorSet[apptypes.ExternalBlock](vs)
	require.Equal(t, uint256.NewInt(350), v.totalVotingPower)
	require.Equal(t, Threshold(uint256.NewInt(350)), v.threshold)

	// nil validator set -> 0
	v2 := NewVotingFromValidatorSet[apptypes.ExternalBlock](nil)
	require.True(t, v2.totalVotingPower.IsZero())
	require.True(t, v2.threshold.IsZero())
}

// --- AddVote / GetVotes ---

func TestAddVote_And_GetVotes(t *testing.T) {
	t.Parallel()

	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(9)) // threshold = ceil(6)+1=7
	b := block(1, 100, 0xAA)

	// start 0
	require.True(t, v.GetVotes(b).IsZero())

	// add nil power -> no-op
	v.AddVote(b, nil, 0, 0)
	require.True(t, v.GetVotes(b).IsZero())

	// add non-zero
	v.AddVote(b, uint256.NewInt(3), 0, 0)
	require.Equal(t, uint256.NewInt(3), v.GetVotes(b))

	// accumulate
	v.AddVote(b, uint256.NewInt(4), 0, 1)
	require.Equal(t, uint256.NewInt(7), v.GetVotes(b)) // 3 + 4

	// duplicate vote - no-op
	v.AddVote(b, uint256.NewInt(4), 0, 1)
	require.Equal(t, uint256.NewInt(7), v.GetVotes(b)) // + 0

	// another block independent
	b2 := block(1, 101, 0xBB)
	v.AddVote(b2, uint256.NewInt(5), 0, 0)
	require.Equal(t, uint256.NewInt(5), v.GetVotes(b2))
	require.Equal(t, uint256.NewInt(7), v.GetVotes(b))
}

func TestAddVote_ZeroPower_NoOp(t *testing.T) {
	t.Parallel()

	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(5))
	b := block(2, 42, 0x01)

	v.AddVote(b, uint256.NewInt(0), 0, 0)
	require.True(t, v.GetVotes(b).IsZero())
}

// --- Finalized ---

func TestFinalizedBlocks_ZeroThreshold_ReturnsNil(t *testing.T) {
	t.Parallel()

	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(0)) // threshold=0
	require.Nil(t, v.Finalized())
}

func TestFinalizedBlocks_ReturnsAllMeetingThreshold(t *testing.T) {
	t.Parallel()

	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(10)) // threshold = ceil(6.66)+1 = 8
	// chain 1
	b1 := block(1, 100, 0x01)
	b2 := block(1, 100, 0x02)
	// chain 2
	b3 := block(2, 7, 0x03)

	// below threshold
	v.AddVote(b1, uint256.NewInt(7), 0, 0) // < 8

	// meets threshold
	v.AddVote(b2, uint256.NewInt(8), 0, 0) // == 8

	// above threshold
	v.AddVote(b3, uint256.NewInt(9), 0, 0) // > 8

	final := v.Finalized()
	// order is not specified; assert set membership
	require.Len(t, final, 2)

	got := map[uint64]map[uint64]map[[32]byte]struct{}{}
	for _, fb := range final {
		if got[fb.ChainID] == nil {
			got[fb.ChainID] = map[uint64]map[[32]byte]struct{}{}
		}

		if got[fb.ChainID][fb.BlockNumber] == nil {
			got[fb.ChainID][fb.BlockNumber] = map[[32]byte]struct{}{}
		}

		got[fb.ChainID][fb.BlockNumber][fb.BlockHash] = struct{}{}
	}

	_, ok12 := got[1][100][b2.BlockHash]
	require.True(t, ok12, "b2 should finalize")

	_, ok27 := got[2][7][b3.BlockHash]
	require.True(t, ok27, "b3 should finalize")

	// b1 should not be present
	if byBN, ok := got[1]; ok {
		if byHash, ok := byBN[100]; ok {
			_, bad := byHash[b1.BlockHash]
			require.False(t, bad, "b1 must not finalize")
		}
	}
}

func TestFinalizedBlocks_MultipleChainsAndBlocks(t *testing.T) {
	t.Parallel()

	// total = 6 -> ceil(4)+1 = 5
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(6))

	// c1-n1-hA: 5 => finalize
	bA := block(1, 1, 0xA)
	v.AddVote(bA, uint256.NewInt(2), 0, 0)
	v.AddVote(bA, uint256.NewInt(3), 0, 1)

	// c1-n1-hB: 4 => not finalize
	bB := block(1, 1, 0xB)
	v.AddVote(bB, uint256.NewInt(4), 0, 0)

	// c1-n2-hC: 5 => finalize
	bC := block(1, 2, 0xC)
	v.AddVote(bC, uint256.NewInt(5), 0, 1)

	// c2-n1-hD: 6 => finalize
	bD := block(2, 1, 0xD)
	v.AddVote(bD, uint256.NewInt(6), 0, 0)

	final := v.Finalized()
	require.Len(t, final, 3)

	// membership checks
	set := make(map[uint64]map[uint64]map[[32]byte]struct{})
	for _, fb := range final {
		if set[fb.ChainID] == nil {
			set[fb.ChainID] = map[uint64]map[[32]byte]struct{}{}
		}

		if set[fb.ChainID][fb.BlockNumber] == nil {
			set[fb.ChainID][fb.BlockNumber] = map[[32]byte]struct{}{}
		}

		set[fb.ChainID][fb.BlockNumber][fb.BlockHash] = struct{}{}
	}

	_, ok := set[1][1][bA.BlockHash]
	require.True(t, ok)

	_, ok = set[1][2][bC.BlockHash]
	require.True(t, ok)

	_, ok = set[2][1][bD.BlockHash]
	require.True(t, ok)

	// ensure bB not included
	if byBN, ok := set[1]; ok {
		if byH, ok := byBN[1]; ok {
			_, bad := byH[bB.BlockHash]
			require.False(t, bad)
		}
	}
}

// --- GetVotes returns a clone (immutability) ---

func TestGetVotes_ReturnsClone_NotAliased(t *testing.T) {
	t.Parallel()

	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(9))
	b := block(3, 33, 0xCC)
	v.AddVote(b, uint256.NewInt(4), 0, 0)

	got := v.GetVotes(b)
	require.Equal(t, uint256.NewInt(4), got)

	// mutate the returned copy; internal must stay intact
	got.Add(got, uint256.NewInt(10))
	require.Equal(t, uint256.NewInt(14), got)
	require.Equal(t, uint256.NewInt(4), v.GetVotes(b))
}

func TestFinalized_DoesNotMutate(t *testing.T) {
	t.Parallel()

	// total = 6 -> threshold = 5
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(6))

	bFin := block(1, 1, 0xAA) // will finalize
	bNop := block(1, 1, 0xBB) // won't finalize

	v.AddVote(bFin, uint256.NewInt(5), 0, 0) // == threshold
	v.AddVote(bNop, uint256.NewInt(4), 0, 0) // below threshold

	// Call Finalized (read-only)
	final1 := v.Finalized()
	require.Len(t, final1, 1)
	require.Equal(t, bFin.BlockHash, final1[0].BlockHash)

	// Ensure voting still contains both entries after Finalized()
	require.Equal(t, uint256.NewInt(5), v.GetVotes(bFin))
	require.Equal(t, uint256.NewInt(4), v.GetVotes(bNop))

	// A second Finalized() should return the same thing (no mutation)
	final2 := v.Finalized()
	require.Len(t, final2, 1)
	require.Equal(t, bFin.BlockHash, final2[0].BlockHash)
}

func TestPopFinalized_RemovesOnlyFinalized_AndPrunesEmpty(t *testing.T) {
	t.Parallel()

	// total = 9 -> threshold = 7 (ceil(6)+1)
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(9))

	// Same chain & block number, different hashes
	bFin := block(1, 100, 0x01) // finalize
	bNop := block(1, 100, 0x02) // not finalize

	// Different chain entirely, also finalize
	bFin2 := block(2, 7, 0x03)

	v.AddVote(bFin, uint256.NewInt(7), 0, 0)   // == threshold
	v.AddVote(bNop, uint256.NewInt(6), 0, 1)   // below threshold
	v.AddVote(bFin2, uint256.NewInt(10), 0, 2) // above threshold

	// Pop and verify returned set
	got := v.PopFinalized()
	// Order is not guaranteed; put in a set for assertions
	gotSet := map[uint64]map[uint64]map[[32]byte]struct{}{}
	for _, e := range got {
		if gotSet[e.ChainID] == nil {
			gotSet[e.ChainID] = map[uint64]map[[32]byte]struct{}{}
		}

		if gotSet[e.ChainID][e.BlockNumber] == nil {
			gotSet[e.ChainID][e.BlockNumber] = map[[32]byte]struct{}{}
		}

		gotSet[e.ChainID][e.BlockNumber][e.BlockHash] = struct{}{}
	}

	_, ok11h1 := gotSet[1][100][bFin.BlockHash]
	require.True(t, ok11h1)

	_, ok27h3 := gotSet[2][7][bFin2.BlockHash]
	require.True(t, ok27h3)
	require.Len(t, got, 2)

	// Finalized ones should be gone
	require.True(t, v.GetVotes(bFin).IsZero())
	require.True(t, v.GetVotes(bFin2).IsZero())

	// Non-finalized one must remain intact
	require.Equal(t, uint256.NewInt(6), v.GetVotes(bNop))

	// Pruning: chain 2 had only one hash entry; after pop it should be pruned.
	// Finalized() over an empty structure should return nil and not panic.
	require.Nil(t, v.Finalized())
}

func TestPopFinalized_NoThreshold_NoOp(t *testing.T) {
	t.Parallel()

	// total = 0 -> threshold = 0 -> PopFinalized should return nil and not mutate
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(0))
	b := block(3, 33, 0xCC)
	v.AddVote(b, uint256.NewInt(100), 0, 0) // any votes present

	got := v.PopFinalized()
	require.Nil(t, got)
	// Entry should still be there since threshold is zero (we return early)
	require.Equal(t, uint256.NewInt(100), v.GetVotes(b))
}

func openVotingDB(t *testing.T) kv.RwDB {
	t.Helper()
	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(t.TempDir()).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				ExternalBlockVotingBucket: {},
				CheckpointVotingBucket:    {},
			}
		}).Open()
	require.NoError(t, err)

	return db
}

func mkEB(chain, num uint64, b0 byte) apptypes.ExternalBlock {
	var h [32]byte

	h[0] = b0

	return apptypes.ExternalBlock{ChainID: chain, BlockNumber: num, BlockHash: h}
}

func mkCP(chain, num uint64, b0 byte) apptypes.Checkpoint {
	var h [32]byte

	h[0] = b0

	return apptypes.Checkpoint{ChainID: chain, BlockNumber: num, BlockHash: h}
}

func TestStoreNewVotingFromStorage_ExternalBlock_RoundTrip(t *testing.T) {
	db := openVotingDB(t)
	defer db.Close()

	// validators: total=10+20+30 = 60; threshold = ceil(2/3*60)+1 = 41
	valset := &ValidatorSet{Set: map[ValidatorID]Stake{
		1: 10, 2: 20, 3: 30,
	}}
	v := NewVotingFromValidatorSet[apptypes.ExternalBlock](valset)

	// Two blocks to tally
	ebA := mkEB(1, 100, 0xAA)
	ebB := mkEB(1, 101, 0xBB)

	// Add votes, including a duplicate signer (should be ignored)
	v.AddVote(ebA, uint256.NewInt(10), 7, 1) // first time creator 1
	v.AddVote(ebA, uint256.NewInt(10), 7, 1) // duplicate creator 1 -> ignored
	v.AddVote(ebA, uint256.NewInt(20), 7, 2) // creator 2
	v.AddVote(ebB, uint256.NewInt(30), 7, 3) // creator 3

	require.Equal(t, uint256.NewInt(30), v.GetVotes(ebA))
	require.Equal(t, uint256.NewInt(30), v.GetVotes(ebB))

	// Store
	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, v.StoreProgress(rw))
	require.NoError(t, rw.Commit())
	rw.Rollback()

	// Load
	ro, err := db.BeginRo(t.Context())
	require.NoError(t, err)
	restored, err := NewVotingFromStorage(ro, apptypes.MakeExternalBlock, valset)
	require.NoError(t, err)
	ro.Rollback()

	// Votes must match
	require.Equal(t, uint256.NewInt(30), restored.GetVotes(ebA))
	require.Equal(t, uint256.NewInt(30), restored.GetVotes(ebB))

	// Ensure signers list has unique creators, sorted
	id := ebA.GetEntityID()

	ent := restored.voting[id.ChainID][id.BlockNumber][id.BlockHash]
	require.NotNil(t, ent)
	require.Len(t, ent.Signers, 2)

	require.Equal(t, uint32(1), ent.Signers[0].Creator)
	require.Equal(t, uint32(2), ent.Signers[1].Creator)

	// entityID pointer should be set (so Finalized/PopFinalized returns non-nil)
	require.NotNil(t, ent.entityID)

	// Force finalize ebA by adding one more vote (creator 3) and store again
	rw, err = db.BeginRw(t.Context())
	require.NoError(t, err)
	restored.AddVote(ebA, uint256.NewInt(30), 7, 3) // total 60 for A
	require.NoError(t, restored.StoreProgress(rw))
	require.NoError(t, rw.Commit())
	rw.Rollback()

	ro, err = db.BeginRo(t.Context())
	require.NoError(t, err)
	restored2, err := NewVotingFromStorage(ro, apptypes.MakeExternalBlock, valset)
	require.NoError(t, err)
	ro.Rollback()

	// A should be finalized now; B not yet
	final := restored2.Finalized()
	require.Len(t, final, 1)
	require.Equal(t, ebA, *final[0])

	// PopFinalized should remove A
	popped := restored2.PopFinalized()
	require.Len(t, popped, 1)
	require.Equal(t, ebA, *popped[0])

	// After pop, A removed, B remains with its votes
	require.Equal(t, uint256.NewInt(0), restored2.GetVotes(ebA))
	require.Equal(t, uint256.NewInt(30), restored2.GetVotes(ebB))
}

func TestStoreNewVotingFromStorage_Checkpoint_RoundTrip(t *testing.T) {
	db := openVotingDB(t)
	defer db.Close()

	valset := &ValidatorSet{Set: map[ValidatorID]Stake{
		10: 15, 11: 15, 12: 20,
	}}
	v := NewVotingFromValidatorSet[apptypes.Checkpoint](valset)
	cp := mkCP(100, 5, 0x42)

	// Votes with unique creators
	v.AddVote(cp, uint256.NewInt(15), 9, 10)
	v.AddVote(cp, uint256.NewInt(15), 9, 11)
	require.Equal(t, uint256.NewInt(30), v.GetVotes(cp))

	// Persist
	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, v.StoreProgress(rw))
	require.NoError(t, rw.Commit())
	rw.Rollback()

	// Reload
	ro, err := db.BeginRo(t.Context())
	require.NoError(t, err)
	restored, err := NewVotingFromStorage(ro, apptypes.MakeCheckpoint, valset)
	require.NoError(t, err)
	ro.Rollback()

	require.Equal(t, uint256.NewInt(30), restored.GetVotes(cp))

	// Ensure key shape was correct and decodable
	rro, err := db.BeginRo(t.Context())
	require.NoError(t, err)

	defer rro.Rollback()

	cur, err := rro.Cursor(CheckpointVotingBucket)
	require.NoError(t, err)

	defer cur.Close()

	k, vraw, err := cur.First()
	require.NoError(t, err)
	require.NotNil(t, k)

	// Check key layout: 8|8|32
	require.Len(t, k, 8+8+32)
	kChain := binary.BigEndian.Uint64(k[:8])
	kNum := binary.BigEndian.Uint64(k[8:16])

	var kHash [32]byte
	copy(kHash[:], k[16:])
	require.Equal(t, cp.ChainID, kChain)
	require.Equal(t, cp.BlockNumber, kNum)
	require.Equal(t, cp.BlockHash, kHash)

	// Check it decodes to Entity[Checkpoint]
	var ent Entity[apptypes.Checkpoint]
	require.NoError(t, cbor.Unmarshal(vraw, &ent))
	require.NotNil(t, ent.Voted)
	require.Equal(t, uint256.NewInt(30), ent.Voted)
}

func TestStoreNewVotingFromStorage_PopFinalized_PrunesState(t *testing.T) {
	db := openVotingDB(t)
	defer db.Close()

	vs := &ValidatorSet{
		Set: map[ValidatorID]Stake{1: 50, 2: 50},
	} // total=100, threshold= (ceil(66)+1)=67
	v := NewVotingFromValidatorSet[apptypes.ExternalBlock](vs)

	eb1 := mkEB(7, 700, 0x01)
	eb2 := mkEB(7, 700, 0x02)

	// Give eb1 enough votes to finalize; eb2 not enough
	v.AddVote(eb1, uint256.NewInt(50), 1, 1)
	v.AddVote(eb1, uint256.NewInt(50), 1, 2) // 100 -> finalized
	v.AddVote(eb2, uint256.NewInt(50), 1, 1) // 50 -> not finalized

	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, v.StoreProgress(rw))
	require.NoError(t, rw.Commit())
	rw.Rollback()

	// Reload and Pop
	ro, err := db.BeginRo(t.Context())
	require.NoError(t, err)
	restored, err := NewVotingFromStorage(ro, apptypes.MakeExternalBlock, vs)
	require.NoError(t, err)
	ro.Rollback()

	popped := restored.PopFinalized()
	require.Len(t, popped, 1)
	require.Equal(t, eb1, *popped[0])

	// Ensure eb1 fully pruned from nested maps; eb2 remains
	require.Equal(t, uint256.NewInt(0), restored.GetVotes(eb1))
	require.Equal(t, uint256.NewInt(50), restored.GetVotes(eb2))
}

func TestStoreNewVotingFromStorage_DoubleVoteProtectionPersists(t *testing.T) {
	db := openVotingDB(t)
	defer db.Close()

	vs := &ValidatorSet{Set: map[ValidatorID]Stake{1: 10, 2: 20}}
	v := NewVotingFromValidatorSet[apptypes.ExternalBlock](vs)
	eb := mkEB(9, 9, 0x9)

	// two different epochs but same creator should still be treated as single signer entry
	// (your current logic dedups by Creator)
	v.AddVote(eb, uint256.NewInt(10), 111, 1)
	v.AddVote(eb, uint256.NewInt(10), 112, 1) // duplicate creator
	v.AddVote(eb, uint256.NewInt(20), 111, 2)

	require.Equal(t, uint256.NewInt(30), v.GetVotes(eb))

	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, v.StoreProgress(rw))
	require.NoError(t, rw.Commit())
	rw.Rollback()

	ro, err := db.BeginRo(t.Context())
	require.NoError(t, err)
	restored, err := NewVotingFromStorage(ro, apptypes.MakeExternalBlock, vs)
	require.NoError(t, err)
	ro.Rollback()

	require.Equal(t, uint256.NewInt(30), restored.GetVotes(eb))

	// Verify signers are unique & sorted by Creator
	id := eb.GetEntityID()
	ent := restored.voting[id.ChainID][id.BlockNumber][id.BlockHash]
	require.NotNil(t, ent)
	require.Len(t, ent.Signers, 2)
	require.Equal(t, uint32(1), ent.Signers[0].Creator)
	require.Equal(t, uint32(2), ent.Signers[1].Creator)

	// Adding the same creator again should not change votes
	restored.AddVote(eb, uint256.NewInt(10), 200, 1)
	require.Equal(t, uint256.NewInt(30), restored.GetVotes(eb))
}
