package logging

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestAttrConstructors(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		a := String("key", "val")
		assertAttr(t, a, "key", "val")
	})

	t.Run("Int", func(t *testing.T) {
		a := Int("count", 42)
		assertAttr(t, a, "count", 42)
	})

	t.Run("Int64", func(t *testing.T) {
		a := Int64("big", 1<<40)
		assertAttr(t, a, "big", int64(1<<40))
	})

	t.Run("Bool", func(t *testing.T) {
		a := Bool("ok", true)
		assertAttr(t, a, "ok", true)
	})

	t.Run("Duration", func(t *testing.T) {
		d := 5 * time.Second
		a := Duration("elapsed", d)
		assertAttr(t, a, "elapsed", d)
	})

	t.Run("Time", func(t *testing.T) {
		now := time.Now()
		a := Time("ts", now)
		assertAttr(t, a, "ts", now)
	})

	t.Run("Any", func(t *testing.T) {
		v := []string{"a", "b"}
		a := Any("list", v)
		if a.Key() != "list" {
			t.Errorf("Key = %q, want %q", a.Key(), "list")
		}
		if !reflect.DeepEqual(a.Value(), v) {
			t.Errorf("Value = %v, want %v", a.Value(), v)
		}
	})

	t.Run("Err", func(t *testing.T) {
		err := errors.New("boom")
		a := Err(err)
		if a.Key() != "error" {
			t.Errorf("Err().Key = %q, want %q", a.Key(), "error")
		}
		if a.Value() != err {
			t.Errorf("Err().Value = %v, want %v", a.Value(), err)
		}
	})

	t.Run("Err_nil", func(t *testing.T) {
		a := Err(nil)
		if a.Key() != "error" {
			t.Errorf("Err(nil).Key = %q, want %q", a.Key(), "error")
		}
		if a.Value() != nil {
			t.Errorf("Err(nil).Value = %v, want nil", a.Value())
		}
	})
}

func assertAttr(t *testing.T, a Attr, wantKey string, wantValue any) {
	t.Helper()
	if a.Key() != wantKey {
		t.Errorf("Key = %q, want %q", a.Key(), wantKey)
	}
	if a.Value() != wantValue {
		t.Errorf("Value = %v (%T), want %v (%T)", a.Value(), a.Value(), wantValue, wantValue)
	}
}
