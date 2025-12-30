package tokens

func As[T any](ev AppEvent) (Event[T], bool) {
	v, ok := ev.(Event[T])

	return v, ok
}

func Filter[T any](evs []AppEvent) []Event[T] {
	out := make([]Event[T], 0, len(evs))
	for _, e := range evs {
		if v, ok := e.(Event[T]); ok {
			out = append(out, v)
		}
	}

	return out
}
