package probe

import (
	"github.com/rs/zerolog"
)

// LoggingConfigReloadObserver logs configuration reload events via zerolog.
// It satisfies config.ConfigReloadObserver via structural typing to avoid
// a circular import (config already imports probe).
type LoggingConfigReloadObserver struct {
	Logger zerolog.Logger
}

func (o *LoggingConfigReloadObserver) ConfigReloadFailed(step string, err error) {
	o.Logger.Error().Err(err).Str("step", step).Msg("config reload failed")
}
