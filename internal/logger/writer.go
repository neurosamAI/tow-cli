package logger

import (
	"bytes"
	"fmt"
	"os"
)

// Writer implements io.Writer for piping command output to the logger
type Writer struct {
	level Level
	buf   bytes.Buffer
}

// NewWriter creates a new log writer at the specified level
func NewWriter(level Level) *Writer {
	return &Writer{level: level}
}

// Write implements io.Writer
func (w *Writer) Write(p []byte) (n int, err error) {
	w.buf.Write(p)

	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Put back incomplete line
			w.buf.WriteString(line)
			break
		}

		line = line[:len(line)-1] // remove trailing newline
		if line == "" {
			continue
		}

		switch w.level {
		case DebugLevel:
			fmt.Fprintf(os.Stderr, "%s    %s%s\n", colorGray, line, colorReset)
		case InfoLevel:
			fmt.Fprintf(os.Stderr, "    %s\n", line)
		case WarnLevel:
			fmt.Fprintf(os.Stderr, "    %s%s%s\n", colorYellow, line, colorReset)
		case ErrorLevel:
			fmt.Fprintf(os.Stderr, "    %s%s%s\n", colorRed, line, colorReset)
		}
	}

	return len(p), nil
}
