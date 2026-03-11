package config_test

import (
	"github.com/project-kessel/parsec/internal/config"
	"github.com/project-kessel/parsec/internal/probe"
)

// Compile-time check that LoggingConfigReloadObserver satisfies
// config.ConfigReloadObserver. The production code can't assert this
// directly because probe already imports config (circular import).
var _ config.ConfigReloadObserver = (*probe.LoggingConfigReloadObserver)(nil)
