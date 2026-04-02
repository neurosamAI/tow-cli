package logger

import (
	"fmt"
	"os"
	"time"
)

// Level represents log severity
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var currentLevel = InfoLevel

// Unexported aliases for internal use (exported constants in colors.go)
const (
	colorReset  = ColorReset
	colorRed    = ColorRed
	colorGreen  = ColorGreen
	colorYellow = ColorYellow
	colorBlue   = ColorBlue
	colorCyan   = ColorCyan
	colorGray   = ColorGray
	colorBold   = ColorBold
)

func SetLevel(l Level) {
	currentLevel = l
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func Debug(format string, args ...interface{}) {
	if currentLevel <= DebugLevel {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "%s[%s]%s %sDEBUG%s %s\n", colorGray, timestamp(), colorReset, colorGray, colorReset, msg)
	}
}

func Info(format string, args ...interface{}) {
	if currentLevel <= InfoLevel {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "%s[%s]%s %sINFO%s  %s\n", colorGray, timestamp(), colorReset, colorBlue, colorReset, msg)
	}
}

func Success(format string, args ...interface{}) {
	if currentLevel <= InfoLevel {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "%s[%s]%s %s✓%s %s\n", colorGray, timestamp(), colorReset, colorGreen, colorReset, msg)
	}
}

func Warn(format string, args ...interface{}) {
	if currentLevel <= WarnLevel {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "%s[%s]%s %sWARN%s  %s\n", colorGray, timestamp(), colorReset, colorYellow, colorReset, msg)
	}
}

func Error(format string, args ...interface{}) {
	if currentLevel <= ErrorLevel {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "%s[%s]%s %sERROR%s %s\n", colorGray, timestamp(), colorReset, colorRed, colorReset, msg)
	}
}

// Step prints a pipeline step indicator
func Step(step, total int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s[%s]%s %s[%d/%d]%s %s\n", colorGray, timestamp(), colorReset, colorCyan, step, total, colorReset, msg)
}

// Header prints a section header
func Header(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "\n%s%s━━━ %s ━━━%s\n", colorBold, colorCyan, msg, colorReset)
}

// ServerAction prints a server-targeted action
func ServerAction(host string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s[%s]%s %s[%s]%s %s\n", colorGray, timestamp(), colorReset, colorYellow, host, colorReset, msg)
}
