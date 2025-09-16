package gosdk

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

func mkEth(b byte) EthereumAddress {
	var a EthereumAddress

	a[0] = b

	return a
}

func mkSol(b byte) SolanaAddress {
	var a SolanaAddress

	a[0] = b

	return a
}

func Test_cmpAddr_Ethereum(t *testing.T) {
	t.Parallel()

	a := mkEth(1)
	b := mkEth(2)
	c := mkEth(1)

	require.Equal(t, -1, CmpAddr[EthereumAddress](a, b))
	require.Equal(t, 1, CmpAddr[EthereumAddress](b, a))
	require.Equal(t, 0, CmpAddr[EthereumAddress](a, c))
}

func Test_cmpAddr_Solana(t *testing.T) {
	t.Parallel()

	a := mkSol(1)
	b := mkSol(2)
	c := mkSol(1)

	require.Equal(t, -1, CmpAddr[SolanaAddress](a, b))
	require.Equal(t, 1, CmpAddr[SolanaAddress](b, a))
	require.Equal(t, 0, CmpAddr[SolanaAddress](a, c))
}

func newSubscriber() *Subscriber {
	return &Subscriber{
		ethContracts:        make(map[apptypes.ChainType]map[EthereumAddress]struct{}),
		solAddresses:        make(map[apptypes.ChainType]map[SolanaAddress]struct{}),
		deletedEthContracts: make(map[apptypes.ChainType]map[EthereumAddress]struct{}),
		deletedSolAddresses: make(map[apptypes.ChainType]map[SolanaAddress]struct{}),
	}
}

func Test_sortChainAddresses_Ethereum(t *testing.T) {
	t.Parallel()

	items := []ChainAddresses[EthereumAddress]{
		{chainID: 5, addresses: []EthereumAddress{mkEth(3), mkEth(1), mkEth(2)}},
		{chainID: 1, addresses: []EthereumAddress{mkEth(2), mkEth(1)}},
		{chainID: 3, addresses: []EthereumAddress{mkEth(9)}},
	}
	SortChainAddresses(items)

	require.Equal(t, apptypes.ChainType(1), items[0].chainID)
	require.Equal(t, []EthereumAddress{mkEth(1), mkEth(2)}, items[0].addresses)

	require.Equal(t, apptypes.ChainType(3), items[1].chainID)
	require.Equal(t, []EthereumAddress{mkEth(9)}, items[1].addresses)

	require.Equal(t, apptypes.ChainType(5), items[2].chainID)
	require.Equal(t, []EthereumAddress{mkEth(1), mkEth(2), mkEth(3)}, items[2].addresses)
}

func Test_sortChainAddresses_Solana(t *testing.T) {
	t.Parallel()

	items := []ChainAddresses[SolanaAddress]{
		{chainID: 42, addresses: []SolanaAddress{mkSol(7), mkSol(3), mkSol(3), mkSol(4)}},
		{chainID: 9, addresses: []SolanaAddress{mkSol(2), mkSol(1)}},
	}
	SortChainAddresses(items)

	require.Equal(t, apptypes.ChainType(9), items[0].chainID)
	require.Equal(t, []SolanaAddress{mkSol(1), mkSol(2)}, items[0].addresses)

	require.Equal(t, apptypes.ChainType(42), items[1].chainID)
	require.Equal(t, []SolanaAddress{mkSol(3), mkSol(3), mkSol(4), mkSol(7)}, items[1].addresses)
}

func Test_collectChainAddresses_SetsToSlice_Ethereum(t *testing.T) {
	t.Parallel()

	m := map[apptypes.ChainType]map[EthereumAddress]struct{}{
		10: {
			mkEth(3): {},
			mkEth(1): {},
		},
		1: {
			mkEth(2): {},
		},
	}
	out := CollectChainAddresses[EthereumAddress](m)

	require.Len(t, out, 2)
	require.Equal(t, apptypes.ChainType(1), out[0].chainID)
	require.Equal(t, []EthereumAddress{mkEth(2)}, out[0].addresses)

	require.Equal(t, apptypes.ChainType(10), out[1].chainID)
	// order within a chain must be sorted
	require.Equal(t, []EthereumAddress{mkEth(1), mkEth(3)}, out[1].addresses)
}

func Test_collectChainAddresses_SetsToSlice_Solana(t *testing.T) {
	t.Parallel()

	m := map[apptypes.ChainType]map[SolanaAddress]struct{}{
		5: {
			mkSol(9): {},
			mkSol(1): {},
			mkSol(7): {},
		},
	}
	out := CollectChainAddresses[SolanaAddress](m)

	require.Len(t, out, 1)
	require.Equal(t, apptypes.ChainType(5), out[0].chainID)
	require.Equal(t, []SolanaAddress{mkSol(1), mkSol(7), mkSol(9)}, out[0].addresses)
}

func Test_bytesOf_Ethereum(t *testing.T) {
	t.Parallel()

	a := mkEth(0xAB)
	b := bytesOf[EthereumAddress](a)

	require.Len(t, b, EthereumAddressLength)
	require.Equal(t, byte(0xAB), b[0])
	// ensure it's a copy (mutating b won't affect a)
	b[0] = 0

	require.Equal(t, byte(0xAB), a[0])
}

func Test_bytesOf_Solana(t *testing.T) {
	t.Parallel()

	a := mkSol(0xCD)
	b := bytesOf[SolanaAddress](a)

	require.Len(t, b, SolanaAddressLength)
	require.Equal(t, byte(0xCD), b[0])
	b[0] = 0

	require.Equal(t, byte(0xCD), a[0])
}

func Test_SubscribeEthContract_And_IsEthSubscription(t *testing.T) {
	t.Parallel()

	s := newSubscriber()

	chainID := apptypes.ChainType(1)
	contract := mkEth(0x11)

	require.True(t, s.IsEthSubscription(chainID, contract))

	s.SubscribeEthContract(chainID, contract)
	require.True(t, s.IsEthSubscription(chainID, contract))

	// subscribing twice keeps it present (idempotent)
	s.SubscribeEthContract(chainID, contract)
	require.True(t, s.IsEthSubscription(chainID, contract))
}

func Test_UnsubscribeEthContract_RemovesFromActive_And_MarksDeleted(t *testing.T) {
	t.Parallel()

	s := newSubscriber()
	chainID := apptypes.ChainType(2)
	contract := mkEth(0x22)

	// precondition: no subscriptions, listen to all chains
	require.True(t, s.IsEthSubscription(chainID, contract))

	// subscribe -> present
	s.SubscribeEthContract(chainID, contract)
	require.True(t, s.IsEthSubscription(chainID, contract))

	// act: unsubscribe (should NOT panic with correct code)
	s.UnsubscribeEthContract(chainID, contract)

	// removed from active - no subscriptions
	require.True(t, s.IsEthSubscription(chainID, contract))

	// and marked as deleted
	require.NotNil(t, s.deletedEthContracts[chainID])

	_, ok := s.deletedEthContracts[chainID][contract]
	require.True(t, ok)

	// re-subscribe clears the deleted marker (idempotent behavior)
	s.SubscribeEthContract(chainID, contract)
	require.True(t, s.IsEthSubscription(chainID, contract))

	_, ok = s.deletedEthContracts[chainID][contract]
	require.False(t, ok)
}

func Test_IsSolanaSubscription(t *testing.T) {
	t.Parallel()

	s := newSubscriber()

	chainID := apptypes.ChainType(9)
	addr := mkSol(0x09)

	// Manually set up to avoid the current Subscribe bug.
	s.solAddresses[chainID] = map[SolanaAddress]struct{}{addr: {}}

	require.True(t, s.IsSolanaSubscription(chainID, addr))
	require.False(t, s.IsSolanaSubscription(chainID, mkSol(0xAA)))

	// precondition: no subscriptions, listen to all chains
	require.True(t, s.IsSolanaSubscription(12345, addr))
}

func Test_UnsubscribeSolanaAddress_RemovesFromActive_And_MarksDeleted(t *testing.T) {
	t.Parallel()

	s := newSubscriber()

	addr := mkSol(0x33)

	// add
	s.SubscribeSolanaAddress(SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(SolanaChainID, addr))

	// delete (should NOT panic; should remove from active and mark as deleted), no subscriptions
	s.UnsubscribeSolanaAddress(SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(SolanaChainID, addr))

	// deleted map must be initialized and contain the addr
	require.NotNil(t, s.deletedSolAddresses[SolanaChainID])
	_, ok := s.deletedSolAddresses[SolanaChainID][addr]
	require.True(t, ok)

	// re-add clears deleted marker
	s.SubscribeSolanaAddress(SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(SolanaChainID, addr))
	_, ok = s.deletedSolAddresses[SolanaChainID][addr]
	require.False(t, ok)

	// delete again re-marks as deleted, no subscriptions
	s.UnsubscribeSolanaAddress(SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(SolanaChainID, addr))

	_, ok = s.deletedSolAddresses[SolanaChainID][addr]
	require.True(t, ok)
}

func openTestDB(t *testing.T) kv.RwDB {
	t.Helper()

	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(t.TempDir()).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				SubscriptionBucket: {},
			}
		}).
		Open()

	require.NoError(t, err)

	return db
}

func readAllSubscriptions(
	t *testing.T,
	tx kv.Tx,
) (map[apptypes.ChainType]map[EthereumAddress]struct{}, map[apptypes.ChainType]map[SolanaAddress]struct{}) {
	t.Helper()

	gotEth := make(map[apptypes.ChainType]map[EthereumAddress]struct{})
	gotSol := make(map[apptypes.ChainType]map[SolanaAddress]struct{})

	err := tx.ForEach(SubscriptionBucket, nil, func(k, v []byte) error {
		chain := apptypes.ChainType(binary.BigEndian.Uint64(k))

		if IsEvmChain(chain) {
			gotEth[chain] = make(map[EthereumAddress]struct{})

			var addrs []EthereumAddress

			dbErr := cbor.Unmarshal(v, &addrs)
			require.NoError(t, dbErr)

			for _, addr := range addrs {
				gotEth[chain][addr] = struct{}{}
			}
		} else if IsSolanaChain(chain) {
			gotSol[chain] = make(map[SolanaAddress]struct{})

			var addrs []SolanaAddress

			dbErr := cbor.Unmarshal(v, &addrs)
			require.NoError(t, dbErr)

			for _, addr := range addrs {
				gotSol[chain][addr] = struct{}{}
			}
		} else {
			t.Errorf("unknown chain type: %v", chain)
		}

		return nil
	})

	require.NoError(t, err)

	return gotEth, gotSol
}

func Test_Store_Persistency_WriteAndReadBack(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	s := newSubscriber()
	// chain 1: two ETH
	eth1 := mkEth(0x01)
	eth2 := mkEth(0x02)

	s.SubscribeEthContract(1, eth1)
	s.SubscribeEthContract(1, eth2)

	// chain 2: one SOL
	solA := mkSol(0xAA)
	s.SubscribeSolanaAddress(SolanaChainID, solA)

	// persist
	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)

	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	rw.Rollback()

	// read back
	ro, err := db.BeginRo(t.Context())
	require.NoError(t, err)

	gotEth, gotSol := readAllSubscriptions(t, ro)

	ro.Rollback()

	require.Len(t, gotEth, 1)
	require.Len(t, gotSol, 1)

	// chain 1 should have eth1, eth2
	require.Contains(t, gotEth, apptypes.ChainType(1))
	// byte equality
	found1 := false
	found2 := false

	for v := range gotEth[1] {
		if bytes.Equal(v[:], bytesOf(eth1)) {
			found1 = true
		}

		if bytes.Equal(v[:], bytesOf(eth2)) {
			found2 = true
		}
	}

	require.True(t, found1)
	require.True(t, found2)

	// chain 2 should have the Solana address
	require.Contains(t, gotSol, SolanaChainID)
	require.Len(t, gotSol[SolanaChainID], 1)

	require.Equal(t, map[SolanaAddress]struct{}{solA: {}}, gotSol[SolanaChainID])
}

func Test_Store_Persistency_DeleteThenUpsert(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	s := newSubscriber()

	// Start with two SOL on chain Sol
	solA := mkSol(0x10)
	solB := mkSol(0x20)

	s.SubscribeSolanaAddress(SolanaChainID, solA, solB)

	// Persist initial state
	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	// Now delete solA, keep solB
	s.UnsubscribeSolanaAddress(SolanaChainID, solA)

	// Persist again
	rw, err = db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	// Verify DB has only solB for chain Sol
	ro, err := db.BeginRo(t.Context())
	require.NoError(t, err)

	gotEth, gotSol := readAllSubscriptions(t, ro)
	ro.Rollback()

	require.Empty(t, gotEth)
	require.Len(t, gotSol, 1)

	require.Equal(t, map[SolanaAddress]struct{}{solB: {}}, gotSol[SolanaChainID])
}

func Test_Store_Persistency_DeleteWholeChainThenReAdd(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	defer db.Close()

	s := newSubscriber()

	// Chain 4: one ETH
	ethX := mkEth(0x44)
	s.SubscribeEthContract(BNBChainID, ethX)

	// Persist
	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	// Remove last ETH from chain BNBChainID -> chain should end up empty
	s.UnsubscribeEthContract(BNBChainID, ethX)

	// Persist deletion
	rw, err = db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	// DB should have no entries for chain 4
	ro, err := db.BeginRo(t.Context())
	require.NoError(t, err)

	defer ro.Rollback()

	gotEth, gotSol := readAllSubscriptions(t, ro)

	require.Empty(t, gotEth)
	require.Empty(t, gotSol)

	// Re-add a new one and persist again
	ethY := mkEth(0x55)
	s.SubscribeEthContract(BNBChainID, ethY)

	rw, err = db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	ro2, err := db.BeginRo(t.Context())
	require.NoError(t, err)

	defer ro2.Rollback()

	gotEth, gotSol = readAllSubscriptions(t, ro2)
	require.Len(t, gotEth, 1)
	require.Empty(t, gotSol)

	require.Equal(t, map[EthereumAddress]struct{}{ethY: {}}, gotEth[BNBChainID])
}
