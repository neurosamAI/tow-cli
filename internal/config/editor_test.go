package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddModule(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	initial := `project:
  name: test-app
  base_dir: /app

modules:
  existing:
    type: generic
`
	os.WriteFile(configPath, []byte(initial), 0644)

	err := AddModule(configPath, "api", "springboot", 8080, "1.0.0")
	if err != nil {
		t.Fatalf("AddModule failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)

	if !strings.Contains(content, "api:") {
		t.Error("expected config to contain 'api:'")
	}
	if !strings.Contains(content, "type: springboot") {
		t.Error("expected config to contain 'type: springboot'")
	}
	if !strings.Contains(content, "port: 8080") {
		t.Error("expected config to contain 'port: 8080'")
	}
	if !strings.Contains(content, `version: "1.0.0"`) {
		t.Error("expected config to contain version")
	}
}

func TestAddModuleDuplicate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	initial := `project:
  name: test-app

modules:
  api:
    type: generic
`
	os.WriteFile(configPath, []byte(initial), 0644)

	err := AddModule(configPath, "api", "springboot", 8080, "")
	if err == nil {
		t.Fatal("expected error for duplicate module")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAddModuleNoModulesSection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	initial := `project:
  name: test-app
`
	os.WriteFile(configPath, []byte(initial), 0644)

	err := AddModule(configPath, "api", "node", 3000, "")
	if err != nil {
		t.Fatalf("AddModule failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)

	if !strings.Contains(content, "modules:") {
		t.Error("expected modules section to be added")
	}
	if !strings.Contains(content, "api:") {
		t.Error("expected api module to be added")
	}
}

func TestAddModuleNoPortNoVersion(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	initial := `modules:
  existing:
    type: generic
`
	os.WriteFile(configPath, []byte(initial), 0644)

	err := AddModule(configPath, "worker", "generic", 0, "")
	if err != nil {
		t.Fatalf("AddModule failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)

	if strings.Contains(content, "port:") {
		t.Error("expected no port line when port is 0")
	}
	if strings.Contains(content, "version:") {
		t.Error("expected no version line when version is empty")
	}
}

func TestAddModuleFileNotFound(t *testing.T) {
	err := AddModule("/nonexistent/path/tow.yaml", "api", "generic", 0, "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRemoveModule(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	initial := `modules:
  api:
    type: springboot
    port: 8080
  worker:
    type: generic
    port: 9090
`
	os.WriteFile(configPath, []byte(initial), 0644)

	err := RemoveModule(configPath, "api")
	if err != nil {
		t.Fatalf("RemoveModule failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)

	if strings.Contains(content, "  api:") {
		t.Error("expected api module to be removed")
	}
	if !strings.Contains(content, "  worker:") {
		t.Error("expected worker module to remain")
	}
}

func TestRemoveModuleFileNotFound(t *testing.T) {
	err := RemoveModule("/nonexistent/path/tow.yaml", "api")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRemoveModuleNotPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	initial := `modules:
  worker:
    type: generic
`
	os.WriteFile(configPath, []byte(initial), 0644)

	// Should not error if module is not present; just a no-op
	err := RemoveModule(configPath, "api")
	if err != nil {
		t.Fatalf("RemoveModule should not fail for missing module: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	if !strings.Contains(string(data), "worker:") {
		t.Error("expected worker to remain")
	}
}

func TestAddServer(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")

	err := AddServer(localPath, "dev", "web-1", "10.0.0.1", 1, []string{"api", "worker"})
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	data, _ := os.ReadFile(localPath)
	content := string(data)

	if !strings.Contains(content, "environments:") {
		t.Error("expected environments section")
	}
	if !strings.Contains(content, "dev:") {
		t.Error("expected dev environment")
	}
	if !strings.Contains(content, "name: web-1") {
		t.Error("expected server name web-1")
	}
	if !strings.Contains(content, "host: 10.0.0.1") {
		t.Error("expected host 10.0.0.1")
	}
	if !strings.Contains(content, "modules: [api, worker]") {
		t.Error("expected modules list")
	}
}

func TestAddServerToExistingEnv(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
`
	os.WriteFile(localPath, []byte(initial), 0644)

	err := AddServer(localPath, "dev", "web-2", "10.0.0.2", 2, nil)
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	data, _ := os.ReadFile(localPath)
	content := string(data)

	if !strings.Contains(content, "name: web-2") {
		t.Error("expected new server web-2")
	}
	if !strings.Contains(content, "host: 10.0.0.2") {
		t.Error("expected host 10.0.0.2")
	}
}

func TestAddServerNoNumber(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")

	err := AddServer(localPath, "staging", "app-1", "192.168.1.1", 0, nil)
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	data, _ := os.ReadFile(localPath)
	content := string(data)

	// number: 0 should not be written
	if strings.Contains(content, "number:") {
		t.Error("expected no number line when number is 0")
	}
}

func TestRemoveServer(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
      - name: web-2
        host: 10.0.0.2
`
	os.WriteFile(localPath, []byte(initial), 0644)

	err := RemoveServer(localPath, "dev", "web-1")
	if err != nil {
		t.Fatalf("RemoveServer failed: %v", err)
	}

	data, _ := os.ReadFile(localPath)
	content := string(data)

	if strings.Contains(content, "name: web-1") {
		t.Error("expected web-1 to be removed")
	}
	if !strings.Contains(content, "name: web-2") {
		t.Error("expected web-2 to remain")
	}
}

func TestRemoveServerFileNotFound(t *testing.T) {
	err := RemoveServer("/nonexistent/path/tow.local.yaml", "dev", "web-1")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAssignModules(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
        modules: [api]
`
	os.WriteFile(localPath, []byte(initial), 0644)

	err := AssignModules(localPath, "dev", "web-1", []string{"worker"})
	if err != nil {
		t.Fatalf("AssignModules failed: %v", err)
	}

	data, _ := os.ReadFile(localPath)
	content := string(data)

	// Should contain both api and worker (order may vary)
	if !strings.Contains(content, "modules:") {
		t.Error("expected modules line")
	}
}

func TestAssignModulesEnvNotFound(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
        modules: [api]
`
	os.WriteFile(localPath, []byte(initial), 0644)

	err := AssignModules(localPath, "staging", "web-1", []string{"worker"})
	if err == nil {
		t.Fatal("expected error for non-existent environment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAssignModulesServerNotFound(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
        modules: [api]
`
	os.WriteFile(localPath, []byte(initial), 0644)

	err := AssignModules(localPath, "dev", "web-99", []string{"worker"})
	if err == nil {
		t.Fatal("expected error for non-existent server")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUnassignModules(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
        modules: [api, worker, batch]
`
	os.WriteFile(localPath, []byte(initial), 0644)

	err := UnassignModules(localPath, "dev", "web-1", []string{"worker"})
	if err != nil {
		t.Fatalf("UnassignModules failed: %v", err)
	}

	data, _ := os.ReadFile(localPath)
	content := string(data)

	if strings.Contains(content, "worker") {
		t.Error("expected worker to be removed from modules")
	}
}

func TestUnassignModulesEnvNotFound(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
        modules: [api]
`
	os.WriteFile(localPath, []byte(initial), 0644)

	err := UnassignModules(localPath, "staging", "web-1", []string{"api"})
	if err == nil {
		t.Fatal("expected error for non-existent environment")
	}
}

func TestUnassignModulesFileNotFound(t *testing.T) {
	err := UnassignModules("/nonexistent/path/tow.local.yaml", "dev", "web-1", []string{"api"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestListServers(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
      - name: web-2
        host: 10.0.0.2
  prod:
    servers:
      - name: prod-1
        host: 52.78.1.1
`
	os.WriteFile(localPath, []byte(initial), 0644)

	servers, err := ListServers(localPath, "dev")
	if err != nil {
		t.Fatalf("ListServers failed: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}
	if servers[0].Name != "web-1" {
		t.Errorf("expected first server name web-1, got %q", servers[0].Name)
	}
	if servers[1].Host != "10.0.0.2" {
		t.Errorf("expected second server host 10.0.0.2, got %q", servers[1].Host)
	}
}

func TestListServersEnvNotFound(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "tow.local.yaml")
	initial := `environments:
  dev:
    servers:
      - name: web-1
        host: 10.0.0.1
`
	os.WriteFile(localPath, []byte(initial), 0644)

	_, err := ListServers(localPath, "staging")
	if err == nil {
		t.Fatal("expected error for non-existent environment")
	}
}

func TestListServersFileNotFound(t *testing.T) {
	_, err := ListServers("/nonexistent/path/tow.local.yaml", "dev")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- Additional config tests ---

func TestConfigWithRetentionNotifications(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
		Retention: RetentionConfig{
			Keep:        10,
			AutoCleanup: true,
		},
		Notifications: []NotificationConfig{
			{Type: "slack", URL: "https://hooks.slack.com/test"},
			{Type: "webhook", URL: "https://example.com/hook"},
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}

	if cfg.Retention.Keep != 10 {
		t.Errorf("expected retention keep 10, got %d", cfg.Retention.Keep)
	}
	if !cfg.Retention.AutoCleanup {
		t.Error("expected auto_cleanup to be true")
	}
	if len(cfg.Notifications) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(cfg.Notifications))
	}
}

func TestConfigWithDeployPathLogDirLogFile(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{
			DeployPath: "{module}-{server}",
			LogDir:     "logs",
			LogFile:    "app.log",
		},
		Environments: map[string]*Environment{},
		Modules:      map[string]*Module{},
	}
	cfg.applyDefaults()

	if cfg.Defaults.DeployPath != "{module}-{server}" {
		t.Errorf("expected deploy_path '{module}-{server}', got %q", cfg.Defaults.DeployPath)
	}
	if cfg.Defaults.LogDir != "logs" {
		t.Errorf("expected log_dir 'logs', got %q", cfg.Defaults.LogDir)
	}
	if cfg.Defaults.LogFile != "app.log" {
		t.Errorf("expected log_file 'app.log', got %q", cfg.Defaults.LogFile)
	}
}

func TestServerIDWithName(t *testing.T) {
	srv := Server{Name: "api-primary", Number: 1}
	if srv.ID() != "api-primary" {
		t.Errorf("expected 'api-primary', got %q", srv.ID())
	}
}

func TestServerIDWithNumber(t *testing.T) {
	srv := Server{Number: 3}
	if srv.ID() != "3" {
		t.Errorf("expected '3', got %q", srv.ID())
	}
}

func TestServerIDWithZeroNumber(t *testing.T) {
	srv := Server{}
	if srv.ID() != "0" {
		t.Errorf("expected '0', got %q", srv.ID())
	}
}

func TestHostsExpansion(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"prod": {
				Servers: []Server{
					{
						Name:    "web",
						Hosts:   []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
						Modules: []string{"api"},
					},
				},
			},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
	}
	cfg.applyDefaults()

	env := cfg.Environments["prod"]
	if len(env.Servers) != 3 {
		t.Fatalf("expected 3 servers from hosts expansion, got %d", len(env.Servers))
	}

	if env.Servers[0].Name != "web-1" {
		t.Errorf("expected name 'web-1', got %q", env.Servers[0].Name)
	}
	if env.Servers[1].Name != "web-2" {
		t.Errorf("expected name 'web-2', got %q", env.Servers[1].Name)
	}
	if env.Servers[2].Name != "web-3" {
		t.Errorf("expected name 'web-3', got %q", env.Servers[2].Name)
	}

	if env.Servers[0].Host != "10.0.0.1" {
		t.Errorf("expected host '10.0.0.1', got %q", env.Servers[0].Host)
	}
	if env.Servers[2].Host != "10.0.0.3" {
		t.Errorf("expected host '10.0.0.3', got %q", env.Servers[2].Host)
	}

	// Modules should be inherited
	for i, srv := range env.Servers {
		if len(srv.Modules) != 1 || srv.Modules[0] != "api" {
			t.Errorf("server %d: expected modules [api], got %v", i, srv.Modules)
		}
	}
}

func TestHostsExpansionNoName(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"dev": {
				Servers: []Server{
					{
						Hosts: []string{"a.example.com", "b.example.com"},
					},
				},
			},
		},
		Modules: map[string]*Module{},
	}
	cfg.applyDefaults()

	env := cfg.Environments["dev"]
	if len(env.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(env.Servers))
	}
	// Without a base name, Name should be empty
	if env.Servers[0].Name != "" {
		t.Errorf("expected empty name, got %q", env.Servers[0].Name)
	}
}

func TestSummaryFromEditor(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"dev":  {Servers: []Server{{Host: "a"}, {Host: "b"}}},
			"prod": {Servers: []Server{{Host: "c"}}},
		},
		Modules: map[string]*Module{
			"api":    {},
			"worker": {},
		},
	}
	envCount, moduleCount, serverCount := cfg.Summary()
	if envCount != 2 {
		t.Errorf("expected 2 envs, got %d", envCount)
	}
	if moduleCount != 2 {
		t.Errorf("expected 2 modules, got %d", moduleCount)
	}
	if serverCount != 3 {
		t.Errorf("expected 3 servers, got %d", serverCount)
	}
}

func TestGetServersForModuleByName(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"prod": {
				Servers: []Server{
					{Name: "api-1", Number: 1, Host: "10.0.0.1"},
					{Name: "api-2", Number: 2, Host: "10.0.0.2"},
					{Name: "worker-1", Number: 3, Host: "10.0.0.3"},
				},
			},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
	}

	servers, _, err := cfg.GetServersForModuleByName("prod", "api", "api-2", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "api-2" {
		t.Errorf("expected api-2, got %q", servers[0].Name)
	}
}
