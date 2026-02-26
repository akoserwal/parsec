package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestSlogLogger_NilReturnsNoop(t *testing.T) {
	l := NewFromSlog(nil)
	if _, ok := l.(noopLogger); !ok {
		t.Errorf("NewFromSlog(nil) returned %T, want noopLogger", l)
	}
}

func TestSlogLogger_LevelMapping(t *testing.T) {
	tests := []struct {
		level    Level
		wantSlog string
	}{
		{Debug, "DEBUG"},
		{Info, "INFO"},
		{Warn, "WARN"},
		{Error, "ERROR"},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		l := NewFromSlog(slog.New(handler))

		l.Log(context.Background(), tt.level, "test")

		var record map[string]any
		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatalf("level %s: failed to parse JSON: %v", tt.level, err)
		}
		if got := record["level"]; got != tt.wantSlog {
			t.Errorf("level %s: slog level = %q, want %q", tt.level, got, tt.wantSlog)
		}
	}
}

func TestSlogLogger_AttrsAppearInOutput(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewFromSlog(slog.New(handler))

	l.Log(context.Background(), Info, "hello",
		String("method", "GET"),
		Int("status", 200),
		Bool("ok", true),
	)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if record["msg"] != "hello" {
		t.Errorf("msg = %q, want %q", record["msg"], "hello")
	}
	if record["method"] != "GET" {
		t.Errorf("method = %v, want %q", record["method"], "GET")
	}
	if record["status"] != float64(200) {
		t.Errorf("status = %v, want 200", record["status"])
	}
	if record["ok"] != true {
		t.Errorf("ok = %v, want true", record["ok"])
	}
}

func TestSlogLogger_ErrAttr(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewFromSlog(slog.New(handler))

	l.Log(context.Background(), Error, "failed", Err(errors.New("boom")))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if record["error"] != "boom" {
		t.Errorf("error = %v, want %q", record["error"], "boom")
	}
}

func TestSlogLogger_DurationAttr(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewFromSlog(slog.New(handler))

	l.Log(context.Background(), Info, "req", Duration("elapsed", 150*time.Millisecond))

	if !strings.Contains(buf.String(), "elapsed=") {
		t.Errorf("output %q does not contain elapsed attribute", buf.String())
	}
}

func TestSlogLogger_EnabledShortCircuit(t *testing.T) {
	handler := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	l := NewFromSlog(slog.New(handler))

	if l.Enabled(context.Background(), Debug) {
		t.Error("Enabled(Debug) should be false when handler level is Warn")
	}
	if l.Enabled(context.Background(), Info) {
		t.Error("Enabled(Info) should be false when handler level is Warn")
	}
	if !l.Enabled(context.Background(), Warn) {
		t.Error("Enabled(Warn) should be true when handler level is Warn")
	}
	if !l.Enabled(context.Background(), Error) {
		t.Error("Enabled(Error) should be true when handler level is Warn")
	}
}

func TestSlogLogger_LogRespectsEnabled(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	l := NewFromSlog(slog.New(handler))

	l.Log(context.Background(), Debug, "should not appear")
	l.Log(context.Background(), Info, "should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output for below-threshold levels, got: %s", buf.String())
	}

	l.Log(context.Background(), Warn, "should appear")
	if buf.Len() == 0 {
		t.Error("expected output for Warn level")
	}
}

func TestSlogLogger_With(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewFromSlog(slog.New(handler))

	child := l.With(String("component", "server"))
	child.Log(context.Background(), Info, "started")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if record["component"] != "server" {
		t.Errorf("component = %v, want %q", record["component"], "server")
	}
}

func TestSlogLogger_WithEmptyReturnsOriginal(t *testing.T) {
	handler := slog.NewJSONHandler(&bytes.Buffer{}, nil)
	l := NewFromSlog(slog.New(handler))
	got := l.With()
	if got != l {
		t.Error("With() with no attrs should return the same logger instance")
	}
}

func TestSlogLogger_WithDoesNotMutateParent(t *testing.T) {
	var parentBuf, childBuf bytes.Buffer
	parentHandler := slog.NewJSONHandler(&parentBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	parent := NewFromSlog(slog.New(parentHandler))

	child := parent.With(String("child_key", "child_val"))
	_ = child

	parent.Log(context.Background(), Info, "parent log")

	var record map[string]any
	if err := json.Unmarshal(parentBuf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse parent JSON: %v", err)
	}
	if _, exists := record["child_key"]; exists {
		t.Error("parent log should not contain child_key attribute")
	}

	childHandler := slog.NewJSONHandler(&childBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	child = NewFromSlog(slog.New(childHandler)).With(String("child_key", "child_val"))
	child.Log(context.Background(), Info, "child log")

	if err := json.Unmarshal(childBuf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse child JSON: %v", err)
	}
	if record["child_key"] != "child_val" {
		t.Errorf("child_key = %v, want %q", record["child_key"], "child_val")
	}
}

func TestSlogLogger_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewFromSlog(slog.New(handler))

	grouped := l.WithGroup("request")
	grouped.Log(context.Background(), Info, "handled",
		String("method", "POST"),
		Int("status", 201),
	)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	group, ok := record["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested 'request' group, got record: %v", record)
	}
	if group["method"] != "POST" {
		t.Errorf("request.method = %v, want %q", group["method"], "POST")
	}
	if group["status"] != float64(201) {
		t.Errorf("request.status = %v, want 201", group["status"])
	}
}
