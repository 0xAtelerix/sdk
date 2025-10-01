package tokens

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// User should define T as an event type
// - Kind: a short string you choose at registration time (e.g. "erc20.approval")
// - Contract/TxHash/LogIndex: provenance
// - SubscribedEvent: the decoded ABI struct T (whatever the user defined)
type Event[T any] struct {
	SubscribedEvent T // From ABI

	// From meta (auto-injected):
	EventKind string
	Contract  string // lg.Address (contract)
	TxHash    string
	LogIndex  uint
}

// Implement AppEvent.
func (e Event[T]) Kind() string { return e.EventKind }

// A typed filter for pulling Event[T] out of a []AppEvent.
type EventFilter[T any] func([]AppEvent) []Event[T]

// RegisterEvent registers an event decoder for T and maps it into AppEvent.
// 'kind' is any label you want to see on the result (e.g. "erc20.approval").
func RegisterEvent[T any](
	r *Registry[AppEvent],
	a abi.ABI,
	eventName string,
	kind string,
) (common.Hash, error) {
	ev, ok := a.Events[eventName]
	if !ok {
		return common.Hash{}, fmt.Errorf("%w: %s", ErrABIUnknownEvent, eventName)
	}

	sig := ev.ID

	// Reuse the generic Register but target the unified AppEvent registry.
	Register[T, AppEvent](r, sig, a, eventName,
		func(t T, m Meta) ([]AppEvent, error) {
			w := Event[T]{
				EventKind:       kind,
				Contract:        m.Contract.Hex(),
				TxHash:          m.TxHash.Hex(),
				LogIndex:        m.LogIndex,
				SubscribedEvent: t,
			}

			return []AppEvent{w}, nil
		},
	)

	return sig, nil
}
