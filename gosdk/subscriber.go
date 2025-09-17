package gosdk

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

type Subscriber struct {
	ethContracts map[apptypes.ChainType]map[EthereumAddress]struct{} // chainID -> EthereumContractAddress
	solAddresses map[apptypes.ChainType]map[SolanaAddress]struct{}   // chainID -> SolanaAddress

	deletedEthContracts map[apptypes.ChainType]map[EthereumAddress]struct{} // chainID -> EthereumContractAddress
	deletedSolAddresses map[apptypes.ChainType]map[SolanaAddress]struct{}   // chainID -> SolanaAddress

	mu sync.RWMutex
}

func NewSubscriber(ctx context.Context, tx kv.RoDB) (*Subscriber, error) {
	roTx, err := tx.BeginRo(ctx)
	if err != nil {
		return nil, err
	}

	defer roTx.Rollback()

	sub := &Subscriber{
		ethContracts:        make(map[apptypes.ChainType]map[EthereumAddress]struct{}),
		solAddresses:        make(map[apptypes.ChainType]map[SolanaAddress]struct{}),
		deletedEthContracts: make(map[apptypes.ChainType]map[EthereumAddress]struct{}),
		deletedSolAddresses: make(map[apptypes.ChainType]map[SolanaAddress]struct{}),
	}

	sub.ethContracts, sub.solAddresses, err = loadAllSubscriptions(roTx)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func (s *Subscriber) SubscribeEthContract(
	chainID apptypes.ChainType,
	contracts ...EthereumAddress,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		s.ethContracts[chainID] = make(map[EthereumAddress]struct{})
	}

	for _, contract := range contracts {
		delete(s.deletedEthContracts[chainID], contract)
		s.ethContracts[chainID][contract] = struct{}{}
	}
}

func (s *Subscriber) UnsubscribeEthContract(
	chainID apptypes.ChainType,
	contracts ...EthereumAddress,
) {
	if len(contracts) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		return
	}

	if _, ok := s.deletedEthContracts[chainID]; !ok {
		s.deletedEthContracts[chainID] = make(map[EthereumAddress]struct{})
	}

	for _, contract := range contracts {
		delete(s.ethContracts[chainID], contract)

		s.deletedEthContracts[chainID][contract] = struct{}{}
	}
}

func (s *Subscriber) IsEthSubscription(chainID apptypes.ChainType, contract EthereumAddress) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// if there is no subscriptions, give all blocks
	if len(s.ethContracts[chainID]) == 0 {
		return true
	}

	if _, ok := s.ethContracts[chainID]; !ok {
		return false
	}

	_, ok := s.ethContracts[chainID][contract]

	return ok
}

func (s *Subscriber) SubscribeSolanaAddress(
	chainID apptypes.ChainType,
	addresses ...SolanaAddress,
) {
	if len(addresses) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.solAddresses[chainID]; !ok {
		s.solAddresses[chainID] = make(map[SolanaAddress]struct{})
	}

	for _, address := range addresses {
		delete(s.deletedSolAddresses[chainID], address)
		s.solAddresses[chainID][address] = struct{}{}
	}
}

func (s *Subscriber) IsSolanaSubscription(chainID apptypes.ChainType, address SolanaAddress) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// if there is no subscriptions, give all blocks
	if len(s.solAddresses[chainID]) == 0 {
		return true
	}

	if _, ok := s.solAddresses[chainID]; !ok {
		return false
	}

	if _, ok := s.solAddresses[chainID][address]; !ok {
		return false
	}

	return true
}

func (s *Subscriber) UnsubscribeSolanaAddress(
	chainID apptypes.ChainType,
	addresses ...SolanaAddress,
) {
	if len(addresses) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.solAddresses[chainID]; !ok {
		return
	}

	if _, ok := s.deletedSolAddresses[chainID]; !ok {
		s.deletedSolAddresses[chainID] = make(map[SolanaAddress]struct{})
	}

	for _, address := range addresses {
		delete(s.solAddresses[chainID], address)

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
	for chainID := range s.ethContracts {
		if evmSet[chainID] == nil {
			evmSet[chainID] = make(map[EthereumAddress]struct{})
		}
	}

	for chainID := range s.deletedEthContracts {
		if evmSet[chainID] == nil {
			evmSet[chainID] = make(map[EthereumAddress]struct{})
		}
	}

	for chainID := range s.solAddresses {
		if solSet[chainID] == nil {
			solSet[chainID] = make(map[SolanaAddress]struct{})
		}
	}

	for chainID := range s.deletedSolAddresses {
		if solSet[chainID] == nil {
			solSet[chainID] = make(map[SolanaAddress]struct{})
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
	for chainID, addSet := range s.ethContracts {
		for addr := range addSet {
			evmSet[chainID][addr] = struct{}{}
		}
	}

	for chainID, addSet := range s.solAddresses {
		for addr := range addSet {
			solSet[chainID][addr] = struct{}{}
		}
	}

	// 4) Sort (deterministic ordering)
	evmItems := CollectChainAddresses(evmSet) // []chainAddresses[EthereumAddress]
	solItems := CollectChainAddresses(solSet) // []chainAddresses[SolanaAddress]

	// 5) Write back (CBOR per chain)
	// EVM
	for _, ca := range evmItems {
		if err = putChainAddresses(tx, ca.chainID, ca.addresses); err != nil {
			return err
		}
	}

	// SOL
	for _, ca := range solItems {
		if err = putChainAddresses(tx, ca.chainID, ca.addresses); err != nil {
			return err
		}
	}

	// Clear the deleted maps after successful persist
	s.deletedEthContracts = make(map[apptypes.ChainType]map[EthereumAddress]struct{})
	s.deletedSolAddresses = make(map[apptypes.ChainType]map[SolanaAddress]struct{})

	return nil
}

// loadAllSubscriptions reads the whole subscription bucket and returns
// two in-memory sets: EVM and Solana, keyed by chainID then address.
func loadAllSubscriptions(tx kv.Tx) (
	map[apptypes.ChainType]map[EthereumAddress]struct{},
	map[apptypes.ChainType]map[SolanaAddress]struct{},
	error,
) {
	evm := make(map[apptypes.ChainType]map[EthereumAddress]struct{})
	sol := make(map[apptypes.ChainType]map[SolanaAddress]struct{})

	err := tx.ForEach(SubscriptionBucket, nil, func(chainIDBytes, addrBytes []byte) error {
		chainID := apptypes.ChainType(binary.BigEndian.Uint64(chainIDBytes))

		if IsEvmChain(chainID) {
			var addrSlice []EthereumAddress
			if err := cbor.Unmarshal(addrBytes, &addrSlice); err != nil {
				return err
			}

			if evm[chainID] == nil {
				evm[chainID] = make(map[EthereumAddress]struct{})
			}

			for _, a := range addrSlice {
				evm[chainID][a] = struct{}{}
			}

			return nil
		}

		if IsSolanaChain(chainID) {
			var addrSlice []SolanaAddress
			if err := cbor.Unmarshal(addrBytes, &addrSlice); err != nil {
				return err
			}

			if sol[chainID] == nil {
				sol[chainID] = make(map[SolanaAddress]struct{})
			}

			for _, a := range addrSlice {
				sol[chainID][a] = struct{}{}
			}

			return nil
		}

		return fmt.Errorf("%w: unknown chain type %d", ErrUnknownChain, chainID)
	})
	if err != nil {
		return nil, nil, err
	}

	return evm, sol, nil
}

func putChainAddresses[T Address](tx kv.RwTx, chainID apptypes.ChainType, addrs []T) error {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], uint64(chainID))

	if len(addrs) == 0 {
		return tx.Delete(SubscriptionBucket, key[:])
	}

	data, err := cbor.Marshal(addrs)
	if err != nil {
		return err
	}

	return tx.Put(SubscriptionBucket, key[:], data)
}

func bytesOf[T Address](a T) []byte {
	b := make([]byte, len(a))

	for i := range len(a) {
		b[i] = a[i]
	}

	return b
}
