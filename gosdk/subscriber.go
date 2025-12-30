package gosdk

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
)

// Subscriber tracks on-chain subscriptions for external block filtering.
// A subscription is keyed by contract and an optional set of topic0 values.
// If the topic set is nil/empty, the subscription matches all topics for that contract.
type Subscriber struct {
	ethContracts map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{} // chainID -> EthereumContractAddress -> topic
	solAddresses map[apptypes.ChainType]map[library.SolanaAddress]struct{}                             // chainID -> SolanaAddress

	EVMEventRegistry *tokens.Registry[tokens.AppEvent]
	evmHandlers      map[string]AppEventHandler // eventName -> Handler

	solanaEventRegistry *tokens.Registry[tokens.AppEvent]

	deletedEthContracts map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{} // chainID -> EthereumContractAddress -> topic
	deletedSolAddresses map[apptypes.ChainType]map[library.SolanaAddress]struct{}                             // chainID -> SolanaAddress

	mu sync.RWMutex
}

func NewSubscriber(ctx context.Context, tx kv.RoDB) (*Subscriber, error) {
	roTx, err := tx.BeginRo(ctx)
	if err != nil {
		return nil, err
	}

	defer roTx.Rollback()

	sub := &Subscriber{
		ethContracts: make(
			map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
		),
		solAddresses:        make(map[apptypes.ChainType]map[library.SolanaAddress]struct{}),
		EVMEventRegistry:    tokens.NewRegistry[tokens.AppEvent](),
		evmHandlers:         make(map[string]AppEventHandler),
		solanaEventRegistry: tokens.NewRegistry[tokens.AppEvent](),
		deletedEthContracts: make(
			map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
		),
		deletedSolAddresses: make(map[apptypes.ChainType]map[library.SolanaAddress]struct{}),
	}

	sub.ethContracts, sub.solAddresses, err = loadAllSubscriptions(roTx)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func (s *Subscriber) SubscribeEthContract(
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	h AppEventHandler, // optional
	topics ...library.EthereumTopic, // optional
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		s.ethContracts[chainID] = make(
			map[library.EthereumAddress]map[library.EthereumTopic]struct{},
		)
	}

	if s.ethContracts[chainID][contract] == nil {
		s.ethContracts[chainID][contract] = make(map[library.EthereumTopic]struct{})
	}

	if len(topics) == 0 || topics[0] == (library.EthereumTopic{}) {
		delete(s.deletedEthContracts[chainID], contract)
		s.ethContracts[chainID][contract] = nil // wildcard topics
	} else {
		for _, topic := range topics {
			delete(s.deletedEthContracts[chainID][contract], topic)
			s.ethContracts[chainID][contract][topic] = struct{}{}
		}
	}

	if h != nil {
		if s.evmHandlers == nil {
			s.evmHandlers = make(map[string]AppEventHandler)
		}

		s.evmHandlers[h.Name()] = h
	}
}

func (s *Subscriber) UnsubscribeEthContract(
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	h AppEventHandler, // optional
	topics ...library.EthereumTopic, // optional
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ethContracts[chainID]; !ok {
		return
	}

	if _, ok := s.deletedEthContracts[chainID]; !ok {
		s.deletedEthContracts[chainID] = make(
			map[library.EthereumAddress]map[library.EthereumTopic]struct{},
		)
	}

	if s.deletedEthContracts[chainID][contract] == nil {
		s.deletedEthContracts[chainID][contract] = make(map[library.EthereumTopic]struct{})
	}

	topicSet, ok := s.ethContracts[chainID][contract]
	if !ok {
		return
	}

	// Wildcard existing: remove entirely.
	if topicSet == nil {
		delete(s.ethContracts[chainID], contract)

		s.deletedEthContracts[chainID][contract][library.EmptyEthereumTopic()] = struct{}{}

		return
	}

	if h != nil {
		if s.evmHandlers == nil {
			return
		}

		delete(s.evmHandlers, h.Name())
	}

	for _, topic := range topics {
		delete(topicSet, topic)

		s.deletedEthContracts[chainID][contract][topic] = struct{}{}
	}

	if len(topicSet) == 0 {
		delete(s.ethContracts[chainID], contract)

		s.deletedEthContracts[chainID][contract][library.EmptyEthereumTopic()] = struct{}{}
	}
}

// IsEthSubscription returns true when the given contract/topic0 pair is subscribed.
// A contract-only subscription (without topic filter) will match any topic.
func (s *Subscriber) IsEthSubscription(
	chainID apptypes.ChainType,
	contract library.EthereumAddress,
	topic ...library.EthereumTopic, // optional
) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// if there is no subscriptions, give all blocks
	if len(s.ethContracts[chainID]) == 0 {
		return true
	}

	topics, ok := s.ethContracts[chainID][contract]
	if !ok {
		return false
	}

	if len(topics) == 0 {
		return true
	}

	// wildcard topic recorded
	if _, ok = topics[library.EthereumTopic{}]; ok {
		return true
	}

	if len(topic) > 0 {
		_, ok = topics[topic[0]]
	}

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

	if _, ok := s.solAddresses[chainID]; !ok {
		s.solAddresses[chainID] = make(map[library.SolanaAddress]struct{})
	}

	for _, address := range addresses {
		delete(s.deletedSolAddresses[chainID], address)
		s.solAddresses[chainID][address] = struct{}{}
	}
}

func (s *Subscriber) IsSolanaSubscription(
	chainID apptypes.ChainType,
	address library.SolanaAddress,
) bool {
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
	addresses ...library.SolanaAddress,
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
		s.deletedSolAddresses[chainID] = make(map[library.SolanaAddress]struct{})
	}

	for _, address := range addresses {
		delete(s.solAddresses[chainID], address)

		s.deletedSolAddresses[chainID][address] = struct{}{}
	}
}

func (s *Subscriber) Store(tx kv.RwTx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	subscribedTopics, solSet, err := loadAllSubscriptions(tx)
	if err != nil {
		return err
	}

	// Ensure inner maps exist for chains referenced by pending ops
	for chainID := range s.ethContracts {
		if subscribedTopics[chainID] == nil {
			subscribedTopics[chainID] = make(
				map[library.EthereumAddress]map[library.EthereumTopic]struct{},
			)
		}
	}

	for chainID := range s.deletedEthContracts {
		if subscribedTopics[chainID] == nil {
			subscribedTopics[chainID] = make(
				map[library.EthereumAddress]map[library.EthereumTopic]struct{},
			)
		}
	}

	for chainID := range s.solAddresses {
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
		for addr, topics := range delSet {
			if topics == nil {
				delete(subscribedTopics[chainID], addr)

				continue
			}

			if subscribedTopics[chainID][addr] == nil {
				continue
			}

		topicLoop:
			for topic := range topics {
				if topic == library.EmptyEthereumTopic() {
					delete(subscribedTopics[chainID], addr)

					break topicLoop
				}

				delete(subscribedTopics[chainID][addr], topic)
			}

			if len(subscribedTopics[chainID][addr]) == 0 {
				delete(subscribedTopics[chainID], addr)
			}
		}
	}

	for chainID, delSet := range s.deletedSolAddresses {
		for addr := range delSet {
			delete(solSet[chainID], addr)
		}
	}

	// 3) Apply appends (merge additions)
	for chainID, addSet := range s.ethContracts {
		for addr, topics := range addSet {
			if topics == nil {
				subscribedTopics[chainID][addr] = nil

				continue
			}

			if subscribedTopics[chainID][addr] == nil {
				subscribedTopics[chainID][addr] = make(map[library.EthereumTopic]struct{})
			}

			for topic := range topics {
				subscribedTopics[chainID][addr][topic] = struct{}{}
			}
		}
	}

	for chainID, addSet := range s.solAddresses {
		for addr := range addSet {
			solSet[chainID][addr] = struct{}{}
		}
	}

	// Remove empty chains from topics and delete their bucket entries.
	for chainID, addrMap := range subscribedTopics {
		if len(addrMap) == 0 {
			var key [8]byte

			const topicKeyMask uint64 = 1 << 63
			binary.BigEndian.PutUint64(key[:], uint64(chainID)|topicKeyMask)
			_ = tx.Delete(SubscriptionBucket, key[:])

			delete(subscribedTopics, chainID)
		}
	}

	// 4) Sort (deterministic ordering)
	evmTopicItems := library.CollectChainTopicsGrouped(
		subscribedTopics,
	) // []chainAddresses[EthereumAddress]
	solItems := library.CollectChainAddresses(
		solSet,
	) // []chainAddresses[SolanaAddress]

	// 5) Write back (CBOR per chain)
	// EVM
	for chainID, entries := range evmTopicItems {
		if err = putChainTopics(tx, chainID, entries); err != nil {
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
	s.deletedEthContracts = make(
		map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
	)
	s.deletedSolAddresses = make(map[apptypes.ChainType]map[library.SolanaAddress]struct{})

	return nil
}

func (s *Subscriber) Handle(eventName string, evs []tokens.AppEvent, tx kv.RwTx) {
	h, ok := s.evmHandlers[eventName]
	if !ok || h == nil {
		return
	}

	h.Handle(evs, tx)
}

// loadAllSubscriptions reads the whole subscription bucket and returns
// two in-memory sets: EVM and Solana, keyed by chainID then address.
func loadAllSubscriptions(tx kv.Tx) (
	map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
	map[apptypes.ChainType]map[library.SolanaAddress]struct{},
	error,
) {
	evm := make(
		map[apptypes.ChainType]map[library.EthereumAddress]map[library.EthereumTopic]struct{},
	)
	sol := make(map[apptypes.ChainType]map[library.SolanaAddress]struct{})

	err := tx.ForEach(SubscriptionBucket, nil, func(chainIDBytes, addrBytes []byte) error {
		raw := binary.BigEndian.Uint64(chainIDBytes)

		const topicKeyMask uint64 = 1 << 63

		isTopicKey := (raw & topicKeyMask) != 0
		chainID := apptypes.ChainType(raw &^ topicKeyMask)

		if library.IsEvmChain(chainID) {
			if evm[chainID] == nil {
				evm[chainID] = make(map[library.EthereumAddress]map[library.EthereumTopic]struct{})
			}

			if isTopicKey {
				var topicSlice []library.ChainTopicAddresses
				if err := cbor.Unmarshal(addrBytes, &topicSlice); err != nil {
					return err
				}

				for _, entry := range topicSlice {
					if evm[chainID][entry.Address] == nil {
						evm[chainID][entry.Address] = make(map[library.EthereumTopic]struct{})
					}

					for _, t := range entry.Topics {
						evm[chainID][entry.Address][t] = struct{}{}
					}
				}

				return nil
			}

			// legacy encoding: address-only means wildcard topics
			var addrSlice []library.EthereumAddress
			if err := cbor.Unmarshal(addrBytes, &addrSlice); err != nil {
				return err
			}

			for _, a := range addrSlice {
				evm[chainID][a] = nil
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
		return tx.Delete(SubscriptionBucket, key[:])
	}

	data, err := cbor.Marshal(addrs)
	if err != nil {
		return err
	}

	return tx.Put(SubscriptionBucket, key[:], data)
}

func bytesOf[T library.Address](a T) []byte {
	b := make([]byte, len(a))

	for i := range len(a) {
		b[i] = a[i]
	}

	return b
}

func putChainTopics(
	tx kv.RwTx,
	chainID apptypes.ChainType,
	entries []library.ChainTopicAddresses,
) error {
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
