package config

// ConfigReloadObserver receives configuration reload events from Loader.
// A nil observer means no events are emitted.
type ConfigReloadObserver interface {
	ConfigReloadFailed(step string, err error)
}
