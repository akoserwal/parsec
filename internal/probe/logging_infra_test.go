package probe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/parsec/internal/datasource"
	"github.com/project-kessel/parsec/internal/keys"
	"github.com/project-kessel/parsec/internal/server"
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

// --- DataSourceCache ---

func TestLoggingDataSourceCacheObserver_DebugEvents(t *testing.T) {
	tests := []struct {
		name string
		call func(datasource.CacheFetchProbe)
		msg  string
	}{
		{"CacheHit", func(p datasource.CacheFetchProbe) { p.CacheHit() }, "cache hit"},
		{"CacheMiss", func(p datasource.CacheFetchProbe) { p.CacheMiss() }, "cache miss"},
		{"CacheExpired", func(p datasource.CacheFetchProbe) { p.CacheExpired() }, "cache entry expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := NewLoggingDataSourceCacheObserver(testLogger(&buf))
			_, p := obs.CacheFetchStarted(context.Background(), "ds")
			tt.call(p)
			assertLog(t, buf.String(), "debug", tt.msg, `"datasource":"ds"`)
		})
	}
}

func TestLoggingDataSourceCacheObserver_FetchFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingDataSourceCacheObserver(testLogger(&buf))
	_, p := obs.CacheFetchStarted(context.Background(), "my_ds")

	p.FetchFailed(errors.New("timeout"))

	assertLog(t, buf.String(), "warn", "data source fetch failed",
		`"datasource":"my_ds"`, `"error":"timeout"`)
}

// --- LuaDataSource ---

func TestLoggingLuaDataSourceObserver_ErrorEvents(t *testing.T) {
	tests := []struct {
		name  string
		call  func(datasource.LuaFetchProbe)
		level string
		msg   string
	}{
		{"ScriptLoadFailed", func(p datasource.LuaFetchProbe) { p.ScriptLoadFailed(errors.New("syntax error")) }, "error", "lua script load failed"},
		{"ScriptExecutionFailed", func(p datasource.LuaFetchProbe) { p.ScriptExecutionFailed(errors.New("nil ref")) }, "error", "lua script execution failed"},
		{"InvalidReturnType", func(p datasource.LuaFetchProbe) { p.InvalidReturnType("number") }, "error", "lua fetch returned invalid type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := NewLoggingLuaDataSourceObserver(testLogger(&buf))
			_, p := obs.LuaFetchStarted(context.Background(), "my_lua_ds")
			tt.call(p)
			assertLog(t, buf.String(), tt.level, tt.msg, `"datasource":"my_lua_ds"`)
		})
	}
}

func TestLoggingLuaDataSourceObserver_FetchCompleted(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingLuaDataSourceObserver(testLogger(&buf))
	_, p := obs.LuaFetchStarted(context.Background(), "my_lua_ds")
	p.FetchCompleted()
	assertLog(t, buf.String(), "debug", "lua fetch completed", `"datasource":"my_lua_ds"`)
}

// --- KeyRotation ---

func TestLoggingKeyRotationObserver_RotationCheckFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingKeyRotationObserver(testLogger(&buf))
	_, p := obs.RotationCheckStarted(context.Background())

	p.RotationCheckFailed(errors.New("slot locked"))

	assertLog(t, buf.String(), "error", "key rotation check failed",
		`"error":"slot locked"`)
}

func TestLoggingKeyRotationObserver_InfoEvents(t *testing.T) {
	tests := []struct {
		name string
		call func(keys.RotationCheckProbe)
		msg  string
		slot string
	}{
		{"RotationCompleted", func(p keys.RotationCheckProbe) { p.RotationCompleted("primary") },
			"key rotation completed", "primary"},
		{"RotationSkippedVersionRace", func(p keys.RotationCheckProbe) { p.RotationSkippedVersionRace("secondary") },
			"another process completed rotation, skipping", "secondary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := NewLoggingKeyRotationObserver(testLogger(&buf))
			_, p := obs.RotationCheckStarted(context.Background())
			tt.call(p)
			assertLog(t, buf.String(), "info", tt.msg,
				fmt.Sprintf(`"slot":"%s"`, tt.slot))
		})
	}
}

func TestLoggingKeyRotationObserver_KeyProviderNotFound(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingKeyRotationObserver(testLogger(&buf))
	_, p := obs.RotationCheckStarted(context.Background())

	p.KeyProviderNotFound("aws_kms", "primary")

	assertLog(t, buf.String(), "warn", "key provider not found, skipping",
		`"provider":"aws_kms"`, `"slot":"primary"`)
}

func TestLoggingKeyRotationObserver_WarningMethods(t *testing.T) {
	tests := []struct {
		name string
		call func(keys.RotationCheckProbe)
		msg  string
	}{
		{"KeyHandleFailed", func(p keys.RotationCheckProbe) { p.KeyHandleFailed("s1", errors.New("e")) }, "failed to get key handle"},
		{"PublicKeyFailed", func(p keys.RotationCheckProbe) { p.PublicKeyFailed("s1", errors.New("e")) }, "failed to get public key"},
		{"ThumbprintFailed", func(p keys.RotationCheckProbe) { p.ThumbprintFailed("s1", errors.New("e")) }, "failed to compute thumbprint"},
		{"MetadataFailed", func(p keys.RotationCheckProbe) { p.MetadataFailed("s1", errors.New("e")) }, "failed to get key metadata"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := NewLoggingKeyRotationObserver(testLogger(&buf))
			_, p := obs.RotationCheckStarted(context.Background())
			tt.call(p)
			assertLog(t, buf.String(), "warn", tt.msg, `"slot":"s1"`)
		})
	}
}

// --- KeyProvider ---

func TestLoggingKeyProviderObserver_OldKeyDeletionFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingKeyProviderObserver(testLogger(&buf))
	_, p := obs.KeyProvisionStarted(context.Background())

	p.OldKeyDeletionFailed("key-123", errors.New("access denied"))

	assertLog(t, buf.String(), "warn", "failed to schedule old key for deletion",
		`"key_id":"key-123"`, `"error":"access denied"`)
}

// --- TrustValidation ---

func TestLoggingTrustValidationObserver_ValidatorFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingTrustValidationObserver(testLogger(&buf))
	_, p := obs.ValidationStarted(context.Background())

	p.ValidatorFailed("oidc_v1", trust.CredentialTypeJWT, errors.New("expired"))

	assertLog(t, buf.String(), "debug", "validator rejected credential",
		`"validator":"oidc_v1"`, `"credential_type":"jwt"`)
}

func TestLoggingTrustValidationObserver_AllValidatorsFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingTrustValidationObserver(testLogger(&buf))
	_, p := obs.ValidationStarted(context.Background())

	p.AllValidatorsFailed(trust.CredentialTypeBearer, 3, errors.New("no match"))

	assertLog(t, buf.String(), "warn", "all validators failed for credential type",
		`"credential_type":"bearer"`, `"attempted":3`)
}

func TestLoggingTrustValidationObserver_ValidatorFiltered(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingTrustValidationObserver(testLogger(&buf))
	_, p := obs.ValidationStarted(context.Background())

	p.ValidatorFiltered("v1", "actor-xyz")

	assertLog(t, buf.String(), "debug", "validator filtered out for actor",
		`"validator":"v1"`, `"actor":"actor-xyz"`)
}

func TestLoggingTrustValidationObserver_FilterEvaluationFailed(t *testing.T) {
	var buf bytes.Buffer
	obs := NewLoggingTrustValidationObserver(testLogger(&buf))
	_, p := obs.ValidationStarted(context.Background())

	p.FilterEvaluationFailed("v2", errors.New("cel error"))

	assertLog(t, buf.String(), "error", "filter evaluation failed",
		`"validator":"v2"`, `"error":"cel error"`)
}

// --- JWKS ---

func TestLoggingJWKSObserver(t *testing.T) {
	tests := []struct {
		name   string
		call   func(server.CacheRefreshProbe)
		msg    string
		fields []string
	}{
		{"InitialCachePopulationFailed",
			func(p server.CacheRefreshProbe) { p.InitialCachePopulationFailed(errors.New("no issuers")) },
			"initial cache population failed, will retry",
			[]string{`"error":"no issuers"`}},
		{"CacheRefreshFailed",
			func(p server.CacheRefreshProbe) { p.CacheRefreshFailed(errors.New("network")) },
			"background cache refresh failed",
			[]string{`"error":"network"`}},
		{"KeyConversionFailed",
			func(p server.CacheRefreshProbe) { p.KeyConversionFailed("kid-1", errors.New("unsupported alg")) },
			"skipping key: conversion failed",
			[]string{`"key_id":"kid-1"`, `"error":"unsupported alg"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := NewLoggingJWKSObserver(testLogger(&buf))
			_, p := obs.CacheRefreshStarted(context.Background())
			tt.call(p)
			assertLog(t, buf.String(), "warn", tt.msg, tt.fields...)
		})
	}
}

// --- ServerLifecycle ---

func TestLoggingServerLifecycleObserver(t *testing.T) {
	tests := []struct {
		name string
		call func(server.ServeProbe)
		msg  string
	}{
		{"GRPCServeFailed", func(p server.ServeProbe) { p.GRPCServeFailed(errors.New("bind error")) }, "gRPC server error"},
		{"HTTPServeFailed", func(p server.ServeProbe) { p.HTTPServeFailed(errors.New("port in use")) }, "HTTP server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			obs := NewLoggingServerLifecycleObserver(testLogger(&buf))
			_, p := obs.ServeStarted(context.Background())
			tt.call(p)
			assertLog(t, buf.String(), "error", tt.msg)
		})
	}
}
