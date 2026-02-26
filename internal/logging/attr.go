package logging

import "time"

// Attr is a key-value pair carried by a log record.
//
// Fields are unexported to enforce construction through typed constructors.
// The concrete adapter (e.g. slog) inspects the runtime type to pick an
// efficient encoding path — slog.Any does an internal type-switch for
// string, int64, float64, bool, time.Time, time.Duration, and error,
// so common types avoid reflection.
type Attr struct {
	key   string
	value any
}

// Key returns the attribute's key.
func (a Attr) Key() string { return a.key }

// Value returns the attribute's value.
func (a Attr) Value() any { return a.value }

// String returns an Attr for a string value.
func String(key, value string) Attr { return Attr{key: key, value: value} }

// Int returns an Attr for an int value.
func Int(key string, value int) Attr { return Attr{key: key, value: value} }

// Int64 returns an Attr for an int64 value.
func Int64(key string, value int64) Attr { return Attr{key: key, value: value} }

// Bool returns an Attr for a bool value.
func Bool(key string, value bool) Attr { return Attr{key: key, value: value} }

// Duration returns an Attr for a time.Duration value.
func Duration(key string, value time.Duration) Attr { return Attr{key: key, value: value} }

// Time returns an Attr for a time.Time value.
func Time(key string, value time.Time) Attr { return Attr{key: key, value: value} }

// Any returns an Attr for an arbitrary value. Prefer the typed constructors
// when the type is known at the call site.
func Any(key string, value any) Attr { return Attr{key: key, value: value} }

// Err returns an Attr with key "error" for the given error.
// This standardises the error key across the entire codebase.
func Err(err error) Attr { return Attr{key: "error", value: err} }
