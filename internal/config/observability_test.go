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
