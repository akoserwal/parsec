package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

func TestSlogLogger_AttrsInOutput(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := NewFromSlog(slog.New(handler))

	l.Log(context.Background(), Info, "hello",
		String("method", "GET"),
		Int("status", 200),
		Bool("ok", true),
		Duration("elapsed", 150*time.Millisecond),
		Err(errors.New("boom")),
	)

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	exact := map[string]any{
		"msg":    "hello",
		"method": "GET",
		"status": float64(200),
		"ok":     true,
		"error":  "boom",
	}
	for key, want := range exact {
		if record[key] != want {
			t.Errorf("%s = %v (%T), want %v (%T)", key, record[key], record[key], want, want)
		}
	}
	if _, exists := record["elapsed"]; !exists {
		t.Error("elapsed attribute missing from output")
	}
}

func TestSlogLogger_LevelFiltering(t *testing.T) {
	t.Run("Enabled", func(t *testing.T) {
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
	})

	t.Run("LogSuppressesOutput", func(t *testing.T) {
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
	})
}

func TestSlogLogger_WithImmutability(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	parent := NewFromSlog(slog.New(handler))

	child := parent.With(String("child_key", "child_val"))

	// Parent must not carry the child's attribute
	parent.Log(context.Background(), Info, "parent log")
	parentLine, _, _ := bytes.Cut(buf.Bytes(), []byte("\n"))

	var parentRecord map[string]any
	if err := json.Unmarshal(parentLine, &parentRecord); err != nil {
		t.Fatalf("failed to parse parent JSON: %v", err)
	}
	if _, exists := parentRecord["child_key"]; exists {
		t.Error("parent log should not contain child_key attribute")
	}

	// Child derived from the same parent must carry its attribute
	child.Log(context.Background(), Info, "child log")
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	childLine := lines[len(lines)-1]

	var childRecord map[string]any
	if err := json.Unmarshal(childLine, &childRecord); err != nil {
		t.Fatalf("failed to parse child JSON: %v", err)
	}
	if childRecord["child_key"] != "child_val" {
		t.Errorf("child_key = %v, want %q", childRecord["child_key"], "child_val")
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
