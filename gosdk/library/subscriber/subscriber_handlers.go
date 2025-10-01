package subscriber

import (
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/0xAtelerix/sdk/gosdk/library/tokens"
)

type AppEventHandler interface {
	Kind() string
	Handle(evs []tokens.AppEvent, tx kv.RwTx)
}

type HandlerFor[T any] struct {
	EventKind string
	Filter    tokens.EventFilter[T]
	Handler   func(tokens.Event[T], kv.RwTx)
}

func (h HandlerFor[T]) Kind() string {
	return h.EventKind
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
	kind string,
	fn func(tokens.Event[T], kv.RwTx),
) AppEventHandler {
	return HandlerFor[T]{
		EventKind: kind,
		Filter:    tokens.Filter[T],
		Handler:   fn,
	}
}
