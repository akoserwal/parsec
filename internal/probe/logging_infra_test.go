package probe

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/parsec/internal/trust"
)

func testLogger(buf *bytes.Buffer) zerolog.Logger {
	return zerolog.New(buf).Level(zerolog.DebugLevel)
}

// assertLog verifies the log output contains the expected level, message,
// and every additional field string (e.g. `"key":"value"`).
func assertLog(t *testing.T, out, level, msg string, fields ...string) {
	t.Helper()
	assert.Contains(t, out, fmt.Sprintf(`"level":"%s"`, level))
	assert.Contains(t, out, msg)
	for _, f := range fields {
		assert.Contains(t, out, f)
	}
}

// --- ConfigReload ---

func TestLoggingConfigReloadObserver_ConfigReloadFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingConfigReloadObserver{Logger: testLogger(&buf)}

	obs.ConfigReloadFailed("unmarshal", errors.New("bad yaml"))

	assertLog(t, buf.String(), "error", "config reload failed",
		`"step":"unmarshal"`, `"error":"bad yaml"`)
}

// --- DataSourceCache ---

func TestLoggingDataSourceCacheObserver_DebugEvents(t *testing.T) {
	tests := []struct {
		name string
		call func(*LoggingDataSourceCacheObserver)
		msg  string
	}{
		{"CacheHit", func(o *LoggingDataSourceCacheObserver) { o.CacheHit("ds") }, "cache hit"},
		{"CacheMiss", func(o *LoggingDataSourceCacheObserver) { o.CacheMiss("ds") }, "cache miss"},
		{"CacheExpired", func(o *LoggingDataSourceCacheObserver) { o.CacheExpired("ds") }, "cache entry expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := &LoggingDataSourceCacheObserver{Logger: testLogger(&buf)}
			tt.call(obs)
			assertLog(t, buf.String(), "debug", tt.msg, `"datasource":"ds"`)
		})
	}
}

func TestLoggingDataSourceCacheObserver_FetchFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingDataSourceCacheObserver{Logger: testLogger(&buf)}

	obs.FetchFailed("my_ds", errors.New("timeout"))

	assertLog(t, buf.String(), "warn", "data source fetch failed",
		`"datasource":"my_ds"`, `"error":"timeout"`)
}

// --- KeyRotation ---

func TestLoggingKeyRotationObserver_RotationCheckFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingKeyRotationObserver{Logger: testLogger(&buf)}

	obs.RotationCheckFailed(errors.New("slot locked"))

	assertLog(t, buf.String(), "error", "key rotation check failed",
		`"error":"slot locked"`)
}

func TestLoggingKeyRotationObserver_InfoEvents(t *testing.T) {
	tests := []struct {
		name string
		call func(*LoggingKeyRotationObserver)
		msg  string
		slot string
	}{
		{"RotationCompleted", func(o *LoggingKeyRotationObserver) { o.RotationCompleted("primary") },
			"key rotation completed", "primary"},
		{"RotationSkippedVersionRace", func(o *LoggingKeyRotationObserver) { o.RotationSkippedVersionRace("secondary") },
			"another process completed rotation, skipping", "secondary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := &LoggingKeyRotationObserver{Logger: testLogger(&buf)}
			tt.call(obs)
			assertLog(t, buf.String(), "info", tt.msg,
				fmt.Sprintf(`"slot":"%s"`, tt.slot))
		})
	}
}

func TestLoggingKeyRotationObserver_KeyProviderNotFound(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingKeyRotationObserver{Logger: testLogger(&buf)}

	obs.KeyProviderNotFound("aws_kms", "primary")

	assertLog(t, buf.String(), "warn", "key provider not found, skipping",
		`"provider":"aws_kms"`, `"slot":"primary"`)
}

func TestLoggingKeyRotationObserver_WarningMethods(t *testing.T) {
	tests := []struct {
		name string
		call func(*LoggingKeyRotationObserver)
		msg  string
	}{
		{"KeyHandleFailed", func(o *LoggingKeyRotationObserver) { o.KeyHandleFailed("s1", errors.New("e")) }, "failed to get key handle"},
		{"PublicKeyFailed", func(o *LoggingKeyRotationObserver) { o.PublicKeyFailed("s1", errors.New("e")) }, "failed to get public key"},
		{"ThumbprintFailed", func(o *LoggingKeyRotationObserver) { o.ThumbprintFailed("s1", errors.New("e")) }, "failed to compute thumbprint"},
		{"MetadataFailed", func(o *LoggingKeyRotationObserver) { o.MetadataFailed("s1", errors.New("e")) }, "failed to get key metadata"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := &LoggingKeyRotationObserver{Logger: testLogger(&buf)}
			tt.call(obs)
			assertLog(t, buf.String(), "warn", tt.msg, `"slot":"s1"`)
		})
	}
}

// --- KeyProvider ---

func TestLoggingKeyProviderObserver_OldKeyDeletionFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingKeyProviderObserver{Logger: testLogger(&buf)}

	obs.OldKeyDeletionFailed("key-123", errors.New("access denied"))

	assertLog(t, buf.String(), "warn", "failed to schedule old key for deletion",
		`"key_id":"key-123"`, `"error":"access denied"`)
}

// --- TrustValidation ---

func TestLoggingTrustValidationObserver_ValidatorFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingTrustValidationObserver{Logger: testLogger(&buf)}

	obs.ValidatorFailed("oidc_v1", trust.CredentialTypeJWT, errors.New("expired"))

	assertLog(t, buf.String(), "debug", "validator rejected credential",
		`"validator":"oidc_v1"`, `"credential_type":"jwt"`)
}

func TestLoggingTrustValidationObserver_AllValidatorsFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingTrustValidationObserver{Logger: testLogger(&buf)}

	obs.AllValidatorsFailed(trust.CredentialTypeBearer, 3, errors.New("no match"))

	assertLog(t, buf.String(), "warn", "all validators failed for credential type",
		`"credential_type":"bearer"`, `"attempted":3`)
}

func TestLoggingTrustValidationObserver_ValidatorFiltered(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingTrustValidationObserver{Logger: testLogger(&buf)}

	obs.ValidatorFiltered("v1", "actor-xyz")

	assertLog(t, buf.String(), "debug", "validator filtered out for actor",
		`"validator":"v1"`, `"actor":"actor-xyz"`)
}

func TestLoggingTrustValidationObserver_FilterEvaluationFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := &LoggingTrustValidationObserver{Logger: testLogger(&buf)}

	obs.FilterEvaluationFailed("v2", errors.New("cel error"))

	assertLog(t, buf.String(), "error", "filter evaluation failed",
		`"validator":"v2"`, `"error":"cel error"`)
}

// --- JWKS ---

func TestLoggingJWKSObserver(t *testing.T) {
	tests := []struct {
		name   string
		call   func(*LoggingJWKSObserver)
		msg    string
		fields []string
	}{
		{"InitialCachePopulationFailed",
			func(o *LoggingJWKSObserver) { o.InitialCachePopulationFailed(errors.New("no issuers")) },
			"initial cache population failed, will retry",
			[]string{`"error":"no issuers"`}},
		{"CacheRefreshFailed",
			func(o *LoggingJWKSObserver) { o.CacheRefreshFailed(errors.New("network")) },
			"background cache refresh failed",
			[]string{`"error":"network"`}},
		{"KeyConversionFailed",
			func(o *LoggingJWKSObserver) { o.KeyConversionFailed("kid-1", errors.New("unsupported alg")) },
			"skipping key: conversion failed",
			[]string{`"key_id":"kid-1"`, `"error":"unsupported alg"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := &LoggingJWKSObserver{Logger: testLogger(&buf)}
			tt.call(obs)
			assertLog(t, buf.String(), "warn", tt.msg, tt.fields...)
		})
	}
}

// --- ServerLifecycle ---

func TestLoggingServerLifecycleObserver(t *testing.T) {
	tests := []struct {
		name string
		call func(*LoggingServerLifecycleObserver)
		msg  string
	}{
		{"GRPCServeFailed", func(o *LoggingServerLifecycleObserver) { o.GRPCServeFailed(errors.New("bind error")) }, "gRPC server error"},
		{"HTTPServeFailed", func(o *LoggingServerLifecycleObserver) { o.HTTPServeFailed(errors.New("port in use")) }, "HTTP server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := &LoggingServerLifecycleObserver{Logger: testLogger(&buf)}
			tt.call(obs)
			assertLog(t, buf.String(), "error", tt.msg)
		})
	}
}
