package config

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/project-kessel/parsec/internal/datasource"
	luaservices "github.com/project-kessel/parsec/internal/lua"
	"github.com/project-kessel/parsec/internal/observer"
	"github.com/project-kessel/parsec/internal/service"
)

// NewDataSourceRegistry creates a data source registry from configuration.
// The observer provides cache lifecycle events for data sources that use caching.
func NewDataSourceRegistry(cfg []DataSourceConfig, transport http.RoundTripper, obs observer.Observer) (*service.DataSourceRegistry, error) {
	registry := service.NewDataSourceRegistry()

	for _, dsCfg := range cfg {
		ds, err := newDataSource(dsCfg, transport, obs)
		if err != nil {
			return nil, fmt.Errorf("failed to create data source %s: %w", dsCfg.Name, err)
		}
		registry.Register(ds)
	}

	return registry, nil
}

func newDataSource(cfg DataSourceConfig, transport http.RoundTripper, obs observer.Observer) (service.DataSource, error) {
	switch cfg.Type {
	case "lua":
		return newLuaDataSource(cfg, transport, obs)
	default:
		return nil, fmt.Errorf("unknown data source type: %s (supported: lua)", cfg.Type)
	}
}

func newLuaDataSource(cfg DataSourceConfig, transport http.RoundTripper, obs observer.Observer) (service.DataSource, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("data source name is required")
	}

	// Get script content (either from file or inline)
	script := cfg.Script
	if cfg.ScriptFile != "" {
		content, err := os.ReadFile(cfg.ScriptFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file %s: %w", cfg.ScriptFile, err)
		}
		script = string(content)
	}

	if script == "" {
		return nil, fmt.Errorf("lua data source requires either script or script_file")
	}

	// Create config source from map
	var configSource luaservices.ConfigSource
	if cfg.Config != nil {
		configSource = luaservices.NewMapConfigSource(cfg.Config)
	}

	// Build HTTP config
	var httpConfig *luaservices.HTTPServiceConfig
	if cfg.HTTPConfig != nil {
		httpCfg, err := buildHTTPConfig(cfg.HTTPConfig, transport)
		if err != nil {
			return nil, fmt.Errorf("failed to build HTTP config: %w", err)
		}
		httpConfig = httpCfg
	}

	// Create base Lua data source
	luaDSConfig := datasource.LuaDataSourceConfig{
		Name:         cfg.Name,
		Script:       script,
		ConfigSource: configSource,
		HTTPConfig:   httpConfig,
		Observer:     obs,
	}

	baseDS, err := datasource.NewLuaDataSource(luaDSConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create lua data source: %w", err)
	}

	if cfg.Caching != nil {
		return wrapWithCaching(baseDS, *cfg.Caching, obs)
	}

	return baseDS, nil
}

// buildHTTPConfig creates an HTTPServiceConfig from the config structure
func buildHTTPConfig(cfg *HTTPConfig, transport http.RoundTripper) (*luaservices.HTTPServiceConfig, error) {
	httpServiceCfg := &luaservices.HTTPServiceConfig{}

	// Parse timeout
	if cfg.Timeout != "" {
		duration, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid http timeout: %w", err)
		}
		httpServiceCfg.Timeout = duration
	} else {
		httpServiceCfg.Timeout = 30 * time.Second // default
	}

	// Use the provided HTTP transport (from top-level config)
	if transport != nil {
		httpServiceCfg.Transport = transport
	}

	return httpServiceCfg, nil
}

// wrapWithCaching wraps a data source with the configured caching layer.
// This is the coupling point where the central observer is narrowed to
// the cache-specific CacheObserver sub-interface.
func wrapWithCaching(ds service.DataSource, cfg CachingConfig, obs observer.Observer) (service.DataSource, error) {
	switch cfg.Type {
	case "in_memory":
		return datasource.NewInMemoryCachingDataSource(ds, obs), nil

	case "distributed":
		groupName := cfg.GroupName
		if groupName == "" {
			groupName = ds.Name() + "-cache"
		}

		cacheSize := cfg.CacheSize
		if cacheSize == 0 {
			cacheSize = 64 << 20 // 64 MB default
		}

		cachingCfg := datasource.DistributedCachingConfig{
			GroupName:      groupName,
			CacheSizeBytes: cacheSize,
		}

		return datasource.NewDistributedCachingDataSource(ds, cachingCfg), nil

	case "none", "":
		// No caching
		return ds, nil

	default:
		return nil, fmt.Errorf("unknown caching type: %s (supported: in_memory, distributed, none)", cfg.Type)
	}
}
