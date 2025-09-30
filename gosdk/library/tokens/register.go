package tokens

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// User should define T as an event type
// - Kind: a short string you choose at registration time (e.g. "erc20.approval")
// - Contract/TxHash/LogIndex: provenance
// - Payload: the decoded ABI struct T (whatever the user defined)
type Event[T any] struct {
	SubscribedEvent T // From ABI

	// From meta (auto-injected):
	Kind     string
	Contract string // lg.Address (contract)
	TxHash   string
	LogIndex uint
}

// RegisterEvent registers an event decoder for T and maps it into GenericEvent.
// 'kind' is any label you want to see on the result (e.g. "erc20.approval").
func RegisterEvent[T any](
	r *Registry[Event[T]],
	a abi.ABI,
	eventName string,
	kind string,
) (common.Hash, error) {
	ev, ok := a.Events[eventName]
	if !ok {
		return common.Hash{}, fmt.Errorf("%w: %s", ErrABIUnknownEvent, eventName)
	}

	sig := ev.ID

	Register[T, Event[T]](r, sig, a, eventName,
		func(t T, m Meta) ([]Event[T], error) {
			return []Event[T]{{
				Kind:            kind,
				Contract:        m.Contract.Hex(),
				TxHash:          m.TxHash.Hex(),
				LogIndex:        m.LogIndex,
				SubscribedEvent: t, // the user's T
			}}, nil
		},
	)

	return sig, nil
}
