package gosdk

import (
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/library/tokens"
)

type AppEventHandler interface {
	Name() string
	Handle(evs []tokens.AppEvent, tx kv.RwTx)
}

type HandlerFor[T any] struct {
	EventName string
	Filter    tokens.EventFilter[T]
	Handler   func(tokens.Event[T], kv.RwTx)
}

func (h HandlerFor[T]) Name() string {
	return h.EventName
}

func (h HandlerFor[T]) Handle(evs []tokens.AppEvent, tx kv.RwTx) {
	for _, e := range h.Filter(evs) {
		// filter narrows []AppEvent -> []Event[T]
		h.Handler(e, tx)
	}
}

// NewEVMHandler builds a type-safe handler and returns a type-erased adapter.
// returns an interface to hide the type parameter
func NewEVMHandler[T any](
	name string,
	fn func(tokens.Event[T], kv.RwTx),
) AppEventHandler {
	return HandlerFor[T]{
		EventName: name,
		Filter:    tokens.Filter[T],
		Handler:   fn,
	}
}
