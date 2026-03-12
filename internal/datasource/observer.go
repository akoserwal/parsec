package datasource

// DataSourceCacheObserver receives cache lifecycle events from InMemoryCachingDataSource.
type DataSourceCacheObserver interface {
	// CacheHit is called when a fetch is served from cache.
	CacheHit(dataSourceName string)

	// CacheMiss is called when a cache miss triggers a fetch from the underlying source.
	CacheMiss(dataSourceName string)

	// CacheExpired is called when a cache entry is found but has expired.
	CacheExpired(dataSourceName string)

	// FetchFailed is called when the underlying data source fetch fails on a cache miss.
	FetchFailed(dataSourceName string, err error)
}

// NoopObserver satisfies DataSourceCacheObserver with empty methods.
// Useful in tests that don't care about observer events.
type NoopObserver struct{}

func (NoopObserver) CacheHit(string)           {}
func (NoopObserver) CacheMiss(string)          {}
func (NoopObserver) CacheExpired(string)       {}
func (NoopObserver) FetchFailed(string, error) {}
