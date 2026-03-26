package module

import (
	"os"
	"path/filepath"
	"testing"
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
