package subscriber

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/0xAtelerix/sdk/gosdk/library/tokens"
	"github.com/0xAtelerix/sdk/gosdk/scheme"
)

type Subscriber struct {
	EthContracts map[apptypes.ChainType]map[library.EthereumAddress]struct{} // chainID -> EthereumContractAddress
	SolAddresses map[apptypes.ChainType]map[library.SolanaAddress]struct{}   // chainID -> SolanaAddress

	EVMEventRegistry *tokens.Registry[tokens.AppEvent]
	EVMHandlers      map[string]AppEventHandler // eventName -> Handler

	SolanaEventRegistry *tokens.Registry[tokens.AppEvent]

	deletedEthContracts map[apptypes.ChainType]map[library.EthereumAddress]struct{} // chainID -> EthereumContractAddress
	deletedSolAddresses map[apptypes.ChainType]map[library.SolanaAddress]struct{}   // chainID -> SolanaAddress

	mu sync.RWMutex
}

func NewSubscriber(ctx context.Context, tx kv.RoDB) (*Subscriber, error) {
	roTx, err := tx.BeginRo(ctx)
	if err != nil {
		return nil, err
	}

	defer roTx.Rollback()

	sub := &Subscriber{
		EthContracts:        make(map[apptypes.ChainType]map[library.EthereumAddress]struct{}),
		SolAddresses:        make(map[apptypes.ChainType]map[library.SolanaAddress]struct{}),
		EVMEventRegistry:    tokens.NewRegistry[tokens.AppEvent](),
		EVMHandlers:         make(map[string]AppEventHandler),
		SolanaEventRegistry: tokens.NewRegistry[tokens.AppEvent](),
		deletedEthContracts: make(map[apptypes.ChainType]map[library.EthereumAddress]struct{}),
		deletedSolAddresses: make(map[apptypes.ChainType]map[library.SolanaAddress]struct{}),
	}

	sub.EthContracts, sub.SolAddresses, err = loadAllSubscriptions(roTx)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func (s *Subscriber) SubscribeEthContract(
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	h ...AppEventHandler,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.EthContracts[chainID]; !ok {
		s.EthContracts[chainID] = make(map[library.EthereumAddress]struct{})
	}

	delete(s.deletedEthContracts[chainID], contract)
	s.EthContracts[chainID][contract] = struct{}{}

	if len(h) > 0 {
		if s.EVMHandlers == nil {
			s.EVMHandlers = make(map[string]AppEventHandler)
		}

		if h[0] == nil {
			return
		}

		s.EVMHandlers[h[0].Name()] = h[0]
	}
}

func (s *Subscriber) UnsubscribeEthContract(
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	h ...AppEventHandler,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.EthContracts[chainID]; !ok {
		return
	}

	if _, ok := s.deletedEthContracts[chainID]; !ok {
		s.deletedEthContracts[chainID] = make(map[library.EthereumAddress]struct{})
	}

	delete(s.EthContracts[chainID], contract)

	s.deletedEthContracts[chainID][contract] = struct{}{}

	if len(h) > 0 {
		if s.EVMHandlers == nil || h[0] == nil {
			return
		}

		delete(s.EVMHandlers, h[0].Name())
	}
}

func (s *Subscriber) IsEthSubscription(
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// if there is no subscriptions, give all blocks
	if len(s.EthContracts[chainID]) == 0 {
		return true
	}

	if _, ok := s.EthContracts[chainID]; !ok {
		return false
	}

	_, ok := s.EthContracts[chainID][contract]

	return ok
}

func (s *Subscriber) SubscribeSolanaAddress(
	chainID apptypes.ChainType,
	addresses ...library.SolanaAddress,
) {
	if len(addresses) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.SolAddresses[chainID]; !ok {
		s.SolAddresses[chainID] = make(map[library.SolanaAddress]struct{})
	}

	for _, address := range addresses {
		delete(s.deletedSolAddresses[chainID], address)
		s.SolAddresses[chainID][address] = struct{}{}
	}
}

func (s *Subscriber) IsSolanaSubscription(
	chainID apptypes.ChainType,
	address library.SolanaAddress,
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// if there is no subscriptions, give all blocks
	if len(s.SolAddresses[chainID]) == 0 {
		return true
	}

	if _, ok := s.SolAddresses[chainID]; !ok {
		return false
	}

	if _, ok := s.SolAddresses[chainID][address]; !ok {
		return false
	}

	return true
}

func (s *Subscriber) UnsubscribeSolanaAddress(
	chainID apptypes.ChainType,
	addresses ...library.SolanaAddress,
) {
	if len(addresses) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.SolAddresses[chainID]; !ok {
		return
	}

	if _, ok := s.deletedSolAddresses[chainID]; !ok {
		s.deletedSolAddresses[chainID] = make(map[library.SolanaAddress]struct{})
	}

	for _, address := range addresses {
		delete(s.SolAddresses[chainID], address)

		s.deletedSolAddresses[chainID][address] = struct{}{}
	}
}

func (s *Subscriber) Store(tx kv.RwTx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	evmSet, solSet, err := loadAllSubscriptions(tx)
	if err != nil {
		return err
	}

	// Ensure inner maps exist for chains referenced by pending ops
	for chainID := range s.EthContracts {
		if evmSet[chainID] == nil {
			evmSet[chainID] = make(map[library.EthereumAddress]struct{})
		}
	}

	for chainID := range s.deletedEthContracts {
		if evmSet[chainID] == nil {
			evmSet[chainID] = make(map[library.EthereumAddress]struct{})
		}
	}

	for chainID := range s.SolAddresses {
		if solSet[chainID] == nil {
			solSet[chainID] = make(map[library.SolanaAddress]struct{})
		}
	}

	for chainID := range s.deletedSolAddresses {
		if solSet[chainID] == nil {
			solSet[chainID] = make(map[library.SolanaAddress]struct{})
		}
	}

	// 2) Apply deletes (remove from sets)
	for chainID, delSet := range s.deletedEthContracts {
		for addr := range delSet {
			delete(evmSet[chainID], addr)
		}
	}

	for chainID, delSet := range s.deletedSolAddresses {
		for addr := range delSet {
			delete(solSet[chainID], addr)
		}
	}

	// 3) Apply appends (merge additions)
	for chainID, addSet := range s.EthContracts {
		for addr := range addSet {
			evmSet[chainID][addr] = struct{}{}
		}
	}

	for chainID, addSet := range s.SolAddresses {
		for addr := range addSet {
			solSet[chainID][addr] = struct{}{}
		}
	}

	// 4) Sort (deterministic ordering)
	evmItems := library.CollectChainAddresses(evmSet) // []chainAddresses[EthereumAddress]
	solItems := library.CollectChainAddresses(solSet) // []chainAddresses[SolanaAddress]

	// 5) Write back (CBOR per chain)
	// EVM
	for _, ca := range evmItems {
		if err = putChainAddresses(tx, ca.ChainID, ca.Addresses); err != nil {
			return err
		}
	}

	// SOL
	for _, ca := range solItems {
		if err = putChainAddresses(tx, ca.ChainID, ca.Addresses); err != nil {
			return err
		}
	}

	// Clear the deleted maps after successful persist
	s.deletedEthContracts = make(map[apptypes.ChainType]map[library.EthereumAddress]struct{})
	s.deletedSolAddresses = make(map[apptypes.ChainType]map[library.SolanaAddress]struct{})

	return nil
}

// loadAllSubscriptions reads the whole subscription bucket and returns
// two in-memory sets: EVM and Solana, keyed by chainID then address.
func loadAllSubscriptions(tx kv.Tx) (
	map[apptypes.ChainType]map[library.EthereumAddress]struct{},
	map[apptypes.ChainType]map[library.SolanaAddress]struct{},
	error,
) {
	evm := make(map[apptypes.ChainType]map[library.EthereumAddress]struct{})
	sol := make(map[apptypes.ChainType]map[library.SolanaAddress]struct{})

	err := tx.ForEach(scheme.SubscriptionBucket, nil, func(chainIDBytes, addrBytes []byte) error {
		chainID := apptypes.ChainType(binary.BigEndian.Uint64(chainIDBytes))

		if library.IsEvmChain(chainID) {
			var addrSlice []library.EthereumAddress
			if err := cbor.Unmarshal(addrBytes, &addrSlice); err != nil {
				return err
			}

			if evm[chainID] == nil {
				evm[chainID] = make(map[library.EthereumAddress]struct{})
			}

			for _, a := range addrSlice {
				evm[chainID][a] = struct{}{}
			}

			return nil
		}

		if library.IsSolanaChain(chainID) {
			var addrSlice []library.SolanaAddress
			if err := cbor.Unmarshal(addrBytes, &addrSlice); err != nil {
				return err
			}

			if sol[chainID] == nil {
				sol[chainID] = make(map[library.SolanaAddress]struct{})
			}

			for _, a := range addrSlice {
				sol[chainID][a] = struct{}{}
			}

			return nil
		}

		return fmt.Errorf("%w: unknown chain type %d", library.ErrUnknownChain, chainID)
	})
	if err != nil {
		return nil, nil, err
	}

	return evm, sol, nil
}

func putChainAddresses[T library.Address](tx kv.RwTx, chainID apptypes.ChainType, addrs []T) error {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], uint64(chainID))

	if len(addrs) == 0 {
		return tx.Delete(scheme.SubscriptionBucket, key[:])
	}

	data, err := cbor.Marshal(addrs)
	if err != nil {
		return err
	}

	return tx.Put(scheme.SubscriptionBucket, key[:], data)
}

func bytesOf[T library.Address](a T) []byte {
	b := make([]byte, len(a))

	for i := range len(a) {
		b[i] = a[i]
	}

	return b
}
