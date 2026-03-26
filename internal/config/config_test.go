package config

import (
	"os"
	"path/filepath"
	"testing"

	yamlPkg "gopkg.in/yaml.v3"
)

func TestLoadValidConfig(t *testing.T) {
	yaml := `
project:
  name: test-app
  base_dir: /app

environments:
  dev:
    ssh_user: ubuntu
    ssh_key_path: /tmp/fake-key
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: springboot
    port: 8080
`
	// Create a fake SSH key so validation passes
	os.WriteFile("/tmp/fake-key", []byte("fake"), 0600)
	defer os.Remove("/tmp/fake-key")

	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Project.Name != "test-app" {
		t.Errorf("expected project name 'test-app', got %q", cfg.Project.Name)
	}
	if cfg.Project.BaseDir != "/app" {
		t.Errorf("expected base_dir '/app', got %q", cfg.Project.BaseDir)
	}
	if len(cfg.Environments) != 1 {
		t.Errorf("expected 1 environment, got %d", len(cfg.Environments))
	}
	if len(cfg.Modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(cfg.Modules))
	}
}

func TestLoadMissingProjectName(t *testing.T) {
	yaml := `
project:
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	_, err := Load(tmpFile)
	if err == nil {
		t.Fatal("expected error for missing project name")
	}
}

func TestLoadMissingModuleType(t *testing.T) {
	yaml := `
project:
  name: test
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    port: 8080
`
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	_, err := Load(tmpFile)
	if err == nil {
		t.Fatal("expected error for missing module type")
	}
}

func TestApplyDefaults(t *testing.T) {
	yaml := `
project:
  name: test
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: springboot
    port: 8080
`
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := cfg.Environments["dev"]
	if env.SSHUser != "ec2-user" {
		t.Errorf("expected default ssh_user 'ec2-user', got %q", env.SSHUser)
	}
	if env.SSHPort != 22 {
		t.Errorf("expected default ssh_port 22, got %d", env.SSHPort)
	}

	mod := cfg.Modules["api"]
	if mod.DeployDir != "deploy" {
		t.Errorf("expected default deploy_dir 'deploy', got %q", mod.DeployDir)
	}
}

func TestGetServersForModule(t *testing.T) {
	yaml := `
project:
  name: test
  base_dir: /app

environments:
  prod:
    servers:
      - number: 1
        host: 10.0.0.1
        modules: [api]
      - number: 2
        host: 10.0.0.2
        modules: [api, worker]
      - number: 3
        host: 10.0.0.3
        modules: [worker]

modules:
  api:
    type: springboot
    port: 8080
  worker:
    type: java
    port: 8081
`
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// api should be on server 1 and 2
	servers, _, err := cfg.GetServersForModule("prod", "api", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("expected 2 servers for api, got %d", len(servers))
	}

	// worker should be on server 2 and 3
	servers, _, err = cfg.GetServersForModule("prod", "worker", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("expected 2 servers for worker, got %d", len(servers))
	}

	// specific server number
	servers, _, err = cfg.GetServersForModule("prod", "api", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("expected 1 server for api on server 1, got %d", len(servers))
	}

	// non-existent module
	_, _, err = cfg.GetServersForModule("prod", "nonexistent", 0)
	if err == nil {
		t.Fatal("expected error for non-existent module")
	}
}

func TestBranchPolicy(t *testing.T) {
	yaml := `
project:
  name: test
  base_dir: /app

environments:
  prod:
    branch_policy:
      allowed: ["main", "release/*"]
      commands: ["deploy", "auto"]
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := cfg.Environments["prod"]
	if env.BranchPolicy == nil {
		t.Fatal("expected branch_policy to be parsed")
	}
	if len(env.BranchPolicy.Allowed) != 2 {
		t.Errorf("expected 2 allowed branches, got %d", len(env.BranchPolicy.Allowed))
	}
	if len(env.BranchPolicy.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(env.BranchPolicy.Commands))
	}
}

func TestValidateDetailed(t *testing.T) {
	yaml := `
project:
  name: test

environments:
  dev:
    servers:
      - number: 1
        host: ""

modules:
  api:
    type: invalid_type
`
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	cfg := &Config{}
	data, _ := os.ReadFile(tmpFile)
	yamlPkg.Unmarshal(data, cfg)
	cfg.applyDefaults()

	issues := cfg.ValidateDetailed()

	if len(issues) == 0 {
		t.Fatal("expected validation issues")
	}

	// Should find: empty host, invalid type, missing base_dir
	foundEmptyHost := false
	foundInvalidType := false
	for _, issue := range issues {
		if contains(issue, "empty host") {
			foundEmptyHost = true
		}
		if contains(issue, "invalid type") {
			foundInvalidType = true
		}
	}

	if !foundEmptyHost {
		t.Error("expected warning about empty host")
	}
	if !foundInvalidType {
		t.Error("expected warning about invalid type")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Additional Validate Tests ---

func TestValidateNoEnvironments(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Modules: map[string]*Module{"api": {Type: "generic"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for no environments")
	}
	if !contains(err.Error(), "at least one environment") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateNoModules(t *testing.T) {
	cfg := &Config{
		Project:      ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{"dev": {Servers: []Server{{Host: "1.2.3.4"}}}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for no modules")
	}
	if !contains(err.Error(), "at least one module") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateEmptyHost(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"dev": {
				Servers: []Server{{Host: ""}},
			},
		},
		Modules: map[string]*Module{"api": {Type: "generic"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty host")
	}
	if !contains(err.Error(), "no host") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateEmptyHostFromUnresolvedEnvVar(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"prod": {
				Servers: []Server{{Host: "${}"}},
			},
		},
		Modules: map[string]*Module{"api": {Type: "generic"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for unresolved env var host")
	}
}

func TestValidateNoServers(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{}},
		},
		Modules: map[string]*Module{"api": {Type: "generic"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for no servers")
	}
	if !contains(err.Error(), "no servers") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePlaintextPasswordBlocked(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {
				Type: "generic",
				SSH: &SSHConfig{
					Auth:     "password",
					Password: "myplaintextpassword",
				},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for plaintext password")
	}
	if !contains(err.Error(), "plaintext passwords") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateEnvVarPasswordAllowed(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {
				Type: "generic",
				SSH: &SSHConfig{
					Auth:     "password",
					Password: "${MY_PASSWORD}",
				},
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error for env var password, got: %v", err)
	}
}

func TestValidateEmptyModuleType(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {Type: ""},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty module type")
	}
	if !contains(err.Error(), "no type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateSuccessful(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// --- applyDefaults Tests ---

func TestApplyDefaultsAllValues(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"dev": {},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
	}
	cfg.applyDefaults()

	if cfg.Defaults.SSHPort != 22 {
		t.Errorf("expected default SSH port 22, got %d", cfg.Defaults.SSHPort)
	}
	if cfg.Defaults.SSHUser != "ec2-user" {
		t.Errorf("expected default SSH user ec2-user, got %q", cfg.Defaults.SSHUser)
	}
	if cfg.Defaults.DeployDir != "deploy" {
		t.Errorf("expected default deploy dir 'deploy', got %q", cfg.Defaults.DeployDir)
	}
	if cfg.Defaults.HealthCheck.Timeout != 300 {
		t.Errorf("expected default health check timeout 300, got %d", cfg.Defaults.HealthCheck.Timeout)
	}
	if cfg.Defaults.HealthCheck.Interval != 5 {
		t.Errorf("expected default health check interval 5, got %d", cfg.Defaults.HealthCheck.Interval)
	}
	if cfg.Defaults.HealthCheck.Retries != 60 {
		t.Errorf("expected default health check retries 60, got %d", cfg.Defaults.HealthCheck.Retries)
	}
	if cfg.Retention.Keep != 5 {
		t.Errorf("expected default retention keep 5, got %d", cfg.Retention.Keep)
	}

	// Environment defaults
	env := cfg.Environments["dev"]
	if env.SSHUser != "ec2-user" {
		t.Errorf("expected env SSH user 'ec2-user', got %q", env.SSHUser)
	}
	if env.SSHPort != 22 {
		t.Errorf("expected env SSH port 22, got %d", env.SSHPort)
	}
}

func TestApplyDefaultsDoesNotOverrideExisting(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{
			SSHPort:    2222,
			SSHUser:    "deploy",
			SSHKeyPath: "/custom/key",
			DeployDir:  "releases",
			HealthCheck: HealthCheckConfig{
				Timeout:  60,
				Interval: 10,
				Retries:  6,
			},
		},
		Retention: RetentionConfig{Keep: 10},
		Environments: map[string]*Environment{
			"dev": {
				SSHUser:    "ubuntu",
				SSHPort:    3333,
				SSHKeyPath: "/dev/key",
			},
		},
		Modules: map[string]*Module{},
	}
	cfg.applyDefaults()

	if cfg.Defaults.SSHPort != 2222 {
		t.Errorf("should not override existing SSH port, got %d", cfg.Defaults.SSHPort)
	}
	if cfg.Defaults.SSHUser != "deploy" {
		t.Errorf("should not override existing SSH user, got %q", cfg.Defaults.SSHUser)
	}
	if cfg.Retention.Keep != 10 {
		t.Errorf("should not override existing retention keep, got %d", cfg.Retention.Keep)
	}

	env := cfg.Environments["dev"]
	if env.SSHUser != "ubuntu" {
		t.Errorf("should not override env SSH user, got %q", env.SSHUser)
	}
	if env.SSHPort != 3333 {
		t.Errorf("should not override env SSH port, got %d", env.SSHPort)
	}
}

func TestApplyDefaultsBaseDir(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{BaseDir: "/custom"},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
		Environments: map[string]*Environment{},
	}
	cfg.applyDefaults()

	// Module deploy dir should be set from defaults
	if cfg.Modules["api"].DeployDir != "deploy" {
		t.Errorf("expected deploy dir 'deploy', got %q", cfg.Modules["api"].DeployDir)
	}
}

// --- GetServersForModule edge cases ---

func TestGetServersForModuleNonExistentEnv(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{"api": {Type: "generic"}},
	}
	_, _, err := cfg.GetServersForModule("staging", "api", 0)
	if err == nil {
		t.Fatal("expected error for non-existent environment")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetServersForModuleNoMatchingServer(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"dev": {
				Servers: []Server{
					{Number: 1, Host: "10.0.0.1", Modules: []string{"worker"}},
				},
			},
		},
		Modules: map[string]*Module{
			"api":    {Type: "generic"},
			"worker": {Type: "generic"},
		},
	}
	_, _, err := cfg.GetServersForModule("dev", "api", 0)
	if err == nil {
		t.Fatal("expected error when no servers match module")
	}
	if !contains(err.Error(), "no servers found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetServersForModuleAllModules(t *testing.T) {
	// Servers with empty Modules list should match all modules
	cfg := &Config{
		Environments: map[string]*Environment{
			"dev": {
				Servers: []Server{
					{Number: 1, Host: "10.0.0.1"}, // no modules restriction
				},
			},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
	}
	servers, _, err := cfg.GetServersForModule("dev", "api", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(servers))
	}
}

func TestGetServersForModuleByNumber(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"prod": {
				Servers: []Server{
					{Number: 1, Host: "10.0.0.1"},
					{Number: 2, Host: "10.0.0.2"},
					{Number: 3, Host: "10.0.0.3"},
				},
			},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic"},
		},
	}

	// Specific server
	servers, _, err := cfg.GetServersForModule("prod", "api", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 || servers[0].Number != 2 {
		t.Errorf("expected server 2, got %v", servers)
	}

	// Non-existent server number
	_, _, err = cfg.GetServersForModule("prod", "api", 99)
	if err == nil {
		t.Fatal("expected error for non-existent server number")
	}
}

// --- GetConfigPath Tests ---

func TestGetConfigPathNilModule(t *testing.T) {
	cfg := &Config{
		Modules: map[string]*Module{},
	}
	result := cfg.GetConfigPath("nonexistent", "dev", 1)
	if result != "" {
		t.Errorf("expected empty for nil module, got %q", result)
	}
}

func TestGetConfigPathEmptyConfigDir(t *testing.T) {
	cfg := &Config{
		Modules: map[string]*Module{
			"api": {ConfigDir: ""},
		},
	}
	result := cfg.GetConfigPath("api", "dev", 1)
	if result != "" {
		t.Errorf("expected empty for empty config dir, got %q", result)
	}
}

func TestGetConfigPathFallback(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Modules: map[string]*Module{
			"api": {ConfigDir: tmpDir},
		},
	}
	result := cfg.GetConfigPath("api", "dev", 1)
	if result != tmpDir {
		t.Errorf("expected fallback to base config dir %q, got %q", tmpDir, result)
	}
}

func TestGetConfigPathEnvSpecific(t *testing.T) {
	tmpDir := t.TempDir()
	envDir := filepath.Join(tmpDir, "prod")
	os.MkdirAll(envDir, 0755)

	cfg := &Config{
		Modules: map[string]*Module{
			"api": {ConfigDir: tmpDir},
		},
	}
	result := cfg.GetConfigPath("api", "prod", 0)
	if result != envDir {
		t.Errorf("expected env-specific dir %q, got %q", envDir, result)
	}
}

func TestGetConfigPathServerSpecific(t *testing.T) {
	tmpDir := t.TempDir()
	specificDir := filepath.Join(tmpDir, "prod-1")
	os.MkdirAll(specificDir, 0755)

	cfg := &Config{
		Modules: map[string]*Module{
			"api": {ConfigDir: tmpDir},
		},
	}
	result := cfg.GetConfigPath("api", "prod", 1)
	if result != specificDir {
		t.Errorf("expected server-specific dir %q, got %q", specificDir, result)
	}
}

func TestGetConfigPathServerSpecificOverridesEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envDir := filepath.Join(tmpDir, "prod")
	specificDir := filepath.Join(tmpDir, "prod-2")
	os.MkdirAll(envDir, 0755)
	os.MkdirAll(specificDir, 0755)

	cfg := &Config{
		Modules: map[string]*Module{
			"api": {ConfigDir: tmpDir},
		},
	}
	result := cfg.GetConfigPath("api", "prod", 2)
	if result != specificDir {
		t.Errorf("expected server-specific dir to override env dir: got %q", result)
	}
}

// --- Summary Tests ---

func TestSummary(t *testing.T) {
	cfg := &Config{
		Environments: map[string]*Environment{
			"dev": {
				Servers: []Server{{Host: "1"}, {Host: "2"}},
			},
			"prod": {
				Servers: []Server{{Host: "3"}},
			},
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

func TestSummaryEmpty(t *testing.T) {
	cfg := &Config{}
	envCount, moduleCount, serverCount := cfg.Summary()
	if envCount != 0 || moduleCount != 0 || serverCount != 0 {
		t.Errorf("expected all zeros, got %d %d %d", envCount, moduleCount, serverCount)
	}
}

// --- ValidateDetailed Additional Tests ---

func TestValidateDetailedNoProjectName(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	foundProjectName := false
	for _, issue := range issues {
		if contains(issue, "project.name") {
			foundProjectName = true
		}
	}
	if !foundProjectName {
		t.Error("expected issue about project.name")
	}
}

func TestValidateDetailedNoBaseDir(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test"},
	}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	foundBaseDir := false
	for _, issue := range issues {
		if contains(issue, "base_dir") {
			foundBaseDir = true
		}
	}
	if !foundBaseDir {
		t.Error("expected issue about base_dir")
	}
}

func TestValidateDetailedNoEnvironments(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test", BaseDir: "/app"},
	}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	found := false
	for _, issue := range issues {
		if contains(issue, "at least one environment") {
			found = true
		}
	}
	if found == false {
		t.Error("expected issue about environments")
	}
}

func TestValidateDetailedNoModules(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test", BaseDir: "/app"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
	}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	found := false
	for _, issue := range issues {
		if contains(issue, "at least one module") {
			found = true
		}
	}
	if !found {
		t.Error("expected issue about modules")
	}
}

func TestValidateDetailedInvalidModuleType(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test", BaseDir: "/app"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {Type: "totally_invalid"},
		},
	}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	found := false
	for _, issue := range issues {
		if contains(issue, "invalid type") {
			found = true
		}
	}
	if !found {
		t.Error("expected issue about invalid type")
	}
}

func TestValidateDetailedConfigDirNotExist(t *testing.T) {
	cfg := &Config{
		Project: ProjectConfig{Name: "test", BaseDir: "/app"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic", ConfigDir: "/nonexistent/config/dir"},
		},
	}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	found := false
	for _, issue := range issues {
		if contains(issue, "config_dir does not exist") {
			found = true
		}
	}
	if !found {
		t.Error("expected issue about config_dir not existing")
	}
}

func TestValidateDetailedConfigDirIsFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	os.WriteFile(tmpFile, []byte("x"), 0644)

	cfg := &Config{
		Project: ProjectConfig{Name: "test", BaseDir: "/app"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic", ConfigDir: tmpFile},
		},
	}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	found := false
	for _, issue := range issues {
		if contains(issue, "not a directory") {
			found = true
		}
	}
	if !found {
		t.Error("expected issue about config_dir not being a directory")
	}
}

func TestValidateDetailedValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		Project: ProjectConfig{Name: "test", BaseDir: "/app"},
		Environments: map[string]*Environment{
			"dev": {Servers: []Server{{Host: "10.0.0.1"}}},
		},
		Modules: map[string]*Module{
			"api": {Type: "generic", ConfigDir: tmpDir},
		},
	}
	cfg.applyDefaults()
	issues := cfg.ValidateDetailed()

	// Should have no issues (or only about SSH key)
	for _, issue := range issues {
		if contains(issue, "required") || contains(issue, "invalid type") || contains(issue, "empty host") {
			t.Errorf("unexpected critical issue: %s", issue)
		}
	}
}

// --- Load Edge Cases ---

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/tow.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte("invalid: [yaml: broken"), 0644)

	_, err := Load(tmpFile)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadWithEnvExpansion(t *testing.T) {
	t.Setenv("TEST_HOST", "10.0.0.99")

	yaml := `
project:
  name: test-app
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: ${TEST_HOST}

modules:
  api:
    type: generic
`
	tmpFile := filepath.Join(t.TempDir(), "tow.yaml")
	os.WriteFile(tmpFile, []byte(yaml), 0644)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	host := cfg.Environments["dev"].Servers[0].Host
	if host != "10.0.0.99" {
		t.Errorf("expected expanded host '10.0.0.99', got %q", host)
	}
}

// --- Merge Local Config Tests ---

func TestMergeLocalOverridesProject(t *testing.T) {
	dir := t.TempDir()

	mainYaml := `
project:
  name: original
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	localYaml := `
project:
  name: overridden
  version: "2.0"
  base_dir: /custom
`
	os.WriteFile(filepath.Join(dir, "tow.yaml"), []byte(mainYaml), 0644)
	os.WriteFile(filepath.Join(dir, "tow.local.yaml"), []byte(localYaml), 0644)

	cfg, err := Load(filepath.Join(dir, "tow.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project.Name != "overridden" {
		t.Errorf("expected overridden name, got %q", cfg.Project.Name)
	}
	if cfg.Project.Version != "2.0" {
		t.Errorf("expected version 2.0, got %q", cfg.Project.Version)
	}
	if cfg.Project.BaseDir != "/custom" {
		t.Errorf("expected /custom base_dir, got %q", cfg.Project.BaseDir)
	}
}

func TestMergeLocalOverridesDefaults(t *testing.T) {
	dir := t.TempDir()

	mainYaml := `
project:
  name: test
  base_dir: /app

defaults:
  ssh_user: ec2-user
  ssh_port: 22

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	localYaml := `
defaults:
  ssh_user: deploy
  ssh_port: 2222
  deploy_dir: releases
`
	os.WriteFile(filepath.Join(dir, "tow.yaml"), []byte(mainYaml), 0644)
	os.WriteFile(filepath.Join(dir, "tow.local.yaml"), []byte(localYaml), 0644)

	cfg, err := Load(filepath.Join(dir, "tow.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Defaults.SSHUser != "deploy" {
		t.Errorf("expected 'deploy', got %q", cfg.Defaults.SSHUser)
	}
	if cfg.Defaults.SSHPort != 2222 {
		t.Errorf("expected 2222, got %d", cfg.Defaults.SSHPort)
	}
}

func TestMergeLocalAddsNewEnvironment(t *testing.T) {
	dir := t.TempDir()

	mainYaml := `
project:
  name: test
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	localYaml := `
environments:
  staging:
    servers:
      - number: 1
        host: 10.0.1.1
`
	os.WriteFile(filepath.Join(dir, "tow.yaml"), []byte(mainYaml), 0644)
	os.WriteFile(filepath.Join(dir, "tow.local.yaml"), []byte(localYaml), 0644)

	cfg, err := Load(filepath.Join(dir, "tow.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := cfg.Environments["staging"]; !ok {
		t.Error("expected staging environment to be added")
	}
}

func TestMergeLocalMergesExistingEnvironment(t *testing.T) {
	dir := t.TempDir()

	mainYaml := `
project:
  name: test
  base_dir: /app

environments:
  dev:
    ssh_user: ec2-user
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	localYaml := `
environments:
  dev:
    ssh_user: ubuntu
    branch: develop
    variables:
      EXTRA: value
`
	os.WriteFile(filepath.Join(dir, "tow.yaml"), []byte(mainYaml), 0644)
	os.WriteFile(filepath.Join(dir, "tow.local.yaml"), []byte(localYaml), 0644)

	cfg, err := Load(filepath.Join(dir, "tow.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := cfg.Environments["dev"]
	if env.SSHUser != "ubuntu" {
		t.Errorf("expected ubuntu, got %q", env.SSHUser)
	}
	if env.Branch != "develop" {
		t.Errorf("expected develop, got %q", env.Branch)
	}
	if env.Variables["EXTRA"] != "value" {
		t.Errorf("expected variable EXTRA=value")
	}
}

func TestMergeLocalAddsNewModule(t *testing.T) {
	dir := t.TempDir()

	mainYaml := `
project:
  name: test
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	localYaml := `
modules:
  worker:
    type: python
    port: 9000
`
	os.WriteFile(filepath.Join(dir, "tow.yaml"), []byte(mainYaml), 0644)
	os.WriteFile(filepath.Join(dir, "tow.local.yaml"), []byte(localYaml), 0644)

	cfg, err := Load(filepath.Join(dir, "tow.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := cfg.Modules["worker"]; !ok {
		t.Error("expected worker module to be added")
	}
	if cfg.Modules["worker"].Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Modules["worker"].Port)
	}
}

func TestMergeLocalMergesExistingModule(t *testing.T) {
	dir := t.TempDir()

	mainYaml := `
project:
  name: test
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: springboot
    port: 8080
`
	localYaml := `
modules:
  api:
    port: 9090
    build_cmd: "custom build"
    variables:
      KEY: value
`
	os.WriteFile(filepath.Join(dir, "tow.yaml"), []byte(mainYaml), 0644)
	os.WriteFile(filepath.Join(dir, "tow.local.yaml"), []byte(localYaml), 0644)

	cfg, err := Load(filepath.Join(dir, "tow.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mod := cfg.Modules["api"]
	if mod.Port != 9090 {
		t.Errorf("expected port 9090, got %d", mod.Port)
	}
	if mod.BuildCmd != "custom build" {
		t.Errorf("expected 'custom build', got %q", mod.BuildCmd)
	}
	if mod.Type != "springboot" {
		t.Errorf("expected type still springboot, got %q", mod.Type)
	}
	if mod.Variables["KEY"] != "value" {
		t.Error("expected variable KEY=value")
	}
}

func TestNoLocalFile(t *testing.T) {
	dir := t.TempDir()

	mainYaml := `
project:
  name: test
  base_dir: /app

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.0.1

modules:
  api:
    type: generic
`
	os.WriteFile(filepath.Join(dir, "tow.yaml"), []byte(mainYaml), 0644)
	// No local file

	cfg, err := Load(filepath.Join(dir, "tow.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project.Name != "test" {
		t.Errorf("expected 'test', got %q", cfg.Project.Name)
	}
}
