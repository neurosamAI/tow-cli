package module

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPluginHandler(t *testing.T) {
	h := &PluginHandler{
		Def: PluginDef{
			Name:            "testservice",
			Description:     "Test service",
			StartCmd:        "{{BASE_DIR}}/current/bin/start --port {{PORT}}",
			StopCmd:         "kill $(lsof -i :{{PORT}} -t)",
			StatusCmd:       "lsof -i :{{PORT}} -t",
			BuildCmd:        "make build ENV={{ENV}}",
			ArtifactPath:    "build/{{MODULE}}.tar.gz",
			PackageIncludes: []string{"bin/", "config/"},
			Package: PackageInfo{
				DefaultVersion: "2.0.0",
			},
		},
	}

	if h.Name() != "testservice" {
		t.Errorf("expected testservice, got %s", h.Name())
	}

	start := h.DefaultStartCmd("/app/svc", 8080)
	if start != "/app/svc/current/bin/start --port 8080" {
		t.Errorf("unexpected start cmd: %s", start)
	}

	stop := h.DefaultStopCmd("/app/svc", 8080)
	if stop != "kill $(lsof -i :8080 -t)" {
		t.Errorf("unexpected stop cmd: %s", stop)
	}

	build := h.DefaultBuildCmd("mymod", "prod")
	if build != "make build ENV=prod" {
		t.Errorf("unexpected build cmd: %s", build)
	}

	artifact := h.DefaultArtifactPath("mymod")
	if artifact != "build/mymod.tar.gz" {
		t.Errorf("unexpected artifact: %s", artifact)
	}

	contents := h.PackageContents("mymod", "")
	if len(contents) != 2 || contents[0] != "bin/" {
		t.Errorf("unexpected contents: %v", contents)
	}
}

func TestPluginHandlerEmptyFields(t *testing.T) {
	h := &PluginHandler{
		Def: PluginDef{
			Name: "minimal",
		},
	}

	if h.DefaultBuildCmd("m", "e") != "" {
		t.Error("expected empty build cmd")
	}
	if h.DefaultStartCmd("/app", 0) != "" {
		t.Error("expected empty start cmd")
	}
	if h.DefaultArtifactPath("mod") != "build/mod.tar.gz" {
		t.Error("expected default artifact path")
	}
}

func TestLoadPluginsFromDirectory(t *testing.T) {
	// Create temp dir with a test plugin
	tmpDir := t.TempDir()
	pluginContent := `
name: testplugin
description: A test plugin
build_cmd: "make build"
start_cmd: "./bin/server"
stop_cmd: "kill $(cat server.pid)"
artifact_path: "build/testplugin.tar.gz"
health_check:
  type: tcp
  timeout: 30
`
	err := os.WriteFile(filepath.Join(tmpDir, "testplugin.yaml"), []byte(pluginContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Load plugins
	err = LoadPlugins(tmpDir)
	if err != nil {
		t.Errorf("unexpected error loading plugins: %v", err)
	}

	// Check plugin was registered
	h, err := Get("testplugin")
	if err != nil {
		t.Errorf("expected testplugin to be registered: %v", err)
	}
	if h.Name() != "testplugin" {
		t.Errorf("expected name testplugin, got %s", h.Name())
	}
}

func TestLoadPluginsNonExistentDir(t *testing.T) {
	err := LoadPlugins("/nonexistent/path")
	if err != nil {
		t.Errorf("expected no error for nonexistent dir, got %v", err)
	}
}

func TestLoadPluginsInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "bad.yaml"), []byte("invalid: [yaml: broken"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Should not return error (just warns)
	err = LoadPlugins(tmpDir)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestLoadPluginsNoName(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "noname.yaml"), []byte("description: no name field"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = LoadPlugins(tmpDir)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestGetPluginDef(t *testing.T) {
	// Built-in handler should return nil
	def := GetPluginDef("java")
	if def != nil {
		t.Error("expected nil for built-in handler")
	}

	// Non-existent should return nil
	def = GetPluginDef("nonexistent")
	if def != nil {
		t.Error("expected nil for non-existent")
	}
}

func TestPluginDirs(t *testing.T) {
	dirs := PluginDirs()
	if len(dirs) < 1 {
		t.Error("expected at least 1 plugin directory")
	}
	if dirs[0] != "plugins" {
		t.Errorf("expected first dir to be 'plugins', got %s", dirs[0])
	}
}

func TestVersionEntryUnmarshalYAMLString(t *testing.T) {
	// Test simple URL string format
	yamlStr := `"https://example.com/pkg-3.7.0.tar.gz"`
	var v VersionEntry
	if err := yaml.Unmarshal([]byte(yamlStr), &v); err != nil {
		t.Fatalf("unmarshal string URL failed: %v", err)
	}
	if v.URL != "https://example.com/pkg-3.7.0.tar.gz" {
		t.Errorf("expected URL string, got %q", v.URL)
	}
	if v.StartCmd != "" {
		t.Errorf("expected empty StartCmd for string format, got %q", v.StartCmd)
	}
}

func TestVersionEntryUnmarshalYAMLStruct(t *testing.T) {
	// Test full override struct format
	yamlStr := `
url: "https://example.com/pkg-3.7.0.tar.gz"
start_cmd: "./bin/start --config=prod"
stop_cmd: "./bin/stop"
`
	var v VersionEntry
	if err := yaml.Unmarshal([]byte(yamlStr), &v); err != nil {
		t.Fatalf("unmarshal struct failed: %v", err)
	}
	if v.URL != "https://example.com/pkg-3.7.0.tar.gz" {
		t.Errorf("expected URL, got %q", v.URL)
	}
	if v.StartCmd != "./bin/start --config=prod" {
		t.Errorf("expected start_cmd, got %q", v.StartCmd)
	}
	if v.StopCmd != "./bin/stop" {
		t.Errorf("expected stop_cmd, got %q", v.StopCmd)
	}
}

func TestVersionEntryUnmarshalYAMLInvalid(t *testing.T) {
	// Neither string nor valid struct
	yamlStr := `[1, 2, 3]`
	var v VersionEntry
	err := yaml.Unmarshal([]byte(yamlStr), &v)
	if err == nil {
		t.Fatal("expected error for invalid YAML type")
	}
}

func TestGetProvisionForVersion(t *testing.T) {
	// Register a test plugin with version-specific provision
	pluginDef := PluginDef{
		Name: "test-provision-svc",
		Provision: PluginProvision{
			Packages: []string{"base-pkg"},
			Commands: []string{"echo base-setup"},
		},
		Package: PackageInfo{
			DefaultVersion: "2.0.0",
			Versions: map[string]VersionEntry{
				"2.0.0": {
					URL: "https://example.com/svc-2.0.0.tar.gz",
					Provision: &PluginProvision{
						Packages: []string{"v2-pkg"},
						Commands: []string{"echo v2-setup"},
					},
				},
				"1.0.0": {
					URL: "https://example.com/svc-1.0.0.tar.gz",
					// No version-specific provision
				},
			},
		},
	}
	Register(&PluginHandler{Def: pluginDef})

	// Test version-specific provision
	prov := GetProvisionForVersion("test-provision-svc", "2.0.0")
	if prov == nil {
		t.Fatal("expected non-nil provision for version 2.0.0")
	}
	if len(prov.Packages) != 1 || prov.Packages[0] != "v2-pkg" {
		t.Errorf("expected v2-pkg, got %v", prov.Packages)
	}

	// Test fallback to default version provision (version "1.0.0" has no provision, falls to default "2.0.0")
	prov = GetProvisionForVersion("test-provision-svc", "1.0.0")
	if prov == nil {
		t.Fatal("expected non-nil provision for version 1.0.0 (fallback to default)")
	}
	if prov.Packages[0] != "v2-pkg" {
		t.Errorf("expected fallback to default version provision, got %v", prov.Packages)
	}

	// Test fallback to plugin-level provision for unknown type
	prov = GetProvisionForVersion("nonexistent-type", "1.0.0")
	if prov != nil {
		t.Errorf("expected nil for nonexistent type, got %v", prov)
	}
}

func TestSetEmbeddedPluginsAndLoad(t *testing.T) {
	// Clear any previously set embedded data
	SetEmbeddedPlugins(nil)
	LoadEmbeddedPlugins() // should be a no-op with nil data

	// Set embedded plugin data
	pluginYAML := []byte(`
name: embedded-test-svc
description: An embedded test service
start_cmd: "./bin/start"
stop_cmd: "./bin/stop"
`)
	SetEmbeddedPlugins(map[string][]byte{
		"embedded-test-svc.yaml": pluginYAML,
	})

	// Load embedded plugins
	LoadEmbeddedPlugins()

	// Verify it was registered
	h, err := Get("embedded-test-svc")
	if err != nil {
		t.Fatalf("expected embedded-test-svc to be registered: %v", err)
	}
	if h.Name() != "embedded-test-svc" {
		t.Errorf("expected name embedded-test-svc, got %s", h.Name())
	}

	// Verify it has correct commands
	start := h.DefaultStartCmd("/app/svc", 8080)
	if start != "./bin/start" {
		t.Errorf("expected './bin/start', got %q", start)
	}
}

func TestLoadEmbeddedPluginsNoName(t *testing.T) {
	// Plugin with no name should derive name from filename
	pluginYAML := []byte(`
description: No name field
start_cmd: "./start"
`)
	SetEmbeddedPlugins(map[string][]byte{
		"derived-name-svc.yaml": pluginYAML,
	})

	LoadEmbeddedPlugins()

	h, err := Get("derived-name-svc")
	if err != nil {
		t.Fatalf("expected derived-name-svc to be registered: %v", err)
	}
	if h.Name() != "derived-name-svc" {
		t.Errorf("expected derived name, got %s", h.Name())
	}
}

func TestLoadEmbeddedPluginsInvalidYAML(t *testing.T) {
	SetEmbeddedPlugins(map[string][]byte{
		"bad.yaml": []byte("[invalid yaml: broken"),
	})

	// Should not panic
	LoadEmbeddedPlugins()
}

func TestPluginHandlerVersionOverride(t *testing.T) {
	h := &PluginHandler{
		Def: PluginDef{
			Name:     "versioned-svc",
			StartCmd: "default-start",
			StopCmd:  "default-stop",
			Package: PackageInfo{
				DefaultVersion: "3.0.0",
				Versions: map[string]VersionEntry{
					"3.0.0": {
						URL:      "https://example.com/svc-3.0.0.tar.gz",
						StartCmd: "v3-start --config=new",
						StopCmd:  "v3-stop",
					},
				},
			},
		},
	}

	// Version override should take precedence
	start := h.DefaultStartCmd("/app/svc", 9092)
	if start != "v3-start --config=new" {
		t.Errorf("expected version override start cmd, got %q", start)
	}

	stop := h.DefaultStopCmd("/app/svc", 9092)
	if stop != "v3-stop" {
		t.Errorf("expected version override stop cmd, got %q", stop)
	}
}

func TestPluginHandlerVersionOverridePartial(t *testing.T) {
	// Version entry with only StartCmd override, StopCmd should use default
	h := &PluginHandler{
		Def: PluginDef{
			Name:     "partial-override-svc",
			StartCmd: "default-start",
			StopCmd:  "default-stop",
			Package: PackageInfo{
				DefaultVersion: "2.0.0",
				Versions: map[string]VersionEntry{
					"2.0.0": {
						URL:      "https://example.com/svc-2.0.0.tar.gz",
						StartCmd: "v2-start",
						// No StopCmd override
					},
				},
			},
		},
	}

	start := h.DefaultStartCmd("/app/svc", 0)
	if start != "v2-start" {
		t.Errorf("expected version override start cmd, got %q", start)
	}

	stop := h.DefaultStopCmd("/app/svc", 0)
	if stop != "default-stop" {
		t.Errorf("expected default stop cmd since version only overrides start, got %q", stop)
	}
}

func TestPluginHandlerVersionOverrideNoOverrideFields(t *testing.T) {
	// Version entry with only URL (no override fields) should not trigger override
	h := &PluginHandler{
		Def: PluginDef{
			Name:     "url-only-svc",
			StartCmd: "default-start",
			Package: PackageInfo{
				DefaultVersion: "1.0.0",
				Versions: map[string]VersionEntry{
					"1.0.0": {
						URL: "https://example.com/svc-1.0.0.tar.gz",
						// No command overrides
					},
				},
			},
		},
	}

	start := h.DefaultStartCmd("/app/svc", 0)
	if start != "default-start" {
		t.Errorf("expected default start cmd (URL-only entry should not override), got %q", start)
	}
}
