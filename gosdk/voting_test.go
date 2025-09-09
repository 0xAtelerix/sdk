package gosdk

import (
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func block(c, n uint64, h0 byte) apptypes.ExternalBlock {
	var h [32]byte

	h[0] = h0

	return apptypes.ExternalBlock{ChainID: c, BlockNumber: n, BlockHash: h}
}

// --- Threshold() tests (ceil version) ---

func TestThreshold_ZeroTotal_ReturnsZero(t *testing.T) {
	got := Threshold(nil)
	require.True(t, got.IsZero())

	got = Threshold(uint256.NewInt(0))
	require.True(t, got.IsZero())
}

func TestThreshold_CeilTwoThirdsPlusOne(t *testing.T) {
	// table: total -> ceil(2*total/3) + 1
	cases := []struct {
		total uint64
		exp   uint64
	}{
		{1, 2},  // ceil(0.66)=1 -> +1=2
		{2, 3},  // ceil(1.33)=2 -> +1=3
		{3, 3},  // ceil(2)=2 -> +1=3
		{4, 4},  // ceil(2.66)=3 -> +1=4
		{5, 5},  // ceil(3.33)=4 -> +1=5
		{6, 5},  // ceil(4)=4 -> +1=5
		{7, 6},  // ceil(4.66)=5 -> +1=6
		{8, 7},  // ceil(5.33)=6 -> +1=7
		{9, 7},  // ceil(6)=6 -> +1=7
		{10, 8}, // ceil(6.66)=7 -> +1=8
		{11, 9}, // ceil(7.33)=8 -> +1=9
		{12, 9}, // ceil(8)=8 -> +1=9
	}
	for _, tc := range cases {
		got := Threshold(uint256.NewInt(tc.total))
		require.Equalf(t, uint256.NewInt(tc.exp), got, "total=%d", tc.total)
	}
}

// --- NewVoting / NewVotingFromValidatorSet ---

func TestNewVoting_StoresCloneAndThreshold(t *testing.T) {
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
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(9)) // threshold = ceil(6)+1=7
	b := block(1, 100, 0xAA)

	// start 0
	require.True(t, v.GetVotes(b).IsZero())

	// add nil power -> no-op
	v.AddVote(b, nil)
	require.True(t, v.GetVotes(b).IsZero())

	// add non-zero
	v.AddVote(b, uint256.NewInt(3))
	require.Equal(t, uint256.NewInt(3), v.GetVotes(b))

	// accumulate
	v.AddVote(b, uint256.NewInt(4))
	require.Equal(t, uint256.NewInt(7), v.GetVotes(b)) // 3 + 4

	// another block independent
	b2 := block(1, 101, 0xBB)
	v.AddVote(b2, uint256.NewInt(5))
	require.Equal(t, uint256.NewInt(5), v.GetVotes(b2))
	require.Equal(t, uint256.NewInt(7), v.GetVotes(b))
}

func TestAddVote_ZeroPower_NoOp(t *testing.T) {
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(5))
	b := block(2, 42, 0x01)

	v.AddVote(b, uint256.NewInt(0))
	require.True(t, v.GetVotes(b).IsZero())
}

// --- Finalized ---

func TestFinalizedBlocks_ZeroThreshold_ReturnsNil(t *testing.T) {
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(0)) // threshold=0
	require.Nil(t, v.Finalized())
}

func TestFinalizedBlocks_ReturnsAllMeetingThreshold(t *testing.T) {
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(10)) // threshold = ceil(6.66)+1 = 8
	// chain 1
	b1 := block(1, 100, 0x01)
	b2 := block(1, 100, 0x02)
	// chain 2
	b3 := block(2, 7, 0x03)

	// below threshold
	v.AddVote(b1, uint256.NewInt(7)) // < 8

	// meets threshold
	v.AddVote(b2, uint256.NewInt(8)) // == 8

	// above threshold
	v.AddVote(b3, uint256.NewInt(9)) // > 8

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
	// total = 6 -> ceil(4)+1 = 5
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(6))

	// c1-n1-hA: 5 => finalize
	bA := block(1, 1, 0xA)
	v.AddVote(bA, uint256.NewInt(2))
	v.AddVote(bA, uint256.NewInt(3))

	// c1-n1-hB: 4 => not finalize
	bB := block(1, 1, 0xB)
	v.AddVote(bB, uint256.NewInt(4))

	// c1-n2-hC: 5 => finalize
	bC := block(1, 2, 0xC)
	v.AddVote(bC, uint256.NewInt(5))

	// c2-n1-hD: 6 => finalize
	bD := block(2, 1, 0xD)
	v.AddVote(bD, uint256.NewInt(6))

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
	v := NewVoting[apptypes.ExternalBlock](uint256.NewInt(9))
	b := block(3, 33, 0xCC)
	v.AddVote(b, uint256.NewInt(4))

	got := v.GetVotes(b)
	require.Equal(t, uint256.NewInt(4), got)

	// mutate the returned copy; internal must stay intact
	got.Add(got, uint256.NewInt(10))
	require.Equal(t, uint256.NewInt(14), got)
	require.Equal(t, uint256.NewInt(4), v.GetVotes(b))
}
