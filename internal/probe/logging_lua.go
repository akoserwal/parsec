package probe

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/datasource"
)

var _ datasource.LuaDataSourceObserver = (*LoggingLuaDataSourceObserver)(nil)

type LoggingLuaDataSourceObserver struct {
	logger zerolog.Logger
}

func NewLoggingLuaDataSourceObserver(logger zerolog.Logger) *LoggingLuaDataSourceObserver {
	return &LoggingLuaDataSourceObserver{logger: logger}
}

func (o *LoggingLuaDataSourceObserver) LuaDataSourceProbe(_ context.Context, dataSourceName string) datasource.LuaDataSourceProbe {
	return &loggingLuaDataSourceProbe{
		logger: o.logger.With().Str("datasource", dataSourceName).Logger(),
	}
}

type loggingLuaDataSourceProbe struct {
	logger zerolog.Logger
}

func (p *loggingLuaDataSourceProbe) ScriptLoadFailed(err error) {
	p.logger.Error().Err(err).Msg("lua script load failed")
}

func (p *loggingLuaDataSourceProbe) ScriptExecutionFailed(err error) {
	p.logger.Error().Err(err).Msg("lua script execution failed")
}

func (p *loggingLuaDataSourceProbe) InvalidReturnType(got string) {
	p.logger.Error().Str("got", got).Msg("lua fetch returned invalid type (expected table or nil)")
}

func (p *loggingLuaDataSourceProbe) FetchCompleted() {
	p.logger.Debug().Msg("lua fetch completed")
}
