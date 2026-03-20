package datasource

import "context"

// DataSourceCacheObserver creates request-scoped probes for cache lifecycle events.
type DataSourceCacheObserver interface {
	DataSourceCacheProbe(ctx context.Context, dataSourceName string) DataSourceCacheProbe
}

// DataSourceCacheProbe logs or records cache events for a single data source
// without repeating the data source name on every call.
type DataSourceCacheProbe interface {
	CacheHit()
	CacheMiss()
	CacheExpired()
	FetchFailed(err error)
}

// LuaDataSourceObserver creates probes for Lua script execution events.
type LuaDataSourceObserver interface {
	LuaDataSourceProbe(ctx context.Context, dataSourceName string) LuaDataSourceProbe
}

// LuaDataSourceProbe receives Lua-specific execution events for one Fetch call.
type LuaDataSourceProbe interface {
	ScriptLoadFailed(err error)
	ScriptExecutionFailed(err error)
	InvalidReturnType(got string)
	FetchCompleted()
}

// --- noop implementations ---

type noopDataSourceCacheProbe struct{}

func (noopDataSourceCacheProbe) CacheHit()         {}
func (noopDataSourceCacheProbe) CacheMiss()        {}
func (noopDataSourceCacheProbe) CacheExpired()     {}
func (noopDataSourceCacheProbe) FetchFailed(error) {}

type noopLuaDataSourceProbe struct{}

func (noopLuaDataSourceProbe) ScriptLoadFailed(error)      {}
func (noopLuaDataSourceProbe) ScriptExecutionFailed(error) {}
func (noopLuaDataSourceProbe) InvalidReturnType(string)    {}
func (noopLuaDataSourceProbe) FetchCompleted()             {}

// NoopObserver satisfies both datasource observer interfaces with empty probes.
type NoopObserver struct{}

func (NoopObserver) DataSourceCacheProbe(context.Context, string) DataSourceCacheProbe {
	return noopDataSourceCacheProbe{}
}

func (NoopObserver) LuaDataSourceProbe(context.Context, string) LuaDataSourceProbe {
	return noopLuaDataSourceProbe{}
}
