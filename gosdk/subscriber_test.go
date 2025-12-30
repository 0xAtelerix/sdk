package gosdk

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/stretchr/testify/require"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/library/tests"
	"github.com/0xAtelerix/sdk/gosdk/library/tokens"
)

func mkEth(b byte) library.EthereumAddress {
	var a library.EthereumAddress

	a[0] = b

	return a
}

func mkSol(b byte) library.SolanaAddress {
	var a library.SolanaAddress

	a[0] = b

	return a
}

func Test_cmpAddr_Ethereum(t *testing.T) {
	t.Parallel()

	a := mkEth(1)
	b := mkEth(2)
	c := mkEth(1)

	require.Equal(t, -1, library.CmpAddr[library.EthereumAddress](a, b))
	require.Equal(t, 1, library.CmpAddr[library.EthereumAddress](b, a))
	require.Equal(t, 0, library.CmpAddr[library.EthereumAddress](a, c))
}

func Test_cmpAddr_Solana(t *testing.T) {
	t.Parallel()

	a := mkSol(1)
	b := mkSol(2)
	c := mkSol(1)

	require.Equal(t, -1, library.CmpAddr[library.SolanaAddress](a, b))
	require.Equal(t, 1, library.CmpAddr[library.SolanaAddress](b, a))
	require.Equal(t, 0, library.CmpAddr[library.SolanaAddress](a, c))
}

func newSubscriber() *Subscriber {
	return &Subscriber{
		ethContracts: make(
			map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
		),
		solAddresses: make(map[apptypes.ChainType]map[library.SolanaAddress]struct{}),
		deletedEthContracts: make(
			map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
		),
		deletedSolAddresses: make(map[apptypes.ChainType]map[library.SolanaAddress]struct{}),

		EVMEventRegistry:    tokens.NewRegistry[tokens.AppEvent](),
		evmHandlers:         make(map[string]AppEventHandler),
		solanaEventRegistry: tokens.NewRegistry[tokens.AppEvent](),
	}
}

func Test_sortChainAddresses_Ethereum(t *testing.T) {
	t.Parallel()

	items := []library.ChainAddresses[library.EthereumAddress]{
		{ChainID: 5, Addresses: []library.EthereumAddress{mkEth(3), mkEth(1), mkEth(2)}},
		{ChainID: 1, Addresses: []library.EthereumAddress{mkEth(2), mkEth(1)}},
		{ChainID: 3, Addresses: []library.EthereumAddress{mkEth(9)}},
	}
	library.SortChainAddresses(items)

	require.Equal(t, apptypes.ChainType(1), items[0].ChainID)
	require.Equal(t, []library.EthereumAddress{mkEth(1), mkEth(2)}, items[0].Addresses)

	require.Equal(t, apptypes.ChainType(3), items[1].ChainID)
	require.Equal(t, []library.EthereumAddress{mkEth(9)}, items[1].Addresses)

	require.Equal(t, apptypes.ChainType(5), items[2].ChainID)
	require.Equal(t, []library.EthereumAddress{mkEth(1), mkEth(2), mkEth(3)}, items[2].Addresses)
}

func Test_sortChainAddresses_Solana(t *testing.T) {
	t.Parallel()

	items := []library.ChainAddresses[library.SolanaAddress]{
		{ChainID: 42, Addresses: []library.SolanaAddress{mkSol(7), mkSol(3), mkSol(3), mkSol(4)}},
		{ChainID: 9, Addresses: []library.SolanaAddress{mkSol(2), mkSol(1)}},
	}
	library.SortChainAddresses(items)

	require.Equal(t, apptypes.ChainType(9), items[0].ChainID)
	require.Equal(t, []library.SolanaAddress{mkSol(1), mkSol(2)}, items[0].Addresses)

	require.Equal(t, apptypes.ChainType(42), items[1].ChainID)
	require.Equal(
		t,
		[]library.SolanaAddress{mkSol(3), mkSol(3), mkSol(4), mkSol(7)},
		items[1].Addresses,
	)
}

func Test_collectChainAddresses_SetsToSlice_Ethereum(t *testing.T) {
	t.Parallel()

	m := map[apptypes.ChainType]map[library.EthereumAddress]struct{}{
		10: {
			mkEth(3): {},
			mkEth(1): {},
		},
		1: {
			mkEth(2): {},
		},
	}
	out := library.CollectChainAddresses[library.EthereumAddress](m)

	require.Len(t, out, 2)
	require.Equal(t, apptypes.ChainType(1), out[0].ChainID)
	require.Equal(t, []library.EthereumAddress{mkEth(2)}, out[0].Addresses)

	require.Equal(t, apptypes.ChainType(10), out[1].ChainID)
	// order within a chain must be sorted
	require.Equal(t, []library.EthereumAddress{mkEth(1), mkEth(3)}, out[1].Addresses)
}

func Test_collectChainAddresses_SetsToSlice_Solana(t *testing.T) {
	t.Parallel()

	m := map[apptypes.ChainType]map[library.SolanaAddress]struct{}{
		5: {
			mkSol(9): {},
			mkSol(1): {},
			mkSol(7): {},
		},
	}
	out := library.CollectChainAddresses[library.SolanaAddress](m)

	require.Len(t, out, 1)
	require.Equal(t, apptypes.ChainType(5), out[0].ChainID)
	require.Equal(t, []library.SolanaAddress{mkSol(1), mkSol(7), mkSol(9)}, out[0].Addresses)
}

func Test_bytesOf_Ethereum(t *testing.T) {
	t.Parallel()

	a := mkEth(0xAB)
	b := bytesOf[library.EthereumAddress](a)

	require.Len(t, b, library.EthereumAddressLength)
	require.Equal(t, byte(0xAB), b[0])
	// ensure it's a copy (mutating b won't affect a)
	b[0] = 0

	require.Equal(t, byte(0xAB), a[0])
}

func Test_bytesOf_Solana(t *testing.T) {
	t.Parallel()

	a := mkSol(0xCD)
	b := bytesOf[library.SolanaAddress](a)

	require.Len(t, b, library.SolanaAddressLength)
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

	s.SubscribeEthContract(chainID, contract, nil)
	require.True(t, s.IsEthSubscription(chainID, contract))

	// subscribing twice keeps it present (idempotent)
	s.SubscribeEthContract(chainID, contract, nil)
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
	s.SubscribeEthContract(chainID, contract, nil)
	require.True(t, s.IsEthSubscription(chainID, contract))

	// act: unsubscribe (should NOT panic with correct code)
	s.UnsubscribeEthContract(chainID, contract, nil)

	// removed from active - no subscriptions, listen to all chains
	require.True(t, s.IsEthSubscription(chainID, contract))

	// and marked as deleted
	require.NotNil(t, s.deletedEthContracts[chainID])

	_, ok := s.deletedEthContracts[chainID][contract]
	require.True(t, ok)

	// re-subscribe clears the deleted marker (idempotent behavior)
	s.SubscribeEthContract(chainID, contract, nil)
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
	s.solAddresses[chainID] = map[library.SolanaAddress]struct{}{addr: {}}

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
	s.SubscribeSolanaAddress(library.SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(library.SolanaChainID, addr))

	// delete (should NOT panic; should remove from active and mark as deleted), no subscriptions
	s.UnsubscribeSolanaAddress(library.SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(library.SolanaChainID, addr))

	// deleted map must be initialized and contain the addr
	require.NotNil(t, s.deletedSolAddresses[library.SolanaChainID])
	_, ok := s.deletedSolAddresses[library.SolanaChainID][addr]
	require.True(t, ok)

	// re-add clears deleted marker
	s.SubscribeSolanaAddress(library.SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(library.SolanaChainID, addr))
	_, ok = s.deletedSolAddresses[library.SolanaChainID][addr]
	require.False(t, ok)

	// delete again re-marks as deleted, no subscriptions
	s.UnsubscribeSolanaAddress(library.SolanaChainID, addr)
	require.True(t, s.IsSolanaSubscription(library.SolanaChainID, addr))

	_, ok = s.deletedSolAddresses[library.SolanaChainID][addr]
	require.True(t, ok)
}

func readAllSubscriptions(
	t *testing.T,
	tx kv.Tx,
) (map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{}, map[apptypes.ChainType]map[library.SolanaAddress]struct{}) {
	t.Helper()

	gotEth := make(
		map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
	)
	gotSol := make(map[apptypes.ChainType]map[library.SolanaAddress]struct{})

	err := tx.ForEach(SubscriptionBucket, nil, func(k, v []byte) error {
		chainID := binary.BigEndian.Uint64(k)

		const topicKeyMask uint64 = 1 << 63

		isTopicKey := (chainID & topicKeyMask) != 0
		chain := apptypes.ChainType(chainID &^ topicKeyMask)

		if library.IsEvmChain(chain) {
			if gotEth[chain] == nil {
				gotEth[chain] = make(map[library.EthereumAddress]map[library.EthereumTopic]struct{})
			}

			if isTopicKey {
				var entries []library.ChainTopicAddresses

				dbErr := cbor.Unmarshal(v, &entries)
				require.NoError(t, dbErr)

				for _, entry := range entries {
					if gotEth[chain][entry.Address] == nil {
						gotEth[chain][entry.Address] = make(map[library.EthereumTopic]struct{})
					}

					for _, topic := range entry.Topics {
						gotEth[chain][entry.Address][topic] = struct{}{}
					}
				}
			} else {
				var addrs []library.EthereumAddress

				dbErr := cbor.Unmarshal(v, &addrs)
				require.NoError(t, dbErr)

				for _, addr := range addrs {
					gotEth[chain][addr] = nil // wildcard topics
				}
			}
		} else if library.IsSolanaChain(chain) {
			gotSol[chain] = make(map[library.SolanaAddress]struct{})

			var addrs []library.SolanaAddress

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

	db, closeFn := tests.TestDB(t, SubscriptionEventLibraryBucket, SubscriptionBucket)
	defer closeFn()

	s := newSubscriber()
	// chain 1: two ETH
	eth1 := mkEth(0x01)
	eth2 := mkEth(0x02)

	s.SubscribeEthContract(1, eth1, nil)
	s.SubscribeEthContract(1, eth2, nil)

	// chain 2: one SOL
	solA := mkSol(0xAA)
	s.SubscribeSolanaAddress(library.SolanaChainID, solA)

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

	// chain 1 should have eth1, eth2 (wildcard topics)
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
	require.Contains(t, gotSol, library.SolanaChainID)
	require.Len(t, gotSol[library.SolanaChainID], 1)

	require.Equal(t, map[library.SolanaAddress]struct{}{solA: {}}, gotSol[library.SolanaChainID])
}

func Test_Store_Persistency_DeleteThenUpsert(t *testing.T) {
	t.Parallel()

	db, closeFn := tests.TestDB(t, SubscriptionEventLibraryBucket, SubscriptionBucket)
	defer closeFn()

	s := newSubscriber()

	// Start with two SOL on chain Sol
	solA := mkSol(0x10)
	solB := mkSol(0x20)

	s.SubscribeSolanaAddress(library.SolanaChainID, solA, solB)

	// Persist initial state
	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	// Now delete solA, keep solB
	s.UnsubscribeSolanaAddress(library.SolanaChainID, solA)

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

	require.Equal(t, map[library.SolanaAddress]struct{}{solB: {}}, gotSol[library.SolanaChainID])
}

func Test_Store_Persistency_DeleteWholeChainThenReAdd(t *testing.T) {
	t.Parallel()

	db, closeFn := tests.TestDB(t, SubscriptionEventLibraryBucket, SubscriptionBucket)
	defer closeFn()

	s := newSubscriber()

	// Chain 4: one ETH
	ethX := mkEth(0x44)
	s.SubscribeEthContract(library.BNBChainID, ethX, nil)

	// Persist
	rw, err := db.BeginRw(t.Context())
	require.NoError(t, err)
	require.NoError(t, s.Store(rw))
	require.NoError(t, rw.Commit())

	// Remove last ETH from chain BNBChainID -> chain should end up empty
	s.UnsubscribeEthContract(library.BNBChainID, ethX, nil)

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
	s.SubscribeEthContract(library.BNBChainID, ethY, nil)

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

	require.Equal(t, map[library.EthereumAddress]map[library.EthereumTopic]struct{}{
		ethY: {},
	},
		gotEth[library.BNBChainID],
	)
}

const wethDepositEventName = "Deposit"

type wethDeposit struct {
	Dst library.EthereumAddress `abi:"dst"`
	Wad *big.Int                `abi:"wad"`
}

func Test_AddEVMEvent_RegistersAndInvokesHandler_ForSubscribedEmitter(t *testing.T) {
	t.Parallel()

	// Arrange subscriber
	s := newSubscriber()

	chainID := apptypes.ChainType(1)
	contract := mkEth(0xFE) // subscription key (converted to go-eth Address below)

	// Build ABI: event Deposit(address indexed dst, uint256 wad)
	a, err := abi.JSON(strings.NewReader(`[
	  {"type":"event","name":"Deposit","inputs":[
	    {"indexed":true,"name":"dst","type":"address"},
	    {"indexed":false,"name":"wad","type":"uint256"}
	  ]}
	]`))
	require.NoError(t, err)

	var (
		got    tokens.Event[wethDeposit]
		called int
	)

	db, closeFn := tests.TestDB(t, SubscriptionEventLibraryBucket, SubscriptionBucket)
	defer closeFn()

	// Register + attach handler via AddEVMEvent
	_, err = AddEVMEvent(t.Context(),
		s,
		library.NewEVMEvent(chainID, contract, wethDepositEventName, a),
		func(ev tokens.Event[wethDeposit], _ kv.RwTx) {
			called++
			got = ev
		},
		db,
	)
	require.NoError(t, err)

	// Build a matching log emitted by *the subscribed contract*.
	wethAddr := common.BytesToAddress(bytesOf(contract))
	dst := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	sig := crypto.Keccak256Hash([]byte("Deposit(address,uint256)"))

	data := tests.MustPack(t, abi.Arguments{{Type: tests.MustType(t, "uint256")}}, big.NewInt(777))
	lg := &gethtypes.Log{
		Address: wethAddr, // <-- authoritative emitter!
		Topics:  []common.Hash{sig, tests.AddrTopic(dst)},
		Data:    data,
		Index:   5,
	}
	txHash := common.HexToHash("0x123")

	// Decode via registry
	evs, matched, err := s.EVMEventRegistry.HandleLog(lg, txHash)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, evs, 1)

	// Gate by subscription using log.Address (NOT tx.To) — as batch loop does.
	ok := s.IsEthSubscription(chainID, library.EthereumAddress(wethAddr))
	require.True(t, ok, "emitter should be subscribed")

	tx, err := db.BeginRw(t.Context())
	require.NoError(t, err)

	defer tx.Rollback()

	// Dispatch to the kind's handler
	h, exists := s.evmHandlers[wethDepositEventName]
	require.True(t, exists)
	h.Handle(evs, tx)

	// Assert handler received typed event
	require.Equal(t, 1, called)
	require.Equal(t, wethDepositEventName, got.EventName)
	require.Equal(t, wethAddr.Hex(), got.Contract)
	require.Equal(t, library.EthereumAddress(dst), got.SubscribedEvent.Dst)
	require.Zero(t, got.SubscribedEvent.Wad.Cmp(big.NewInt(777)))
	require.Equal(t, uint(5), got.LogIndex)
	require.Equal(t, txHash.Hex(), got.TxHash)
}

func Test_AddEVMEvent_NotInvoked_WhenEmitterNotSubscribed(t *testing.T) {
	t.Parallel()

	s := newSubscriber()

	chainID := apptypes.ChainType(1)
	contract := mkEth(0xAA) // we subscribe this one

	// ABI
	a, err := abi.JSON(strings.NewReader(`[
	  {"type":"event","name":"Deposit","inputs":[
	    {"indexed":true,"name":"dst","type":"address"},
	    {"indexed":false,"name":"wad","type":"uint256"}
	  ]}
	]`))
	require.NoError(t, err)

	db, closeFn := tests.TestDB(t, SubscriptionEventLibraryBucket, SubscriptionBucket)
	defer closeFn()

	var called int

	_, err = AddEVMEvent[wethDeposit](t.Context(),
		s,
		library.NewEVMEvent(chainID, contract, "Deposit", a),
		func(tokens.Event[wethDeposit], kv.RwTx) {
			called++
		},
		db,
	)
	require.NoError(t, err)

	// Log comes from a DIFFERENT contract (not subscribed)
	other := common.HexToAddress("0x0000000000000000000000000000000000000BAD")
	sig := crypto.Keccak256Hash([]byte("Deposit(address,uint256)"))
	dst := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	data := tests.MustPack(t, abi.Arguments{{Type: tests.MustType(t, "uint256")}}, big.NewInt(1))
	lg := &gethtypes.Log{
		Address: other, // <- not subscribed
		Topics:  []common.Hash{sig, tests.AddrTopic(dst)},
		Data:    data,
		Index:   0,
	}
	txHash := common.HexToHash("0x9")

	evs, matched, err := s.EVMEventRegistry.HandleLog(lg, txHash)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, evs, 1)

	// Subscription gate should block dispatch
	ok := s.IsEthSubscription(chainID, library.EthereumAddress(other))
	require.False(t, ok)

	tx, err := db.BeginRw(t.Context())
	require.NoError(t, err)

	defer tx.Rollback()

	// (Simulate batch loop: since not subscribed, don't dispatch)
	if ok {
		s.evmHandlers[wethDepositEventName].Handle(evs, tx)
	}

	require.Equal(t, 0, called)
}

func Test_AddEVMEvent_MultipleKinds_DispatchSeparately(t *testing.T) {
	t.Parallel()

	s := newSubscriber()
	chainID := apptypes.ChainType(1)

	contractA := mkEth(0xA1)
	contractB := mkEth(0xB2)

	// Two ABIs with different events
	abiA, err := abi.JSON(strings.NewReader(`[
	  {"type":"event","name":"Deposit","inputs":[
	    {"indexed":true,"name":"dst","type":"address"},
	    {"indexed":false,"name":"wad","type":"uint256"}
	  ]}
	]`))
	require.NoError(t, err)

	type erc20Approval struct {
		Owner   common.Address `abi:"owner"`
		Spender common.Address `abi:"spender"`
		Value   *big.Int       `abi:"value"`
	}

	abiB, err := abi.JSON(strings.NewReader(`[
	  {"type":"event","name":"Approval","inputs":[
	    {"indexed":true,"name":"owner","type":"address"},
	    {"indexed":true,"name":"spender","type":"address"},
	    {"indexed":false,"name":"value","type":"uint256"}
	  ]}
	]`))
	require.NoError(t, err)

	db, closeFn := tests.TestDB(t, SubscriptionEventLibraryBucket, SubscriptionBucket)
	defer closeFn()

	// Register + attach both
	var depCalls, apprCalls int

	_, err = AddEVMEvent[wethDeposit](
		t.Context(),
		s,
		library.NewEVMEvent(chainID, contractA, wethDepositEventName, abiA),
		func(tokens.Event[wethDeposit], kv.RwTx) {
			depCalls++
		},
		db,
	)
	require.NoError(t, err)

	_, err = tokens.RegisterEvent[erc20Approval](
		s.EVMEventRegistry,
		abiB,
		"Approval",
	)
	require.NoError(t, err)

	// Attach handler manually (simulating AddEVMEvent-like attach)
	s.SubscribeEthContract(chainID, contractB, NewEVMHandler("Approval",
		func(tokens.Event[erc20Approval], kv.RwTx) {
			apprCalls++
		},
	))

	// Build one log per contract/kind
	// A: Deposit
	dst := common.HexToAddress("0x00000000000000000000000000000000000000A5")
	lgA := &gethtypes.Log{
		Address: common.BytesToAddress(bytesOf(contractA)),
		Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("Deposit(address,uint256)")),
			tests.AddrTopic(dst),
		},
		Data: tests.MustPack(
			t,
			abi.Arguments{{Type: tests.MustType(t, "uint256")}},
			big.NewInt(10),
		),
		Index: 1,
	}

	// B: Approval
	owner := common.HexToAddress("0x0000000000000000000000000000000000000C01")
	spender := common.HexToAddress("0x0000000000000000000000000000000000000C02")
	lgB := &gethtypes.Log{
		Address: common.BytesToAddress(bytesOf(contractB)),
		Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("Approval(address,address,uint256)")),
			tests.AddrTopic(owner),
			tests.AddrTopic(spender),
		},
		Data: tests.MustPack(
			t,
			abi.Arguments{{Type: tests.MustType(t, "uint256")}},
			big.NewInt(99),
		),
		Index: 2,
	}

	tx, err := db.BeginRw(t.Context())
	require.NoError(t, err)

	defer tx.Rollback()

	// Decode and dispatch both
	for _, lg := range []*gethtypes.Log{lgA, lgB} {
		evs, matched, err := s.EVMEventRegistry.HandleLog(lg, common.HexToHash("0xdeadbeef"))
		require.NoError(t, err)
		require.True(t, matched)

		emitter := library.EthereumAddress(lg.Address)
		if s.IsEthSubscription(chainID, emitter) {
			// group-by-kind (as your batch loop would)
			byKind := map[string][]tokens.AppEvent{}
			for _, e := range evs {
				byKind[e.Name()] = append(byKind[e.Name()], e)
			}

			for k, list := range byKind {
				if h, ok := s.evmHandlers[k]; ok {
					h.Handle(list, tx)
				}
			}
		}
	}

	require.Equal(t, 1, depCalls, "deposit handler should run once")
	require.Equal(t, 1, apprCalls, "approval handler should run once")
}

func Test_EVMEvent_PersistAndRestore(t *testing.T) {
	t.Parallel()

	db, closeFn := tests.TestDB(t, SubscriptionEventLibraryBucket, SubscriptionBucket)
	defer closeFn()

	ctx := t.Context()

	// Fresh subscriber (empty registry/handlers)
	s, err := NewSubscriber(ctx, db)
	require.NoError(t, err)

	// Build a simple WETH Deposit event ABI
	wethABI, err := abi.JSON(strings.NewReader(`[
	  {"type":"event","name":"Deposit","inputs":[
	    {"indexed":true,"name":"dst","type":"address"},
	    {"indexed":false,"name":"wad","type":"uint256"}
	  ]}
	]`))
	require.NoError(t, err)

	// Spec to persist
	chainID := apptypes.ChainType(1)
	contract := mkEth(0xEE)
	evName := "Deposit"

	spec := &library.EVMEvent{
		ChainID:   chainID,
		EventName: evName,
		Contract:  contract,
		ABI:       wethABI,
	}

	// handler to attach
	var handlerCalls int64

	handler := func(ev tokens.Event[struct {
		Dst common.Address `abi:"dst"`
		Wad *big.Int       `abi:"wad"`
	}], _ kv.RwTx,
	) {
		atomic.AddInt64(&handlerCalls, ev.SubscribedEvent.Wad.Int64())
	}

	// 1) AddEVMEvent should register in-memory + persist to DB
	_, err = AddEVMEvent(ctx, s, spec, handler, db)
	require.NoError(t, err)

	// Verify it’s persisted
	ro, err := db.BeginRo(ctx)
	require.NoError(t, err)

	defer ro.Rollback()

	key := EVMEventKey(chainID, contract, evName)

	raw, err := ro.GetOne(SubscriptionEventLibraryBucket, key)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	var got library.EVMEvent
	require.NoError(t, cbor.Unmarshal(raw, &got))

	require.Equal(t, spec.ChainID, got.ChainID)
	require.Equal(t, spec.EventName, got.EventName)
	require.Equal(t, spec.Contract, got.Contract)

	// ABI round-trip: we can spot-check the event exists and the signature matches
	ev, ok := got.ABI.Events[evName]
	require.True(t, ok, "event must be present in restored ABI")
	require.Equal(t, wethABI.Events[evName].ID, ev.ID, "topic0 must match")

	// 2) Simulate restart: new subscriber (empty in-memory state)
	s2, err := NewSubscriber(ctx, db)
	require.NoError(t, err)
	require.Empty(t, s2.evmHandlers)

	// 3) Restore the event + attach (compile-time typed) handler
	require.NoError(t, LoadEVMEvent(ctx, s2, chainID, contract, evName, db, handler))

	// Ensure handler registered under the event name key (per addEVMEvent)
	h, ok := s2.evmHandlers[evName]
	require.True(t, ok, "handler should be restored into evmHandlers")
	require.Equal(t, evName, h.Name())

	// 4) Sanity: the registry should now know how to decode a matching log
	// Build synthetic Deposit log
	dst := common.Address{0: 0xAA}
	wad := big.NewInt(777)
	sig := crypto.Keccak256Hash([]byte("Deposit(address,uint256)"))

	data := tests.MustPack(t, abi.Arguments{{Type: tests.MustType(t, "uint256")}}, wad)
	lg := &gethtypes.Log{
		Address: common.Address(contract),
		Topics:  []common.Hash{sig, tests.AddrTopic(dst)},
		Data:    data,
		Index:   0,
	}
	txHash := common.HexToHash("0xabc")

	events, matched, derr := s2.EVMEventRegistry.HandleLog(lg, txHash)
	require.NoError(t, derr)
	require.True(t, matched)
	require.NotEmpty(t, events)

	rwTx, err := db.BeginRw(t.Context())
	require.NoError(t, err)

	defer rwTx.Rollback()

	h.Handle(events, rwTx)
	require.Equal(t, int64(777), handlerCalls)
}
