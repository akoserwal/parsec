package probe

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/keys"
	"github.com/project-kessel/parsec/internal/server"
)

// LoggingDataSourceObserver satisfies datasource.DataSourceObserver
// by combining cache and Lua logging observers.
type LoggingDataSourceObserver struct {
	datasource.NoOpObserver
	cache *LoggingDataSourceCacheObserver
	lua   *LoggingLuaDataSourceObserver
}

func NewLoggingDataSourceObserver(cacheLogger, luaLogger zerolog.Logger) *LoggingDataSourceObserver {
	return &LoggingDataSourceObserver{
		cache: NewLoggingDataSourceCacheObserver(cacheLogger),
		lua:   NewLoggingLuaDataSourceObserver(luaLogger),
	}
}

func (o *LoggingDataSourceObserver) CacheFetchStarted(ctx context.Context, dataSourceName string) (context.Context, datasource.CacheFetchProbe) {
	return o.cache.CacheFetchStarted(ctx, dataSourceName)
}

func (o *LoggingDataSourceObserver) LuaFetchStarted(ctx context.Context, dataSourceName string) (context.Context, datasource.LuaFetchProbe) {
	return o.lua.LuaFetchStarted(ctx, dataSourceName)
}

var _ datasource.DataSourceObserver = (*LoggingDataSourceObserver)(nil)

// LoggingKeysObserver satisfies keys.KeysObserver
// by combining rotation and provider logging observers.
type LoggingKeysObserver struct {
	keys.NoOpObserver
	rotation *LoggingKeyRotationObserver
	provider *LoggingKeyProviderObserver
}

func NewLoggingKeysObserver(rotationLogger, providerLogger zerolog.Logger) *LoggingKeysObserver {
	return &LoggingKeysObserver{
		rotation: NewLoggingKeyRotationObserver(rotationLogger),
		provider: NewLoggingKeyProviderObserver(providerLogger),
	}
}

func (o *LoggingKeysObserver) RotationCheckStarted(ctx context.Context) (context.Context, keys.RotationCheckProbe) {
	return o.rotation.RotationCheckStarted(ctx)
}

func (o *LoggingKeysObserver) KeyProvisionStarted(ctx context.Context) (context.Context, keys.KeyProvisionProbe) {
	return o.provider.KeyProvisionStarted(ctx)
}

var _ keys.KeysObserver = (*LoggingKeysObserver)(nil)

// LoggingServerObserver satisfies server.ServerObserver
// by combining JWKS and lifecycle logging observers.
type LoggingServerObserver struct {
	server.NoOpObserver
	jwks      *LoggingJWKSObserver
	lifecycle *LoggingServerLifecycleObserver
}

func NewLoggingServerObserver(jwksLogger, lifecycleLogger zerolog.Logger) *LoggingServerObserver {
	return &LoggingServerObserver{
		jwks:      NewLoggingJWKSObserver(jwksLogger),
		lifecycle: NewLoggingServerLifecycleObserver(lifecycleLogger),
	}
}

func (o *LoggingServerObserver) CacheRefreshStarted(ctx context.Context) (context.Context, server.CacheRefreshProbe) {
	return o.jwks.CacheRefreshStarted(ctx)
}

func (o *LoggingServerObserver) ServeStarted(ctx context.Context) (context.Context, server.ServeProbe) {
	return o.lifecycle.ServeStarted(ctx)
}

var _ server.ServerObserver = (*LoggingServerObserver)(nil)
