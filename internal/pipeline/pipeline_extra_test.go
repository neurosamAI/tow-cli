package pipeline

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestDeployWithMockExecutor(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Modules["api"].ArtifactPath = "build/api.tar.gz"
	cfg.Retention.AutoCleanup = false

	mock := &ssh.MockExecutor{DryRun: true}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "ln -s") {
			return &ssh.ExecResult{Host: host, Stdout: "DEPLOY_OK\n"}, nil
		}
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		if strings.Contains(command, "CLEANUP_DONE") {
			return &ssh.ExecResult{Host: host, Stdout: "CLEANUP_DONE removed=0\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}

	p := NewWithExecutor(cfg, mock)

	err := p.Deploy("dev", "api", 0)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	cmds := mock.GetCommands()
	if len(cmds) == 0 {
		t.Fatal("expected commands to be executed")
	}
}

func TestDeployWithAutoCleanup(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Retention.AutoCleanup = true
	cfg.Retention.Keep = 3

	mock := &ssh.MockExecutor{DryRun: true}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "ln -s") {
			return &ssh.ExecResult{Host: host, Stdout: "DEPLOY_OK\n"}, nil
		}
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		if strings.Contains(command, "CLEANUP_DONE") {
			return &ssh.ExecResult{Host: host, Stdout: "CLEANUP_DONE removed=1\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}

	p := NewWithExecutor(cfg, mock)

	err := p.Deploy("dev", "api", 0)
	if err != nil {
		t.Fatalf("Deploy with auto cleanup failed: %v", err)
	}

	// Verify cleanup command was issued
	cmds := mock.GetCommands()
	foundCleanup := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "CLEANUP_DONE") {
			foundCleanup = true
			break
		}
	}
	if !foundCleanup {
		t.Error("expected cleanup command to be executed")
	}
}

func TestAutoWithMockExecutor(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Retention.AutoCleanup = false

	mock := &ssh.MockExecutor{DryRun: true}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "ln -s") {
			return &ssh.ExecResult{Host: host, Stdout: "DEPLOY_OK\n"}, nil
		}
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}

	p := NewWithExecutor(cfg, mock)

	err := p.Auto("dev", "api", 0)
	if err != nil {
		t.Fatalf("Auto failed: %v", err)
	}
}

func TestAutoWithRollbackSuccess(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Retention.AutoCleanup = false

	mock := &ssh.MockExecutor{DryRun: true}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "ln -s") {
			return &ssh.ExecResult{Host: host, Stdout: "DEPLOY_OK\n"}, nil
		}
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}

	p := NewWithExecutor(cfg, mock)

	err := p.AutoWithRollback("dev", "api", 0)
	if err != nil {
		t.Fatalf("AutoWithRollback failed: %v", err)
	}
}

func TestAutoWithRollbackOnStartFailure(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Retention.AutoCleanup = false

	startAttempt := 0
	mock := &ssh.MockExecutor{DryRun: true}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "ln -s") && strings.Contains(command, "DEPLOY_OK") {
			return &ssh.ExecResult{Host: host, Stdout: "DEPLOY_OK\n"}, nil
		}
		// Rollback ln -s command
		if strings.Contains(command, "ln -s") && strings.Contains(command, "ROLLBACK_OK") {
			return &ssh.ExecResult{Host: host, Stdout: "ROLLBACK_OK from a to b\n"}, nil
		}
		if strings.Contains(command, "start.sh") {
			startAttempt++
			if startAttempt == 1 {
				// First start (deploy) fails
				return &ssh.ExecResult{Host: host, Stdout: "OK\n", ExitCode: 1, Stderr: "start failed"}, nil
			}
			return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
		}
		if strings.Contains(command, "nc -z") {
			if startAttempt <= 1 {
				return &ssh.ExecResult{Host: host, Stdout: "FAILED\n"}, nil
			}
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}

	p := NewWithExecutor(cfg, mock)

	err := p.AutoWithRollback("dev", "api", 0)
	// Should fail but with rollback message
	if err == nil {
		t.Fatal("expected error from AutoWithRollback when start fails")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("expected 'rolled back' in error, got: %v", err)
	}
}

func TestDeployRolling(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := pipelineConfig()
	cfg.Retention.AutoCleanup = false

	mock := &ssh.MockExecutor{DryRun: true}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "ln -s") {
			return &ssh.ExecResult{Host: host, Stdout: "DEPLOY_OK\n"}, nil
		}
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}

	p := NewWithExecutor(cfg, mock)
	p.Rolling = true

	err := p.Deploy("dev", "api", 0)
	if err != nil {
		t.Fatalf("Deploy with rolling failed: %v", err)
	}
}

func TestPackageWithLayout(t *testing.T) {
	SetDryRun(false) // actually run cp/tar

	tmpDir := t.TempDir()

	// Create source files
	binDir := filepath.Join(tmpDir, "script")
	os.MkdirAll(binDir, 0755)
	os.WriteFile(filepath.Join(binDir, "start.sh"), []byte("#!/bin/bash\necho start"), 0755)
	os.WriteFile(filepath.Join(binDir, "stop.sh"), []byte("#!/bin/bash\necho stop"), 0755)

	libDir := filepath.Join(tmpDir, "build", "libs")
	os.MkdirAll(libDir, 0755)
	os.WriteFile(filepath.Join(libDir, "api.jar"), []byte("fake-jar-content"), 0644)

	artifactPath := filepath.Join(tmpDir, "build", "api.tar.gz")

	cfg := pipelineConfig()
	cfg.Modules["api"].PackageLayout = map[string]string{
		filepath.Join(tmpDir, "script") + "/":        "bin/",
		filepath.Join(tmpDir, "build", "libs") + "/": "lib/",
	}
	cfg.Modules["api"].ArtifactPath = artifactPath

	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package with layout failed: %v", err)
	}

	// Verify artifact was created
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Fatal("expected artifact file to be created")
	}

	// Verify tar contents by extracting
	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)
	extractCmd := exec.Command("tar", "xzf", artifactPath, "-C", extractDir)
	if err := extractCmd.Run(); err != nil {
		t.Fatalf("extracting tar: %v", err)
	}

	// Check that bin/ and lib/ directories exist in the extracted archive
	if _, err := os.Stat(filepath.Join(extractDir, "bin")); os.IsNotExist(err) {
		t.Error("expected bin/ directory in tar archive")
	}
	if _, err := os.Stat(filepath.Join(extractDir, "lib")); os.IsNotExist(err) {
		t.Error("expected lib/ directory in tar archive")
	}
}

func TestPackageWithLayoutModuleSubstitution(t *testing.T) {
	SetDryRun(false)

	tmpDir := t.TempDir()

	// Create source files using ${MODULE} substitution
	modDir := filepath.Join(tmpDir, "api")
	os.MkdirAll(filepath.Join(modDir, "build"), 0755)
	os.WriteFile(filepath.Join(modDir, "build", "app.jar"), []byte("jar-content"), 0644)

	artifactPath := filepath.Join(tmpDir, "build", "api.tar.gz")
	os.MkdirAll(filepath.Join(tmpDir, "build"), 0755)

	cfg := pipelineConfig()
	cfg.Modules["api"].PackageLayout = map[string]string{
		filepath.Join(tmpDir, "${MODULE}", "build") + "/": "lib/",
	}
	cfg.Modules["api"].ArtifactPath = artifactPath

	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package with layout module substitution failed: %v", err)
	}

	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Fatal("expected artifact file to be created")
	}
}

func TestPackageWithLayoutEnvSubstitution(t *testing.T) {
	SetDryRun(false)

	tmpDir := t.TempDir()

	// Create config/dev/ directory
	configDir := filepath.Join(tmpDir, "config", "dev")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "app.yml"), []byte("server:\n  port: 8080"), 0644)

	artifactPath := filepath.Join(tmpDir, "build", "api.tar.gz")
	os.MkdirAll(filepath.Join(tmpDir, "build"), 0755)

	cfg := pipelineConfig()
	cfg.Modules["api"].PackageLayout = map[string]string{
		filepath.Join(tmpDir, "config", "${ENV}") + "/": "conf/",
	}
	cfg.Modules["api"].ArtifactPath = artifactPath

	sshMgr := ssh.NewManager(false)
	sshMgr.DryRun = true
	p := New(cfg, sshMgr)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("Package with layout env substitution failed: %v", err)
	}

	// Verify artifact was created
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Fatal("expected artifact file to be created")
	}

	// Extract and verify conf/ directory
	extractDir := filepath.Join(tmpDir, "extract")
	os.MkdirAll(extractDir, 0755)
	extractCmd := exec.Command("tar", "xzf", artifactPath, "-C", extractDir)
	if err := extractCmd.Run(); err != nil {
		t.Fatalf("extracting tar: %v", err)
	}

	if _, err := os.Stat(filepath.Join(extractDir, "conf")); os.IsNotExist(err) {
		t.Error("expected conf/ directory in tar archive after ${ENV} substitution")
	}
}
