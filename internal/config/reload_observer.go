package config

// ConfigReloadObserver receives configuration reload events from Loader.
type ConfigReloadObserver interface {
	ConfigReloadFailed(step string, err error)
}

// NoopConfigReloadObserver satisfies ConfigReloadObserver with empty methods.
// Useful in tests that don't care about observer events.
type NoopConfigReloadObserver struct{}

func (NoopConfigReloadObserver) ConfigReloadFailed(string, error) {}
