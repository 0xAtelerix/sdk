package tokens

// Unifying result interface for ALL events your app cares about.
type AppEvent interface {
	Name() string
}
