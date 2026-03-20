package probe

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/datasource"
)

var _ datasource.DataSourceCacheObserver = (*LoggingDataSourceCacheObserver)(nil)

// LoggingDataSourceCacheObserver logs data source cache events via zerolog.
type LoggingDataSourceCacheObserver struct {
	logger zerolog.Logger
}

func NewLoggingDataSourceCacheObserver(logger zerolog.Logger) *LoggingDataSourceCacheObserver {
	return &LoggingDataSourceCacheObserver{logger: logger}
}

func (o *LoggingDataSourceCacheObserver) DataSourceCacheProbe(_ context.Context, dataSourceName string) datasource.DataSourceCacheProbe {
	return &loggingDataSourceCacheProbe{
		logger: o.logger.With().Str("datasource", dataSourceName).Logger(),
	}
}

type loggingDataSourceCacheProbe struct {
	logger zerolog.Logger
}

func (p *loggingDataSourceCacheProbe) CacheHit() {
	p.logger.Debug().Msg("cache hit")
}

func (p *loggingDataSourceCacheProbe) CacheMiss() {
	p.logger.Debug().Msg("cache miss")
}

func (p *loggingDataSourceCacheProbe) CacheExpired() {
	p.logger.Debug().Msg("cache entry expired")
}

func (p *loggingDataSourceCacheProbe) FetchFailed(err error) {
	p.logger.Warn().Err(err).Msg("data source fetch failed")
}
