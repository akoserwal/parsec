package probe

import (
	"github.com/rs/zerolog"
)

// LoggingConfigReloadObserver logs configuration reload events via zerolog.
// It satisfies config.ConfigReloadObserver via structural typing to avoid
// a circular import (config already imports probe).
type LoggingConfigReloadObserver struct {
	logger zerolog.Logger
}

func NewLoggingConfigReloadObserver(logger zerolog.Logger) *LoggingConfigReloadObserver {
	return &LoggingConfigReloadObserver{logger: logger}
}

func (o *LoggingConfigReloadObserver) ConfigReloadFailed(step string, err error) {
	o.logger.Error().Err(err).Str("step", step).Msg("config reload failed")
}
