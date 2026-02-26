package logging

// Level represents log severity. Values use sequential iota to avoid
// leaking any backend's numeric model. Mapping to concrete levels
// (e.g. slog.LevelDebug = -4) happens only inside the adapter.
type Level uint8

const (
	Debug Level = iota
	Info
	Warn
	Error
)

// String returns the human-readable name of the level.
func (l Level) String() string {
	switch l {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
