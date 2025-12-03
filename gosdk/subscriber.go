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

// Subscriber tracks on-chain subscriptions for external block filtering.
// A subscription is keyed by contract and an optional set of topic0 values.
// If the topic set is nil/empty, the subscription matches all topics for that contract.
type Subscriber struct {
	ethTopics    map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{} // nil/empty topic set = wildcard  // chainID -> EthereumContractAddress -> topic
	solAddresses map[apptypes.ChainType]map[SolanaAddress]struct{}                // chainID -> SolanaAddress

	deletedEthTopics map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{}
	deletedSolAddr   map[apptypes.ChainType]map[SolanaAddress]struct{}

	mu sync.RWMutex
}

func NewSubscriber(ctx context.Context, tx kv.RoDB) (*Subscriber, error) {
	roTx, err := tx.BeginRo(ctx)
	if err != nil {
		return nil, err
	}

	defer roTx.Rollback()

	sub := &Subscriber{
		ethTopics:        make(map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{}),
		solAddresses:     make(map[apptypes.ChainType]map[SolanaAddress]struct{}),
		deletedEthTopics: make(map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{}),
		deletedSolAddr:   make(map[apptypes.ChainType]map[SolanaAddress]struct{}),
	}

	sub.ethTopics, sub.solAddresses, err = loadAllSubscriptions(roTx)
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

	if _, ok := s.ethTopics[chainID]; !ok {
		s.ethTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
	}
	if _, ok := s.deletedEthTopics[chainID]; !ok {
		s.deletedEthTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
	}

	for _, contract := range contracts {
		delete(s.deletedEthTopics[chainID], contract)
		s.ethTopics[chainID][contract] = nil // wildcard topics
	}
}

// SubscribeEthContractWithTopics registers a contract subscription scoped to specific topic0 values.
// Passing no topics (or a zero topic) records a wildcard subscription for that contract.
func (s *Subscriber) SubscribeEthContractWithTopics(
	chainID apptypes.ChainType,
	contract EthereumAddress,
	topics ...[32]byte,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ethTopics[chainID] == nil {
		s.ethTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
	}

	if s.deletedEthTopics[chainID] == nil {
		s.deletedEthTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
	}

	// zero topic means wildcard
	if len(topics) == 0 || topics[0] == ([32]byte{}) {
		s.ethTopics[chainID][contract] = nil
		delete(s.deletedEthTopics[chainID], contract)
		return
	}

	if s.ethTopics[chainID][contract] == nil {
		s.ethTopics[chainID][contract] = make(map[[32]byte]struct{})
	}

	for _, topic := range topics {
		delete(s.deletedEthTopics[chainID][contract], topic)
		s.ethTopics[chainID][contract][topic] = struct{}{}
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

	if _, ok := s.ethTopics[chainID]; !ok {
		return
	}

	if _, ok := s.deletedEthTopics[chainID]; !ok {
		s.deletedEthTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
	}

	for _, contract := range contracts {
		delete(s.ethTopics[chainID], contract)
		s.deletedEthTopics[chainID][contract] = nil // nil marks full delete
	}
}

// UnsubscribeEthTopics removes topic filters for a contract; if topics is empty, remove all topics.
func (s *Subscriber) UnsubscribeEthTopics(
	chainID apptypes.ChainType,
	contract EthereumAddress,
	topics ...[32]byte,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	topicSet := s.ethTopics[chainID][contract]
	if topicSet == nil {
		return
	}

	if s.deletedEthTopics[chainID] == nil {
		s.deletedEthTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
	}

	if s.deletedEthTopics[chainID][contract] == nil {
		s.deletedEthTopics[chainID][contract] = make(map[[32]byte]struct{})
	}

	if len(topics) == 0 {
		for t := range topicSet {
			delete(topicSet, t)
			s.deletedEthTopics[chainID][contract][t] = struct{}{}
		}

		return
	}

	for _, topic := range topics {
		delete(topicSet, topic)
		s.deletedEthTopics[chainID][contract][topic] = struct{}{}
	}
}

func (s *Subscriber) IsEthSubscription(chainID apptypes.ChainType, contract EthereumAddress) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// if there is no subscriptions, give all blocks
	if len(s.ethTopics[chainID]) == 0 {
		return true
	}

	_, ok := s.ethTopics[chainID][contract]
	return ok
}

// IsEthTopicSubscription returns true when the given contract/topic0 pair is subscribed.
// A contract-only subscription (without topic filter) will match any topic.
func (s *Subscriber) IsEthTopicSubscription(
	chainID apptypes.ChainType,
	contract EthereumAddress,
	topic0 [32]byte,
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// If there are no EVM subscriptions at all for the chain, treat as wildcard.
	if len(s.ethTopics[chainID]) == 0 {
		return true
	}

	topics := s.ethTopics[chainID][contract]
	if topics == nil || len(topics) == 0 {
		return true
	}

	// wildcard topic recorded
	if _, ok := topics[[32]byte{}]; ok {
		return true
	}

	_, ok := topics[topic0]

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
		delete(s.deletedSolAddr[chainID], address)
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

	if _, ok := s.deletedSolAddr[chainID]; !ok {
		s.deletedSolAddr[chainID] = make(map[SolanaAddress]struct{})
	}

	for _, address := range addresses {
		delete(s.solAddresses[chainID], address)

		s.deletedSolAddr[chainID][address] = struct{}{}
	}
}

func (s *Subscriber) Store(tx kv.RwTx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	curTopics, solSet, err := loadAllSubscriptions(tx)
	if err != nil {
		return err
	}

	for chainID := range s.ethTopics {
		if curTopics[chainID] == nil {
			curTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
		}
	}
	for chainID := range s.deletedEthTopics {
		if curTopics[chainID] == nil {
			curTopics[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
		}
	}
	for chainID := range s.solAddresses {
		if solSet[chainID] == nil {
			solSet[chainID] = make(map[SolanaAddress]struct{})
		}
	}
	for chainID := range s.deletedSolAddr {
		if solSet[chainID] == nil {
			solSet[chainID] = make(map[SolanaAddress]struct{})
		}
	}

	for chainID, delSet := range s.deletedEthTopics {
		for addr, topics := range delSet {
			if topics == nil {
				delete(curTopics[chainID], addr)
				continue
			}
			if curTopics[chainID][addr] == nil {
				continue
			}
			for topic := range topics {
				delete(curTopics[chainID][addr], topic)
			}
			if len(curTopics[chainID][addr]) == 0 {
				delete(curTopics[chainID], addr)
			}
		}
	}

	for chainID, delSet := range s.deletedSolAddr {
		for addr := range delSet {
			delete(solSet[chainID], addr)
		}
	}

	for chainID, addSet := range s.ethTopics {
		for addr, topics := range addSet {
			if topics == nil {
				curTopics[chainID][addr] = nil
				continue
			}
			if curTopics[chainID][addr] == nil {
				curTopics[chainID][addr] = make(map[[32]byte]struct{})
			}
			for topic := range topics {
				curTopics[chainID][addr][topic] = struct{}{}
			}
		}
	}

	for chainID, addSet := range s.solAddresses {
		for addr := range addSet {
			solSet[chainID][addr] = struct{}{}
		}
	}

	// Remove empty chains from topics and delete their bucket entries.
	for chainID, addrMap := range curTopics {
		if len(addrMap) == 0 {
			var key [8]byte
			const topicKeyMask uint64 = 1 << 63
			binary.BigEndian.PutUint64(key[:], uint64(chainID)|topicKeyMask)
			_ = tx.Delete(SubscriptionBucket, key[:])
			delete(curTopics, chainID)
		}
	}

	evmTopicItems := CollectChainTopicsGrouped(curTopics)
	solItems := CollectChainAddresses(solSet)

	for chainID, entries := range evmTopicItems {
		if err = putChainTopics(tx, chainID, entries); err != nil {
			return err
		}
	}

	for _, ca := range solItems {
		if err = putChainAddresses(tx, ca.chainID, ca.addresses); err != nil {
			return err
		}
	}

	s.deletedEthTopics = make(map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{})
	s.deletedSolAddr = make(map[apptypes.ChainType]map[SolanaAddress]struct{})

	return nil
}

// loadAllSubscriptions reads the whole subscription bucket and returns
// two in-memory sets: EVM and Solana, keyed by chainID then address.
func loadAllSubscriptions(tx kv.Tx) (
	map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{},
	map[apptypes.ChainType]map[SolanaAddress]struct{},
	error,
) {
	evm := make(map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{})
	sol := make(map[apptypes.ChainType]map[SolanaAddress]struct{})

	err := tx.ForEach(SubscriptionBucket, nil, func(chainIDBytes, addrBytes []byte) error {
		raw := binary.BigEndian.Uint64(chainIDBytes)
		const topicKeyMask uint64 = 1 << 63
		isTopicKey := (raw & topicKeyMask) != 0
		chainID := apptypes.ChainType(raw &^ topicKeyMask)

		if IsEvmChain(chainID) {
			if evm[chainID] == nil {
				evm[chainID] = make(map[EthereumAddress]map[[32]byte]struct{})
			}

			if isTopicKey {
				var topicSlice []chainTopicAddresses
				if err := cbor.Unmarshal(addrBytes, &topicSlice); err != nil {
					return err
				}

				for _, entry := range topicSlice {
					if evm[chainID][entry.Address] == nil {
						evm[chainID][entry.Address] = make(map[[32]byte]struct{})
					}
					for _, t := range entry.Topics {
						evm[chainID][entry.Address][t] = struct{}{}
					}
				}

				return nil
			}

			// legacy encoding: address-only means wildcard topics
			var addrSlice []EthereumAddress
			if err := cbor.Unmarshal(addrBytes, &addrSlice); err != nil {
				return err
			}

			for _, a := range addrSlice {
				evm[chainID][a] = nil
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

type chainTopicAddresses struct {
	Address EthereumAddress `cbor:"1,keyasint"`
	Topics  [][32]byte      `cbor:"2,keyasint"`
}

// CollectChainTopicsGrouped converts nested maps into a stable slice per chain.
func CollectChainTopicsGrouped(
	src map[apptypes.ChainType]map[EthereumAddress]map[[32]byte]struct{},
) map[apptypes.ChainType][]chainTopicAddresses {
	out := make(map[apptypes.ChainType][]chainTopicAddresses)

	for chainID, addrMap := range src {
		for addr, topics := range addrMap {
			item := chainTopicAddresses{
				Address: addr,
				Topics:  make([][32]byte, 0, len(topics)),
			}

			if topics != nil {
				for t := range topics {
					item.Topics = append(item.Topics, t)
				}
			}

			out[chainID] = append(out[chainID], item)
		}
	}

	return out
}

func putChainTopics(tx kv.RwTx, chainID apptypes.ChainType, entries []chainTopicAddresses) error {
	if len(entries) == 0 {
		return nil
	}

	data, err := cbor.Marshal(entries)
	if err != nil {
		return err
	}

	var key [8]byte
	const topicKeyMask uint64 = 1 << 63
	binary.BigEndian.PutUint64(key[:], uint64(chainID)|topicKeyMask)

	return tx.Put(SubscriptionBucket, key[:], data)
}

func bytesOf[T Address](a T) []byte {
	b := make([]byte, len(a))

	for i := range len(a) {
		b[i] = a[i]
	}

	return b
}
