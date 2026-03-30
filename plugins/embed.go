// Bundled infrastructure plugins — embedded in binary at compile time.
// Import this package to auto-register all bundled plugins.
package plugins

import (
	"embed"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/module"
)

//go:embed *.yaml
var content embed.FS

func init() {
	entries, err := content.ReadDir(".")
	if err != nil {
		return
	}

	data := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		b, err := content.ReadFile(entry.Name())
		if err != nil {
			continue
		}
		data[entry.Name()] = b
	}

	module.SetEmbeddedPlugins(data)
	module.LoadEmbeddedPlugins()
}
