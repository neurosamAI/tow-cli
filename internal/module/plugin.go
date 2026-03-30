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

// EmbeddedPluginData holds raw YAML content of bundled plugins (set by main package via SetEmbeddedPlugins)
var embeddedPluginData map[string][]byte

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
	// Supported versions — value can be a URL string or a VersionOverride object
	Versions map[string]VersionEntry `yaml:"versions"`

	// Default version to use
	DefaultVersion string `yaml:"default_version"`

	// URL template: {{VERSION}} is replaced
	URLTemplate string `yaml:"url_template"`
}

// VersionEntry holds either a simple URL string or a full override.
// YAML supports both formats:
//
//	"3.7.0": "https://..."                          (simple URL)
//	"3.7.0":                                        (full override)
//	  url: "https://..."
//	  start_cmd: "..."
type VersionEntry struct {
	URL         string             `yaml:"url"`
	StartCmd    string             `yaml:"start_cmd"`
	StopCmd     string             `yaml:"stop_cmd"`
	StatusCmd   string             `yaml:"status_cmd"`
	BuildCmd    string             `yaml:"build_cmd"`
	Provision   *PluginProvision   `yaml:"provision"`
	HealthCheck *PluginHealthCheck `yaml:"health_check"`
}

// UnmarshalYAML allows VersionEntry to accept both a plain string (URL) and a map
func (v *VersionEntry) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try string first (simple URL format)
	var url string
	if err := unmarshal(&url); err == nil {
		v.URL = url
		return nil
	}

	// Try struct (full override format)
	type alias VersionEntry
	var a alias
	if err := unmarshal(&a); err != nil {
		return err
	}
	*v = VersionEntry(a)
	return nil
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

// versionOverride returns the override for the default version, if any
func (h *PluginHandler) versionOverride() *VersionEntry {
	ver := h.Def.Package.DefaultVersion
	if ver == "" {
		return nil
	}
	if entry, ok := h.Def.Package.Versions[ver]; ok {
		// Only return if it has override fields (not just a URL)
		if entry.StartCmd != "" || entry.StopCmd != "" || entry.StatusCmd != "" || entry.BuildCmd != "" || entry.Provision != nil {
			return &entry
		}
	}
	return nil
}

func (h *PluginHandler) DefaultBuildCmd(moduleName, env string) string {
	if ov := h.versionOverride(); ov != nil && ov.BuildCmd != "" {
		return h.substitute(ov.BuildCmd, moduleName, "", 0, env)
	}
	return h.substitute(h.Def.BuildCmd, moduleName, "", 0, env)
}

func (h *PluginHandler) DefaultStartCmd(baseDir string, port int) string {
	if ov := h.versionOverride(); ov != nil && ov.StartCmd != "" {
		return h.substitute(ov.StartCmd, "", baseDir, port, "")
	}
	return h.substitute(h.Def.StartCmd, "", baseDir, port, "")
}

func (h *PluginHandler) DefaultStopCmd(baseDir string, port int) string {
	if ov := h.versionOverride(); ov != nil && ov.StopCmd != "" {
		return h.substitute(ov.StopCmd, "", baseDir, port, "")
	}
	return h.substitute(h.Def.StopCmd, "", baseDir, port, "")
}

func (h *PluginHandler) DefaultStatusCmd(baseDir string, port int) string {
	if ov := h.versionOverride(); ov != nil && ov.StatusCmd != "" {
		return h.substitute(ov.StatusCmd, "", baseDir, port, "")
	}
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

// GetProvisionForVersion returns version-specific provision config, falling back to plugin defaults
func GetProvisionForVersion(typeName, version string) *PluginProvision {
	def := GetPluginDef(typeName)
	if def == nil {
		return nil
	}

	// Check version-specific override
	if version != "" {
		if entry, ok := def.Package.Versions[version]; ok && entry.Provision != nil {
			return entry.Provision
		}
	}

	// Check default version override
	if def.Package.DefaultVersion != "" {
		if entry, ok := def.Package.Versions[def.Package.DefaultVersion]; ok && entry.Provision != nil {
			return entry.Provision
		}
	}

	// Fallback to plugin-level provision
	return &def.Provision
}

// PluginDirs returns the default plugin directories to search
func PluginDirs() []string {
	dirs := []string{"plugins"}

	home, err := os.UserHomeDir()
	if err == nil {
		dirs = append(dirs, filepath.Join(home, ".tow", "plugins"))
	}

	return dirs
}

// SetEmbeddedPlugins registers bundled plugin YAML data (called from main)
func SetEmbeddedPlugins(data map[string][]byte) {
	embeddedPluginData = data
}

// LoadEmbeddedPlugins loads plugins from embedded data (bundled in binary)
func LoadEmbeddedPlugins() {
	if embeddedPluginData == nil {
		return
	}
	for name, data := range embeddedPluginData {
		var def PluginDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			continue
		}
		if def.Name == "" {
			def.Name = strings.TrimSuffix(name, ".yaml")
		}
		// Don't overwrite built-in or filesystem-loaded handlers
		if _, exists := registry[def.Name]; !exists {
			Register(&PluginHandler{Def: def})
		}
	}
}
