package subscriber

import "github.com/0xAtelerix/sdk/gosdk/library/tokens"

type AppEventHandler interface {
	Kind() string
	Handle(evs []tokens.AppEvent)
}

type HandlerFor[T any] struct {
	EventKind string
	Filter    tokens.EventFilter[T]
	Handler   func(tokens.Event[T])
}

func (h HandlerFor[T]) Kind() string {
	return h.EventKind
}

func (h HandlerFor[T]) Handle(evs []tokens.AppEvent) {
	for _, e := range h.Filter(evs) {
		// filter narrows []AppEvent -> []Event[T]
		h.Handler(e)
	}
}

// NewEVMHandler builds a type-safe handler and returns a type-erased adapter.
// returns an interface to hide the type parameter
func NewEVMHandler[T any](
	kind string,
	filter tokens.EventFilter[T],
	fn func(tokens.Event[T]),
) AppEventHandler {
	return HandlerFor[T]{
		EventKind: kind,
		Filter:    filter,
		Handler:   fn,
	}
}
