package tokens

// Unifying result interface for ALL events your app cares about.
// TODO: remove R and T types in favour of AppEvent + Event[T]
type AppEvent interface {
	Kind() string
}
