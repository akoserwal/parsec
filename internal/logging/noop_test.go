package logging

import (
	"context"
	"testing"
)

func TestNoopLogger_ImplementsInterface(t *testing.T) {
	var _ Logger = NewNoop()
}

func TestNoopLogger_EnabledReturnsFalse(t *testing.T) {
	l := NewNoop()
	for _, level := range []Level{Debug, Info, Warn, Error} {
		if l.Enabled(context.Background(), level) {
			t.Errorf("NewNoop().Enabled(ctx, %s) = true, want false", level)
		}
	}
}

func TestNoopLogger_WithReturnsSelf(t *testing.T) {
	l := NewNoop()
	got := l.With(String("k", "v"))
	if got != l {
		t.Error("With() should return the same noop instance")
	}
}

func TestNoopLogger_WithGroupReturnsSelf(t *testing.T) {
	l := NewNoop()
	got := l.WithGroup("g")
	if got != l {
		t.Error("WithGroup() should return the same noop instance")
	}
}

func TestNoopLogger_LogDoesNotPanic(t *testing.T) {
	l := NewNoop()
	l.Log(context.Background(), Error, "should not panic", String("k", "v"))
}
