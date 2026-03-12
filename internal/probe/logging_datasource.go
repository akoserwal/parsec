package probe

import (
	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/datasource"
)

// Compile-time interface check.
var _ datasource.DataSourceCacheObserver = (*LoggingDataSourceCacheObserver)(nil)

// LoggingDataSourceCacheObserver logs data source cache events via zerolog.
type LoggingDataSourceCacheObserver struct {
	logger zerolog.Logger
}

func NewLoggingDataSourceCacheObserver(logger zerolog.Logger) *LoggingDataSourceCacheObserver {
	return &LoggingDataSourceCacheObserver{logger: logger}
}

func (o *LoggingDataSourceCacheObserver) CacheHit(dataSourceName string) {
	o.logger.Debug().Str("datasource", dataSourceName).Msg("cache hit")
}

func (o *LoggingDataSourceCacheObserver) CacheMiss(dataSourceName string) {
	o.logger.Debug().Str("datasource", dataSourceName).Msg("cache miss")
}

func (o *LoggingDataSourceCacheObserver) CacheExpired(dataSourceName string) {
	o.logger.Debug().Str("datasource", dataSourceName).Msg("cache entry expired")
}

func (o *LoggingDataSourceCacheObserver) FetchFailed(dataSourceName string, err error) {
	o.logger.Warn().Err(err).Str("datasource", dataSourceName).Msg("data source fetch failed")
}
