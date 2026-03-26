// Plugin system for community-contributed module handlers.
// by neurosam.AI — https://neurosam.ai
//
// Plugins are YAML files that define module type handlers.
// They can be placed in:
//   1. plugins/ directory in the project root
//   2. ~/.tow/plugins/ for global plugins
//   3. Distributed via community registry (future)
//
// This allows anyone to contribute support for new services
// (e.g., Elasticsearch, RabbitMQ, Nginx) without writing Go code.

package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginDef defines a module handler via YAML
type PluginDef struct {
	Name        string `yaml:"name"`        // e.g., "elasticsearch"
	Description string `yaml:"description"` // Human-readable description
	Version     string `yaml:"version"`     // Plugin version
	Author      string `yaml:"author"`

	// Package info — for external packages with multiple versions
	Package PackageInfo `yaml:"package"`

	// Default commands — use {{MODULE}}, {{BASE_DIR}}, {{PORT}}, {{ENV}} placeholders
	BuildCmd  string `yaml:"build_cmd"`
	StartCmd  string `yaml:"start_cmd"`
	StopCmd   string `yaml:"stop_cmd"`
	StatusCmd string `yaml:"status_cmd"`

	// Artifacts
	ArtifactPath    string   `yaml:"artifact_path"`
	PackageIncludes []string `yaml:"package_includes"`

	// Health check defaults
	HealthCheck PluginHealthCheck `yaml:"health_check"`

	// Provision — module-specific server setup
	Provision PluginProvision `yaml:"provision"`

	// Data directories to persist across deployments
	DataDirs []string `yaml:"data_dirs"`

	// Log file name (relative to logs/)
	LogFile string `yaml:"log_file"`
}

// PackageInfo defines external package versioning
type PackageInfo struct {
	// Supported versions with download URLs
	// e.g., "3.7.0": "https://downloads.apache.org/kafka/3.7.0/kafka_2.13-3.7.0.tgz"
	Versions map[string]string `yaml:"versions"`

	// Default version to use
	DefaultVersion string `yaml:"default_version"`

	// URL template: {{VERSION}} is replaced
	// e.g., "https://downloads.apache.org/kafka/{{VERSION}}/kafka_2.13-{{VERSION}}.tgz"
	URLTemplate string `yaml:"url_template"`
}

// PluginHealthCheck defines default health check for the plugin
type PluginHealthCheck struct {
	Type    string `yaml:"type"`   // tcp, http, command
	Target  string `yaml:"target"` // uses {{PORT}}, {{BASE_DIR}}
	Timeout int    `yaml:"timeout"`
}

// PluginProvision defines server provisioning steps
type PluginProvision struct {
	// System packages to install (apt/yum auto-detected)
	Packages []string `yaml:"packages"`

	// Directories to create on the server
	Directories []string `yaml:"directories"` // uses {{BASE_DIR}}

	// Shell commands to run during provisioning
	Commands []string `yaml:"commands"` // uses {{BASE_DIR}}, {{MODULE}}, {{PORT}}
}

// PluginHandler wraps a PluginDef to implement the Handler interface
type PluginHandler struct {
	Def PluginDef
}

func (h *PluginHandler) Name() string { return h.Def.Name }

func (h *PluginHandler) DefaultBuildCmd(moduleName, env string) string {
	return h.substitute(h.Def.BuildCmd, moduleName, "", 0, env)
}

func (h *PluginHandler) DefaultStartCmd(baseDir string, port int) string {
	return h.substitute(h.Def.StartCmd, "", baseDir, port, "")
}

func (h *PluginHandler) DefaultStopCmd(baseDir string, port int) string {
	return h.substitute(h.Def.StopCmd, "", baseDir, port, "")
}

func (h *PluginHandler) DefaultStatusCmd(baseDir string, port int) string {
	cmd := h.Def.StatusCmd
	if cmd == "" {
		if port > 0 {
			return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
		}
		return ""
	}
	return h.substitute(cmd, "", baseDir, port, "")
}

func (h *PluginHandler) DefaultArtifactPath(moduleName string) string {
	path := h.Def.ArtifactPath
	if path == "" {
		return fmt.Sprintf("build/%s.tar.gz", moduleName)
	}
	return h.substitute(path, moduleName, "", 0, "")
}

func (h *PluginHandler) PackageContents(moduleName, baseDir string) []string {
	if len(h.Def.PackageIncludes) > 0 {
		return h.Def.PackageIncludes
	}
	return []string{"bin/", "config/"}
}

// substitute replaces {{MODULE}}, {{BASE_DIR}}, {{PORT}}, {{ENV}}, {{VERSION}} in a template
func (h *PluginHandler) substitute(tmpl, moduleName, baseDir string, port int, env string) string {
	if tmpl == "" {
		return ""
	}
	r := strings.NewReplacer(
		"{{MODULE}}", moduleName,
		"{{BASE_DIR}}", baseDir,
		"{{PORT}}", fmt.Sprintf("%d", port),
		"{{ENV}}", env,
		"{{VERSION}}", h.Def.Package.DefaultVersion,
	)
	return r.Replace(tmpl)
}

// LoadPlugins loads all YAML plugin definitions from the given directories
func LoadPlugins(dirs ...string) error {
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			if err := loadPlugin(path); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to load plugin %s: %v\n", path, err)
			}
		}
	}

	return nil
}

func loadPlugin(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var def PluginDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	if def.Name == "" {
		return fmt.Errorf("%s: name is required", path)
	}

	// Don't overwrite built-in handlers
	if _, exists := registry[def.Name]; exists {
		return nil
	}

	Register(&PluginHandler{Def: def})
	return nil
}

// GetPluginDef returns the plugin definition if the handler is a plugin
func GetPluginDef(typeName string) *PluginDef {
	h, ok := registry[typeName]
	if !ok {
		return nil
	}
	if ph, ok := h.(*PluginHandler); ok {
		return &ph.Def
	}
	return nil
}

// PluginDir returns the default plugin directories to search
func PluginDirs() []string {
	dirs := []string{"plugins"}

	home, err := os.UserHomeDir()
	if err == nil {
		dirs = append(dirs, filepath.Join(home, ".tow", "plugins"))
	}

	return dirs
}
