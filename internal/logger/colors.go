package logger

import "fmt"

// Terminal color codes
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorGray    = "\033[90m"
	ColorBold    = "\033[1m"

	// Bright variants
	ColorBrightRed     = "\033[91m"
	ColorBrightGreen   = "\033[92m"
	ColorBrightYellow  = "\033[93m"
	ColorBrightBlue    = "\033[94m"
	ColorBrightMagenta = "\033[95m"
)

// ServerColors is a rotating palette for multi-server output
var ServerColors = []string{
	ColorCyan,
	ColorYellow,
	ColorGreen,
	ColorMagenta,
	ColorBlue,
	ColorBrightRed,
	ColorBrightGreen,
	ColorBrightYellow,
	ColorBrightBlue,
	ColorBrightMagenta,
}

// ServerColor returns a color for the given server index (rotating)
func ServerColor(index int) string {
	return ServerColors[index%len(ServerColors)]
}

// ColorPrefix returns a colored "[name] " prefix for server log output
func ColorPrefix(name string, index int) string {
	return fmt.Sprintf("%s[%s]%s ", ServerColor(index), name, ColorReset)
}
