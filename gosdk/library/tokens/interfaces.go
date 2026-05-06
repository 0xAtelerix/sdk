package tokens

// AppEvent is the unifying result interface for all events an app cares about.
type AppEvent interface {
	Name() string
}
