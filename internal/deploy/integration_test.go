package deploy

import (
	"strings"
	"testing"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/ssh"
)

// integrationConfig returns a config suitable for integration tests with MockExecutor.
func integrationConfig() *config.Config {
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
			"api-server": {
				Type:         "springboot",
				Port:         8080,
				ArtifactPath: "build/api-server.tar.gz",
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
			Keep:        5,
			AutoCleanup: true,
		},
	}
}

func TestInitCreatesDirectories(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	err := d.Init("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	cmds := mock.GetCommands()
	if len(cmds) == 0 {
		t.Fatal("expected at least one command, got none")
	}

	mkdirCmd := cmds[0]
	if !strings.HasPrefix(mkdirCmd, "mkdir -p ") {
		t.Errorf("expected mkdir -p command, got: %s", mkdirCmd)
	}

	for _, dir := range []string{"/app/api-server", "upload", "deploy", "logs", "conf"} {
		if !strings.Contains(mkdirCmd, dir) {
			t.Errorf("expected mkdir command to contain %q, got: %s", dir, mkdirCmd)
		}
	}
}

func TestUploadTransfersFile(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{DryRun: true}
	d := New(cfg, mock)

	// In dry-run mode, artifact file doesn't need to exist
	err := d.Upload("dev", "api-server", 0, "")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
}

func TestInstallCreatesSymlink(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		// Return DEPLOY_OK for install commands
		if strings.Contains(command, "ln -s") {
			return &ssh.ExecResult{Host: host, Stdout: "DEPLOY_OK\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Install("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundSymlink := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "ln -s") && strings.Contains(cmd, "current") {
			foundSymlink = true
			break
		}
	}
	if !foundSymlink {
		t.Errorf("expected a symlink command creating 'current', commands were:\n%s", strings.Join(cmds, "\n"))
	}
}

func TestStartRunsCommand(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		// For health check, return HEALTHY
		if strings.Contains(command, "HEALTHY") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Start("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundStart := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "start.sh") {
			foundStart = true
			break
		}
	}
	if !foundStart {
		t.Errorf("expected a start command, commands were:\n%s", strings.Join(cmds, "\n"))
	}
}

func TestStopRunsCommand(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Stop("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundStop := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "stop.sh") {
			foundStop = true
			break
		}
	}
	if !foundStop {
		t.Errorf("expected a stop command, commands were:\n%s", strings.Join(cmds, "\n"))
	}
}

func TestStatusChecksPort(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "lsof") || strings.Contains(command, "8080") {
			return &ssh.ExecResult{Host: host, Stdout: "RUNNING pid=1234 uptime=01:23 mem=512000KB\ndeployment=deploy/20260401-120000\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Status("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundPortCheck := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "8080") {
			foundPortCheck = true
			break
		}
	}
	if !foundPortCheck {
		t.Errorf("expected a port check command for 8080, commands were:\n%s", strings.Join(cmds, "\n"))
	}
}

func TestRollbackSwitchesSymlink(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "ln -s") && strings.Contains(command, "ROLLBACK_OK") {
			return &ssh.ExecResult{Host: host, Stdout: "ROLLBACK_OK from 20260401-120000 to 20260331-100000\n"}, nil
		}
		// For restart after rollback — start/stop
		if strings.Contains(command, "HEALTHY") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Rollback("dev", "api-server", 0, "")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundSymlink := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "ln -s") && strings.Contains(cmd, "current") {
			foundSymlink = true
			break
		}
	}
	if !foundSymlink {
		t.Errorf("expected a symlink switch command, commands were:\n%s", strings.Join(cmds, "\n"))
	}
}

func TestCleanupRemovesOld(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "CLEANUP_DONE") {
			return &ssh.ExecResult{Host: host, Stdout: "removed: 20260101-000000\nCLEANUP_DONE removed=1\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Cleanup("dev", "api-server", 0, 3)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundCleanup := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "rm -rf") && strings.Contains(cmd, "deploy") {
			foundCleanup = true
			break
		}
	}
	if !foundCleanup {
		t.Errorf("expected a cleanup command with rm -rf, commands were:\n%s", strings.Join(cmds, "\n"))
	}
}
