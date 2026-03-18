package config

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }

// jsonLogCtx builds a LoggerContext that writes JSON to buf.
// Writer is set to the raw buf so format overrides work correctly.
func jsonLogCtx(buf *bytes.Buffer) LoggerContext {
	return LoggerContext{
		Logger: zerolog.New(buf).With().Timestamp().Logger().Level(zerolog.InfoLevel),
		Writer: buf,
	}
}

func TestEventLogger_NilConfig_InheritsBase(t *testing.T) {
	var buf bytes.Buffer
	logCtx := jsonLogCtx(&buf)

	logger := EventLogger(logCtx, "test_event", nil)
	logger.Info().Msg("hello")

	assert.Contains(t, buf.String(), `"event":"test_event"`)
	assert.Contains(t, buf.String(), `"message":"hello"`)
}

func TestEventLogger_LevelAndEnabled(t *testing.T) {
	tests := []struct {
		name      string
		baseLevel zerolog.Level
		eventCfg  *EventLoggingConfig
		emitLevel zerolog.Level
		wantEmpty bool
	}{
		{
			name:      "nil config inherits base level",
			baseLevel: zerolog.InfoLevel,
			eventCfg:  nil,
			emitLevel: zerolog.DebugLevel,
			wantEmpty: true,
		},
		{
			name:      "level override widens to debug",
			baseLevel: zerolog.InfoLevel,
			eventCfg:  &EventLoggingConfig{LogLevel: "debug"},
			emitLevel: zerolog.DebugLevel,
			wantEmpty: false,
		},
		{
			name:      "level override restricts to error",
			baseLevel: zerolog.DebugLevel,
			eventCfg:  &EventLoggingConfig{LogLevel: "error"},
			emitLevel: zerolog.InfoLevel,
			wantEmpty: true,
		},
		{
			name:      "enabled false suppresses all",
			baseLevel: zerolog.InfoLevel,
			eventCfg:  &EventLoggingConfig{Enabled: boolPtr(false)},
			emitLevel: zerolog.ErrorLevel,
			wantEmpty: true,
		},
		{
			name:      "enabled true no suppression",
			baseLevel: zerolog.InfoLevel,
			eventCfg:  &EventLoggingConfig{Enabled: boolPtr(true)},
			emitLevel: zerolog.InfoLevel,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logCtx := LoggerContext{
				Logger: zerolog.New(&buf).With().Timestamp().Logger().Level(tt.baseLevel),
				Writer: &buf,
			}
			logger := EventLogger(logCtx, "evt", tt.eventCfg)
			logger.WithLevel(tt.emitLevel).Msg("test msg")

			if tt.wantEmpty {
				assert.Empty(t, buf.String())
			} else {
				assert.Contains(t, buf.String(), "test msg")
			}
		})
	}
}

func TestEventLogger_FormatOverride_JSONToText(t *testing.T) {
	var buf bytes.Buffer
	logCtx := jsonLogCtx(&buf)

	logger := EventLogger(logCtx, "text_event", &EventLoggingConfig{
		LogFormat: "text",
	})
	logger.Info().Msg("text output")

	output := buf.String()
	require.NotEmpty(t, output)
	assert.False(t, json.Valid([]byte(output)),
		"output should NOT be valid JSON when format overridden to text; got: %s", output)
	assert.Contains(t, output, "text output")
}

func TestEventLogger_FormatOverride_TextToJSON(t *testing.T) {
	var rawBuf bytes.Buffer
	textWriter := zerolog.ConsoleWriter{Out: &rawBuf, NoColor: true}
	logCtx := LoggerContext{
		Logger: zerolog.New(textWriter).With().Timestamp().Logger().Level(zerolog.InfoLevel),
		Writer: &rawBuf,
	}

	logger := EventLogger(logCtx, "json_event", &EventLoggingConfig{
		LogFormat: "json",
	})
	logger.Info().Msg("json output")

	output := rawBuf.String()
	require.NotEmpty(t, output)
	assert.True(t, json.Valid([]byte(output)),
		"output should be valid JSON when format overridden to json; got: %s", output)
}

func TestEventLogger_FormatAndLevel_Combined(t *testing.T) {
	var buf bytes.Buffer
	logCtx := jsonLogCtx(&buf)

	logger := EventLogger(logCtx, "combo", &EventLoggingConfig{
		LogFormat: "text",
		LogLevel:  "debug",
	})

	logger.Debug().Msg("combo debug")
	output := buf.String()

	require.NotEmpty(t, output)
	assert.Contains(t, output, "combo debug")
	assert.False(t, json.Valid([]byte(output)),
		"should be text format, not JSON; got: %s", output)
}

func TestDeriveLoggerContext(t *testing.T) {
	t.Run("child level override applies", func(t *testing.T) {
		var buf bytes.Buffer
		parent := LoggerContext{
			Logger: zerolog.New(&buf).With().Timestamp().Logger().Level(zerolog.InfoLevel),
			Writer: &buf,
		}
		child := deriveLoggerContext(parent, &ObservabilityConfig{LogLevel: "debug"})

		child.Logger.Debug().Msg("child debug")
		assert.Contains(t, buf.String(), "child debug",
			"child log_level=debug should widen the parent's info level")
	})

	t.Run("child format override applies", func(t *testing.T) {
		var buf bytes.Buffer
		parent := LoggerContext{
			Logger: zerolog.New(&buf).With().Timestamp().Logger().Level(zerolog.InfoLevel),
			Writer: &buf,
		}
		child := deriveLoggerContext(parent, &ObservabilityConfig{LogFormat: "text"})

		child.Logger.Info().Msg("text child")
		output := buf.String()
		require.NotEmpty(t, output)
		assert.False(t, json.Valid([]byte(output)),
			"child log_format=text should override parent JSON; got: %s", output)
	})

	t.Run("no overrides returns parent as-is", func(t *testing.T) {
		var buf bytes.Buffer
		parent := LoggerContext{
			Logger: zerolog.New(&buf).With().Timestamp().Logger().Level(zerolog.WarnLevel),
			Writer: &buf,
		}
		child := deriveLoggerContext(parent, &ObservabilityConfig{})

		child.Logger.Info().Msg("should not appear")
		assert.Empty(t, buf.String(), "child with no overrides should inherit parent warn level")
	})

	t.Run("shares parent raw sink", func(t *testing.T) {
		var buf bytes.Buffer
		parent := LoggerContext{
			Logger: zerolog.New(&buf).With().Timestamp().Logger().Level(zerolog.InfoLevel),
			Writer: &buf,
		}
		child := deriveLoggerContext(parent, &ObservabilityConfig{LogLevel: "debug"})

		assert.Equal(t, parent.Writer, child.Writer,
			"child must share the parent's raw sink")
	})
}

func TestValidateObservabilityConfig_Type(t *testing.T) {
	valid := []string{"", "logging", "noop", "metrics", "composite"}
	for _, typ := range valid {
		t.Run("valid_"+typ, func(t *testing.T) {
			cfg := &ObservabilityConfig{Type: typ}
			if typ == "metrics" {
				cfg.Metrics = &MetricsConfig{Enabled: true}
			}
			err := ValidateObservabilityConfig(cfg)
			assert.NoError(t, err)
		})
	}

	invalid := []string{"metricss", "log", "UNKNOWN", "tracing"}
	for _, typ := range invalid {
		t.Run("invalid_"+typ, func(t *testing.T) {
			err := ValidateObservabilityConfig(&ObservabilityConfig{Type: typ})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid observability.type")
			assert.Contains(t, err.Error(), typ)
		})
	}
}

func TestValidateObservabilityConfig_MetricsMismatch(t *testing.T) {
	t.Run("metrics type without metrics config", func(t *testing.T) {
		err := ValidateObservabilityConfig(&ObservabilityConfig{Type: "metrics"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metrics.enabled is not true")
	})

	t.Run("metrics type with metrics disabled", func(t *testing.T) {
		err := ValidateObservabilityConfig(&ObservabilityConfig{
			Type:    "metrics",
			Metrics: &MetricsConfig{Enabled: false},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metrics.enabled is not true")
	})

	t.Run("metrics type with metrics enabled passes", func(t *testing.T) {
		err := ValidateObservabilityConfig(&ObservabilityConfig{
			Type:    "metrics",
			Metrics: &MetricsConfig{Enabled: true},
		})
		assert.NoError(t, err)
	})
}

func TestValidateObservabilityConfig_CompositeMetricsMismatch(t *testing.T) {
	t.Run("composite with metrics sub-observer validates recursively", func(t *testing.T) {
		err := ValidateObservabilityConfig(&ObservabilityConfig{
			Type: "composite",
			Observers: []ObservabilityConfig{
				{Type: "logging"},
				{Type: "metrics"},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metrics.enabled is not true")
	})

	t.Run("composite with valid metrics sub-observer passes", func(t *testing.T) {
		err := ValidateObservabilityConfig(&ObservabilityConfig{
			Type: "composite",
			Observers: []ObservabilityConfig{
				{Type: "logging"},
				{Type: "metrics", Metrics: &MetricsConfig{Enabled: true}},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("composite without metrics sub-observer passes", func(t *testing.T) {
		err := ValidateObservabilityConfig(&ObservabilityConfig{
			Type: "composite",
			Observers: []ObservabilityConfig{
				{Type: "logging"},
				{Type: "noop"},
			},
		})
		assert.NoError(t, err)
	})
}

func TestNewObserver_MetricsTypeReturnsError(t *testing.T) {
	t.Run("type metrics returns clear error", func(t *testing.T) {
		_, err := NewObserver(&ObservabilityConfig{Type: "metrics"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires metrics dependencies")
		assert.Contains(t, err.Error(), "NewObserverWithLogger")
	})

	t.Run("composite with metrics sub-observer returns clear error", func(t *testing.T) {
		_, err := NewObserver(&ObservabilityConfig{
			Type: "composite",
			Observers: []ObservabilityConfig{
				{Type: "logging"},
				{Type: "metrics"},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires metrics dependencies")
	})

	t.Run("type logging works without deps", func(t *testing.T) {
		obs, err := NewObserver(&ObservabilityConfig{Type: "logging"})
		assert.NoError(t, err)
		assert.NotNil(t, obs)
	})

	t.Run("type noop works without deps", func(t *testing.T) {
		obs, err := NewObserver(&ObservabilityConfig{Type: "noop"})
		assert.NoError(t, err)
		assert.NotNil(t, obs)
	})

	t.Run("nil config works without deps", func(t *testing.T) {
		obs, err := NewObserver(nil)
		assert.NoError(t, err)
		assert.NotNil(t, obs)
	})
}
