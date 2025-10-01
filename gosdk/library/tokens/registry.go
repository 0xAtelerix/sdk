package tokens

import (
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Meta carries useful provenance for the decoded event.
type Meta struct {
	Contract common.Address // lg.Address
	TxHash   common.Hash    // receipt.TxHash
	LogIndex uint           // lg.Index
}

// R is whatever the user wants to produce.
type Handler[R any] func(meta Meta, val any) ([]R, error)

type EventType any

// internal entry
type regEntry[R any] struct {
	abi       abi.ABI
	eventName string
	decode    func(a abi.ABI, name string, lg *gethtypes.Log) (EventType, bool, error)
	mapper    Handler[R]
}

type Registry[R any] struct {
	mu sync.RWMutex
	// Allow multiple entries per topic0 so ERC-20/ERC-721 can coexist.
	m map[common.Hash][]regEntry[R]
}

func NewRegistry[R any]() *Registry[R] {
	return &Registry[R]{m: make(map[common.Hash][]regEntry[R])}
}

// Register binds a topic0 to a typed decoder T and a mapper to R. T is an event type.
func Register[T any, R any](
	r *Registry[R],
	evSig common.Hash,
	a abi.ABI,
	eventName string,
	mapper func(T, Meta) ([]R, error),
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// TODO: add event check on eventName

	entry := regEntry[R]{
		abi:       a,
		eventName: eventName,
		decode: func(a abi.ABI, name string, lg *gethtypes.Log) (EventType, bool, error) {
			return DecodeEventInto[T](a, name, lg)
		},
		mapper: func(meta Meta, v any) ([]R, error) {
			return mapper(v.(T), meta)
		},
	}
	r.m[evSig] = append(r.m[evSig], entry)
}

// HandleLog tries to decode and map a single log into []R based on topic0.
func (r *Registry[R]) HandleLog(lg *gethtypes.Log, txHash common.Hash) ([]R, bool, error) {
	if len(lg.Topics) == 0 {
		return nil, false, nil
	}

	r.mu.RLock()
	entries, ok := r.m[lg.Topics[0]]
	r.mu.RUnlock()

	if !ok || len(entries) == 0 {
		return nil, false, nil
	}

	meta := Meta{Contract: lg.Address, TxHash: txHash, LogIndex: lg.Index}

	// Try all candidates registered under this signature until one matches.
	for _, ent := range entries {
		val, matched, err := ent.decode(ent.abi, ent.eventName, lg)
		if !matched {
			continue
		}

		if err != nil {
			// If decode matched but errored, surface it (bad data)
			return nil, true, err
		}

		out, mapErr := ent.mapper(meta, val)

		return out, true, mapErr
	}

	// None matched
	return nil, false, nil
}
