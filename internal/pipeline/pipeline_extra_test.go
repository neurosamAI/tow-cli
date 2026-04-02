package pipeline

import (
	"strings"
	"testing"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/ssh"
)

// pipelineConfig returns a minimal config for pipeline tests
func pipelineConfig() *config.Config {
	return &config.Config{
		Project: config.ProjectConfig{
			Name:    "test-project",
			BaseDir: "/app",
		},
		Environments: map[string]*config.Environment{
			"dev": {
				SSHUser:    "testuser",
				SSHPort:    22,
				SSHKeyPath: "~/.ssh/test.pem",
				Servers: []config.Server{
					{Number: 1, Host: "10.0.1.10"},
				},
			},
		},
		Modules: map[string]*config.Module{
			"api": {
				Type:         "springboot",
				Port:         8080,
				BuildCmd:     "echo build",
				ArtifactPath: "build/api.tar.gz",
				StartCmd:     "./bin/start.sh",
				StopCmd:      "./bin/stop.sh",
				HealthCheck: config.HealthCheckConfig{
					Type:     "tcp",
					Target:   ":8080",
					Timeout:  2,
					Interval: 1,
					Retries:  1,
				},
			},
		},
		Retention: config.RetentionConfig{
			Keep:        3,
			AutoCleanup: false,
		},
	}
}

func TestBuildWithHooksExecution(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].Hooks = config.HooksConfig{
		PreBuild:  "echo pre-build",
		PostBuild: "echo post-build",
	}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "dev")
	if err != nil {
		t.Fatalf("Build with hooks failed: %v", err)
	}
}

func TestBuildWithPreBuildHookFailure(t *testing.T) {
	SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].Hooks = config.HooksConfig{
		PreBuild: "false", // exit code 1
	}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "dev")
	if err == nil {
		t.Fatal("expected error when pre-build hook fails")
	}
	if !strings.Contains(err.Error(), "pre-build hook") {
		t.Errorf("expected pre-build hook error, got: %v", err)
	}
}

func TestPackageCreationDryRun(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].ArtifactPath = "build/api.tar.gz"
	cfg.Modules["api"].PackageIncludes = []string{"config/", "scripts/"}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package dry-run failed: %v", err)
	}
}

func TestPackageNoIncludes(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	// Use a type that won't have handler-specific includes
	cfg.Modules["api"].Type = "unknown_no_handler"
	cfg.Modules["api"].PackageIncludes = nil
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	// Should succeed with warning about no files to package
	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package with no includes should not fail: %v", err)
	}
}

func TestBuildModuleNotFoundInPipeline(t *testing.T) {
	cfg := pipelineConfig()
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("nonexistent", "dev")
	if err == nil {
		t.Fatal("expected error for nonexistent module")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestPackageModuleNotFoundInPipeline(t *testing.T) {
	cfg := pipelineConfig()
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("nonexistent", "dev")
	if err == nil {
		t.Fatal("expected error for nonexistent module")
	}
}

func TestBuildNoBuildCommandInPipeline(t *testing.T) {
	cfg := pipelineConfig()
	cfg.Modules["api"].BuildCmd = ""
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "dev")
	if err != nil {
		t.Fatalf("expected no error when no build cmd: %v", err)
	}
}

func TestBuildWithVariableSubstitution(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].BuildCmd = "./gradlew :${MODULE}:bootJar -Pprofile=${ENV}"
	cfg.Modules["api"].Variables = map[string]string{
		"CUSTOM": "value",
	}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "prod")
	if err != nil {
		t.Fatalf("Build with variables failed: %v", err)
	}
}

func TestRunLocalCmdActualExecution(t *testing.T) {
	SetDryRun(false)

	err := runLocalCmd("echo hello")
	if err != nil {
		t.Fatalf("expected echo to succeed: %v", err)
	}
}

func TestRunLocalCmdFailure(t *testing.T) {
	SetDryRun(false)

	err := runLocalCmd("false")
	if err == nil {
		t.Fatal("expected error from 'false' command")
	}
}

func TestPipelineRollingFlag(t *testing.T) {
	cfg := pipelineConfig()
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	if p.Rolling {
		t.Error("expected Rolling to be false by default")
	}
	p.Rolling = true
	if !p.Rolling {
		t.Error("expected Rolling to be true after setting")
	}
}

func TestBuildWithPostBuildHookFailure(t *testing.T) {
	SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].BuildCmd = "echo build"
	cfg.Modules["api"].Hooks = config.HooksConfig{
		PostBuild: "false", // exit code 1
	}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "dev")
	if err == nil {
		t.Fatal("expected error when post-build hook fails")
	}
	if !strings.Contains(err.Error(), "post-build hook") {
		t.Errorf("expected post-build hook error, got: %v", err)
	}
}

func TestBuildWithEmptyHooks(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].Hooks = config.HooksConfig{} // empty hooks
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "dev")
	if err != nil {
		t.Fatalf("Build with empty hooks should succeed: %v", err)
	}
}

func TestPackageWithConfigDir(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	// Create a temp config dir
	tmpDir := t.TempDir()
	cfg.Modules["api"].ConfigDir = tmpDir
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package with config dir should succeed: %v", err)
	}
}

func TestRunStepsSingleStep(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", BaseDir: "/app"},
	}
	p := New(cfg, nil)

	called := false
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Only", func() error { called = true; return nil }},
	}

	err := p.runSteps(steps)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if !called {
		t.Error("expected step to be called")
	}
}

func TestRunStepsFirstFails(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", BaseDir: "/app"},
	}
	p := New(cfg, nil)

	secondCalled := false
	steps := []struct {
		name string
		fn   func() error
	}{
		{"First", func() error { return strings.NewReader("").UnreadByte() }},
		{"Second", func() error { secondCalled = true; return nil }},
	}

	err := p.runSteps(steps)
	if err == nil {
		t.Fatal("expected error when first step fails")
	}
	if secondCalled {
		t.Error("second step should not run when first fails")
	}
}

func TestPackageGenericModuleType(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].Type = "generic"
	cfg.Modules["api"].ArtifactPath = "build/api.tar.gz"
	cfg.Modules["api"].PackageIncludes = []string{"bin/", "lib/"}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package generic type failed: %v", err)
	}
}

func TestPackageDefaultArtifactPath(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].ArtifactPath = "" // should use default
	cfg.Modules["api"].PackageIncludes = []string{"app/"}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package with default artifact path failed: %v", err)
	}
}

func TestPackageJavaSpringBoot(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].Type = "springboot"
	cfg.Modules["api"].ArtifactPath = "build/api.tar.gz"
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package springboot type failed: %v", err)
	}
}

func TestBuildActualCommandSuccess(t *testing.T) {
	SetDryRun(false) // actually run the command

	cfg := pipelineConfig()
	cfg.Modules["api"].BuildCmd = "true" // always succeeds
	cfg.Modules["api"].Hooks = config.HooksConfig{}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "dev")
	if err != nil {
		t.Fatalf("Build with 'true' command should succeed: %v", err)
	}
}

func TestBuildActualCommandFailure(t *testing.T) {
	SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].BuildCmd = "false" // always fails
	cfg.Modules["api"].Hooks = config.HooksConfig{}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Build("api", "dev")
	if err == nil {
		t.Fatal("expected error from failed build")
	}
	if !strings.Contains(err.Error(), "build failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPackageActualCommandFailure(t *testing.T) {
	SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].ArtifactPath = "/nonexistent/path/artifact.tar.gz"
	cfg.Modules["api"].PackageIncludes = []string{"/also/nonexistent"}
	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err == nil {
		t.Fatal("expected error from failed package command")
	}
	if !strings.Contains(err.Error(), "packaging failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSubstituteVarsWithAllPlaceholders(t *testing.T) {
	vars := map[string]string{
		"PROFILE": "production",
		"PORT":    "8080",
	}
	cmd := "${ENV}-${MODULE}-${PROFILE}:${PORT}"
	result := substituteVars(cmd, "prod", "api", vars)
	expected := "prod-api-production:8080"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
