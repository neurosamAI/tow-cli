package logger

import (
	"bytes"
	"testing"
)

func TestSetLevel(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()

	tests := []struct {
		name  string
		level Level
	}{
		{"debug", DebugLevel},
		{"info", InfoLevel},
		{"warn", WarnLevel},
		{"error", ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.level)
			if currentLevel != tt.level {
				t.Errorf("expected level %d, got %d", tt.level, currentLevel)
			}
		})
	}
}

func TestLevelConstants(t *testing.T) {
	if DebugLevel >= InfoLevel {
		t.Error("DebugLevel should be less than InfoLevel")
	}
	if InfoLevel >= WarnLevel {
		t.Error("InfoLevel should be less than WarnLevel")
	}
	if WarnLevel >= ErrorLevel {
		t.Error("WarnLevel should be less than ErrorLevel")
	}
}

func TestTimestamp(t *testing.T) {
	ts := timestamp()
	if len(ts) != 8 { // HH:MM:SS
		t.Errorf("expected timestamp length 8, got %d: %q", len(ts), ts)
	}
	if ts[2] != ':' || ts[5] != ':' {
		t.Errorf("expected HH:MM:SS format, got %q", ts)
	}
}

// Test that all logger functions don't panic at various log levels
func TestDebugNoPanic(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()

	SetLevel(DebugLevel)
	Debug("test %s %d", "msg", 42)

	SetLevel(InfoLevel)
	Debug("should not print %s", "hidden")
}

func TestInfoNoPanic(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()

	SetLevel(InfoLevel)
	Info("test %s %d", "msg", 42)

	SetLevel(ErrorLevel)
	Info("should not print %s", "hidden")
}

func TestSuccessNoPanic(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()

	SetLevel(InfoLevel)
	Success("test %s %d", "msg", 42)

	SetLevel(ErrorLevel)
	Success("should not print %s", "hidden")
}

func TestWarnNoPanic(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()

	SetLevel(WarnLevel)
	Warn("test %s %d", "msg", 42)

	SetLevel(ErrorLevel)
	Warn("should not print %s", "hidden")
}

func TestErrorNoPanic(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()

	SetLevel(ErrorLevel)
	Error("test %s %d", "msg", 42)
}

func TestStepNoPanic(t *testing.T) {
	Step(1, 5, "Building %s", "api")
	Step(5, 5, "Done")
}

func TestHeaderNoPanic(t *testing.T) {
	Header("Deploy: %s → %s", "api", "prod")
	Header("Simple header")
}

func TestServerActionNoPanic(t *testing.T) {
	ServerAction("10.0.0.1", "Installing %s", "api")
	ServerAction("host", "simple action")
}

func TestDebugWithFormatting(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()

	SetLevel(DebugLevel)
	// Should not panic with various format specifiers
	Debug("string: %s, int: %d, float: %f, bool: %t", "hello", 42, 3.14, true)
	Debug("no args")
	Debug("")
}

func TestNewWriter(t *testing.T) {
	levels := []Level{DebugLevel, InfoLevel, WarnLevel, ErrorLevel}
	for _, l := range levels {
		w := NewWriter(l)
		if w == nil {
			t.Errorf("NewWriter(%d) returned nil", l)
		}
		if w.level != l {
			t.Errorf("expected level %d, got %d", l, w.level)
		}
	}
}

func TestWriterWrite(t *testing.T) {
	original := currentLevel
	defer func() { currentLevel = original }()
	SetLevel(DebugLevel)

	tests := []struct {
		name  string
		level Level
		input string
	}{
		{"debug single line", DebugLevel, "hello world\n"},
		{"info single line", InfoLevel, "hello world\n"},
		{"warn single line", WarnLevel, "hello world\n"},
		{"error single line", ErrorLevel, "hello world\n"},
		{"multiple lines", InfoLevel, "line1\nline2\nline3\n"},
		{"empty line filtered", InfoLevel, "before\n\nafter\n"},
		{"no trailing newline", InfoLevel, "partial"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWriter(tt.level)
			n, err := w.Write([]byte(tt.input))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("expected %d bytes written, got %d", len(tt.input), n)
			}
		})
	}
}

func TestWriterIncompleteLines(t *testing.T) {
	w := NewWriter(InfoLevel)

	// Write partial line
	n, err := w.Write([]byte("partial"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7, got %d", n)
	}

	// Complete the line
	n, err = w.Write([]byte(" data\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6, got %d", n)
	}
}

func TestWriterEmptyWrite(t *testing.T) {
	w := NewWriter(InfoLevel)
	n, err := w.Write([]byte{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestWriterBufferPersistence(t *testing.T) {
	w := NewWriter(InfoLevel)

	// Write incomplete line
	w.Write([]byte("hello"))

	// Buffer should still hold "hello"
	if w.buf.Len() == 0 {
		t.Error("expected buffer to hold incomplete data")
	}
}

func TestWriterImplementsIOWriter(t *testing.T) {
	w := NewWriter(InfoLevel)

	// Verify it works with bytes.Buffer (which accepts io.Writer)
	var buf bytes.Buffer
	_ = buf // just making sure Writer satisfies io.Writer interface
	var _ interface{ Write([]byte) (int, error) } = w
}
