package gosdk

import (
	"context"
	"encoding/binary"
	"fmt"
	"slices"
	"sync"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type EthereumAddress [EthereumAddressLength]byte

type SolanaAddress [SolanaAddressLength]byte

type Address interface {
	~[EthereumAddressLength]byte | ~[SolanaAddressLength]byte
}

const (
	EthereumAddressLength = 20
	SolanaAddressLength   = 32
)

type Subscriber struct {
	ethContracts map[uint64]map[EthereumAddress]struct{} // chainID -> EthereumContractAddress
	solAddresses map[uint64]map[SolanaAddress]struct{}   // chainID -> SolanaAddress

	deletedEthContracts map[uint64]map[EthereumAddress]struct{} // chainID -> EthereumContractAddress
	deletedSolAddresses map[uint64]map[SolanaAddress]struct{}   // chainID -> SolanaAddress

	mu sync.RWMutex
}

func NewSubscriber(ctx context.Context, tx kv.RoDB) (*Subscriber, error) {
	roTx, err := tx.BeginRo(ctx)
	if err != nil {
		return nil, err
	}

	defer roTx.Rollback()

	sub := &Subscriber{
		ethContracts:        make(map[uint64]map[EthereumAddress]struct{}),
		solAddresses:        make(map[uint64]map[SolanaAddress]struct{}),
		deletedEthContracts: make(map[uint64]map[EthereumAddress]struct{}),
		deletedSolAddresses: make(map[uint64]map[SolanaAddress]struct{}),
	}

	err = roTx.ForEach(subscriptionBucket, nil, func(chainIDBytes, addrBytes []byte) error {
		chainID := binary.BigEndian.Uint64(chainIDBytes)

		switch len(addrBytes) {
		case EthereumAddressLength:
			var addr [EthereumAddressLength]byte
			copy(addr[:], addrBytes)

			sub.ethContracts[chainID][addr] = struct{}{}
		case SolanaAddressLength:
			var addr [SolanaAddressLength]byte
			copy(addr[:], addrBytes)

			sub.solAddresses[chainID][addr] = struct{}{}
		default:
			return fmt.Errorf("%w: invalid address length %d", ErrUnknownChain, len(addrBytes))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func (s *Subscriber) SubscribeEthContract(chainID uint64, contract EthereumAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		s.ethContracts[chainID] = make(map[EthereumAddress]struct{})
	}

	s.ethContracts[chainID][contract] = struct{}{}
}

func (s *Subscriber) UnsubscribeEthContract(chainID uint64, contract EthereumAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		return
	}

	delete(s.ethContracts[chainID], contract)
	s.deletedEthContracts[chainID][contract] = struct{}{}
}

func (s *Subscriber) IsEthSubscription(chainID uint64, contract EthereumAddress) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		return false
	}

	if _, ok := s.ethContracts[chainID][contract]; !ok {
		return false
	}

	return true
}

func (s *Subscriber) SubscribeSolanaAddress(chainID uint64, address SolanaAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		s.solAddresses[chainID] = make(map[SolanaAddress]struct{})
	}

	s.solAddresses[chainID][address] = struct{}{}
}

func (s *Subscriber) IsSolanaSubscription(chainID uint64, address SolanaAddress) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.solAddresses[chainID]; !ok {
		return false
	}

	if _, ok := s.solAddresses[chainID][address]; !ok {
		return false
	}

	return true
}

func (s *Subscriber) UnsubscribeSolanaAddress(chainID uint64, address SolanaAddress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.solAddresses[chainID]; !ok {
		return
	}

	delete(s.solAddresses[chainID], address)
	s.deletedSolAddresses[chainID][address] = struct{}{}
}

func (s *Subscriber) Store(tx kv.RwTx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := deleteChainAddresses(tx, s.deletedEthContracts)
	if err != nil {
		return err
	}

	err = deleteChainAddresses(tx, s.deletedSolAddresses)
	if err != nil {
		return err
	}

	err = putChainAddresses(tx, s.ethContracts)
	if err != nil {
		return err
	}

	return putChainAddresses(tx, s.solAddresses)
}

type chainAddresses[T Address] struct {
	chainID   uint64
	addresses []T
}

func cmpAddr[T Address](a, b T) int {
	for i := range len(a) {
		if a[i] < b[i] {
			return -1
		}

		if a[i] > b[i] {
			return 1
		}
	}

	return 0
}

func sortChainAddresses[T Address](items []chainAddresses[T]) {
	slices.SortFunc(items, func(a, b chainAddresses[T]) int {
		switch {
		case a.chainID < b.chainID:
			return -1
		case a.chainID > b.chainID:
			return 1
		default:
			return 0
		}
	})

	for i := range items {
		slices.SortFunc(items[i].addresses, cmpAddr[T])
	}
}

func collectChainAddresses[T Address](m map[uint64]map[T]struct{}) []chainAddresses[T] {
	out := make([]chainAddresses[T], 0, len(m))

	for chainID, set := range m {
		ca := chainAddresses[T]{
			chainID:   chainID,
			addresses: make([]T, 0, len(set)),
		}

		for addr := range set {
			ca.addresses = append(ca.addresses, addr)
		}

		out = append(out, ca)
	}

	sortChainAddresses(out)

	return out
}

func deleteChainAddresses[T Address](tx kv.RwTx, m map[uint64]map[T]struct{}) error {
	items := collectChainAddresses(m)

	var (
		key [8]byte
		err error
	)

	for _, ca := range items {
		binary.BigEndian.PutUint64(key[:], ca.chainID)

		if err = tx.Delete(subscriptionBucket, key[:]); err != nil {
			return err
		}
	}

	return nil
}

func putChainAddresses[T Address](tx kv.RwTx, m map[uint64]map[T]struct{}) error {
	items := collectChainAddresses(m)

	var (
		key [8]byte
		err error
	)

	for _, ca := range items {
		binary.BigEndian.PutUint64(key[:], ca.chainID)

		for _, addr := range ca.addresses {
			if err = tx.Put(subscriptionBucket, key[:], bytesOf(addr)); err != nil {
				return err
			}
		}
	}

	return nil
}

func bytesOf[T Address](a T) []byte {
	b := make([]byte, len(a))

	for i := range len(a) {
		b[i] = a[i]
	}

	return b
}
