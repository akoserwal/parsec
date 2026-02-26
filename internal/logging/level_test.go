package logging

import "testing"

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{Debug, "DEBUG"},
		{Info, "INFO"},
		{Warn, "WARN"},
		{Error, "ERROR"},
		{Level(255), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLevel_Ordering(t *testing.T) {
	if !(Debug < Info && Info < Warn && Warn < Error) {
		t.Fatal("levels must be ordered Debug < Info < Warn < Error")
	}
}
