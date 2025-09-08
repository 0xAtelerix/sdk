package gosdk

import (
	"context"
	"encoding/binary"
	"sync"

	"github.com/ledgerwatch/erigon-lib/kv"
)

type EthereumAddress [EthereumAddressLength]byte

type SolanaAddress [SolanaAddressLength]byte

const (
	EthereumAddressLength = 20
	SolanaAddressLength   = 32
)

type Subscriber struct {
	ethContracts map[uint64]map[EthereumAddress]struct{} // chainID -> EthereumContractAddress
	solAddresses map[uint64]map[SolanaAddress]struct{}   // chainID -> SolanaAddress

	mu sync.RWMutex
}

func NewSubscriber(ctx context.Context, tx kv.RoDB) (*Subscriber, error) {
	roTx, err := tx.BeginRo(ctx)
	if err != nil {
		return nil, err
	}

	sub := &Subscriber{
		ethContracts: make(map[uint64]map[EthereumAddress]struct{}),
		solAddresses: make(map[uint64]map[SolanaAddress]struct{}),
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
			return

		}

		return nil
	})

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
}
