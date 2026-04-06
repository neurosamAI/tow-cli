package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/ssh"
)

func TestLogsReadsFromServer(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "tail") {
			return &ssh.ExecResult{
				Host:   host,
				Stdout: "2026-04-01 12:00:00 INFO Application started\n2026-04-01 12:00:01 INFO Ready\n",
			}, nil
		}
		// For resolveLogPath detection command
		return &ssh.ExecResult{Host: host, Stdout: "/app/api-server/log/std.log\n"}, nil
	}
	d := New(cfg, mock)

	// Should not error
	err := d.Logs("dev", "api-server", 1, "", 50, false)
	if err != nil {
		t.Fatalf("Logs failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundTail := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "tail") {
			foundTail = true
			break
		}
	}
	if !foundTail {
		t.Errorf("expected tail command, commands were:\n%s", strings.Join(cmds, "\n"))
	}
}

func TestLogsSingleServer(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "log line 1\nlog line 2\n"}, nil
	}
	d := New(cfg, mock)

	// Single server path (only 1 server in integrationConfig)
	err := d.Logs("dev", "api-server", 0, "", 100, false)
	if err != nil {
		t.Fatalf("Logs single server failed: %v", err)
	}
}

func TestLogsWithFilter(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "filtered output\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Logs("dev", "api-server", 1, "ERROR", 50, false)
	if err != nil {
		t.Fatalf("Logs with filter failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundGrep := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "grep") && strings.Contains(cmd, "ERROR") {
			foundGrep = true
			break
		}
	}
	if !foundGrep {
		t.Errorf("expected grep with filter, commands were:\n%s", strings.Join(cmds, "\n"))
	}
}

func TestDownloadCallsSCP(t *testing.T) {
	cfg := integrationConfig()
	downloadCalled := false
	mock := &ssh.MockExecutor{}
	mock.DownloadFn = func(env *config.Environment, host, remotePath, localDir string) error {
		downloadCalled = true
		if !strings.Contains(remotePath, "/app/api-server") {
			t.Errorf("expected remote path under /app/api-server, got %q", remotePath)
		}
		return nil
	}
	d := New(cfg, mock)

	tmpDir := t.TempDir()
	err := d.Download("dev", "api-server", 1, "logs/app.log", tmpDir)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if !downloadCalled {
		t.Error("expected Download function to be called on executor")
	}
}

func TestProvisionRunsCommands(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "java -version") {
			return &ssh.ExecResult{Host: host, Stdout: "openjdk version 17\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	opts := ProvisionOptions{
		Timezone:     "Asia/Seoul",
		Locale:       "en_US.UTF-8",
		InstallJRE:   true,
		InstallTools: true,
	}
	err := d.Provision("dev", "api-server", 0, opts)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	cmds := mock.GetCommands()
	if len(cmds) == 0 {
		t.Fatal("expected commands to be executed")
	}

	foundTimezone := false
	foundLocale := false
	foundJRE := false
	foundTools := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "Asia/Seoul") {
			foundTimezone = true
		}
		if strings.Contains(cmd, "en_US.UTF-8") {
			foundLocale = true
		}
		if strings.Contains(cmd, "java") {
			foundJRE = true
		}
		if strings.Contains(cmd, "lsof") || strings.Contains(cmd, "TOOLS_OK") {
			foundTools = true
		}
	}
	if !foundTimezone {
		t.Error("expected timezone command")
	}
	if !foundLocale {
		t.Error("expected locale command")
	}
	if !foundJRE {
		t.Error("expected JRE install command")
	}
	if !foundTools {
		t.Error("expected tools install command")
	}
}

func TestNotifyDoesNotPanic(t *testing.T) {
	cfg := integrationConfig()
	cfg.Notifications = nil
	d := New(cfg, nil)

	// Should not panic with nil notifications
	d.Notify("dev", "api-server", "deploy_start", "test")
}

func TestNotifyDoesNotPanicEmpty(t *testing.T) {
	cfg := integrationConfig()
	cfg.Notifications = []config.NotificationConfig{}
	d := New(cfg, nil)

	d.Notify("dev", "api-server", "deploy_start", "test")
}

func TestExpandConfigDir(t *testing.T) {
	srcDir := t.TempDir()

	// Create a config file with env var references
	os.Setenv("TEST_DB_HOST", "db.example.com")
	defer os.Unsetenv("TEST_DB_HOST")

	configContent := `server:
  host: ${TEST_DB_HOST}
  port: 5432
`
	os.WriteFile(filepath.Join(srcDir, "database.yml"), []byte(configContent), 0644)

	// Create a subdirectory with another file
	subDir := filepath.Join(srcDir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "plain.txt"), []byte("no vars here"), 0644)

	expanded, err := expandConfigDir(srcDir)
	if err != nil {
		t.Fatalf("expandConfigDir failed: %v", err)
	}
	defer os.RemoveAll(expanded)

	// Check expanded file
	data, err := os.ReadFile(filepath.Join(expanded, "database.yml"))
	if err != nil {
		t.Fatalf("reading expanded file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "db.example.com") {
		t.Errorf("expected env var to be expanded, got: %s", content)
	}
	if strings.Contains(content, "${TEST_DB_HOST}") {
		t.Errorf("expected env var to be replaced, still found ${TEST_DB_HOST}")
	}

	// Check plain file
	plainData, err := os.ReadFile(filepath.Join(expanded, "sub", "plain.txt"))
	if err != nil {
		t.Fatalf("reading plain file: %v", err)
	}
	if string(plainData) != "no vars here" {
		t.Errorf("expected plain file unchanged, got: %s", string(plainData))
	}
}

func TestHealthCheckTCP(t *testing.T) {
	cfg := integrationConfig()
	// Adjust health check for fast test
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{
		Type:     "tcp",
		Target:   ":8080",
		Timeout:  2,
		Interval: 1,
		Retries:  1,
	}

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err != nil {
		t.Fatalf("waitForHealthy should succeed: %v", err)
	}
}

func TestHealthCheckTimeout(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{
		Type:     "tcp",
		Target:   ":8080",
		Timeout:  1,
		Interval: 1,
		Retries:  1,
	}

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "nc -z") {
			// Return a response that does NOT contain "HEALTHY" as a substring
			return &ssh.ExecResult{Host: host, Stdout: "FAILED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err == nil {
		t.Fatal("expected health check timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestHealthCheckNoConfig(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{}
	cfg.Modules["api-server"].Port = 0

	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err != nil {
		t.Fatalf("expected no error when no health check, got: %v", err)
	}
}

func TestHealthCheckHTTP(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{
		Type:     "http",
		Target:   "http://localhost:8080/health",
		Timeout:  2,
		Interval: 1,
		Retries:  1,
	}

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "curl") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err != nil {
		t.Fatalf("expected healthy, got: %v", err)
	}
}

func TestHealthCheckUnknownType(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{
		Type:     "unknown_type",
		Timeout:  2,
		Interval: 1,
		Retries:  1,
	}

	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err == nil {
		t.Fatal("expected error for unknown health check type")
	}
	if !strings.Contains(err.Error(), "unknown health check type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuditLogWritesFile(t *testing.T) {
	// Change to temp directory to avoid polluting project dir
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg := integrationConfig()
	d := New(cfg, nil)

	d.WriteAuditLog("dev", "api-server", "deploy", "test deployment")

	auditFile := filepath.Join(tmpDir, ".tow", "audit.log")
	data, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("expected audit log to be created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "env=dev") {
		t.Error("expected audit log to contain env=dev")
	}
	if !strings.Contains(content, "module=api-server") {
		t.Error("expected audit log to contain module=api-server")
	}
	if !strings.Contains(content, "action=deploy") {
		t.Error("expected audit log to contain action=deploy")
	}
}

func TestGetGitInfo(t *testing.T) {
	// Should not panic regardless of whether we're in a git repo
	commit, branch, message := getGitInfo()
	// In CI or non-git environments these may be empty, that's fine
	_ = commit
	_ = branch
	_ = message
}

func TestRemoteBaseDirForServerWithPattern(t *testing.T) {
	cfg := integrationConfig()
	cfg.Defaults.DeployPath = "{module}-{server}"
	d := New(cfg, nil)

	dir := d.RemoteBaseDirForServer("api-server", config.Server{Number: 2})
	if dir != "/app/api-server-2" {
		t.Errorf("expected /app/api-server-2, got %s", dir)
	}
}

func TestRemoteBaseDirForServerWithName(t *testing.T) {
	cfg := integrationConfig()
	cfg.Defaults.DeployPath = "{module}-{server}"
	d := New(cfg, nil)

	dir := d.RemoteBaseDirForServer("api-server", config.Server{Name: "primary"})
	if dir != "/app/api-server-primary" {
		t.Errorf("expected /app/api-server-primary, got %s", dir)
	}
}

func TestRemoteBaseDirForServerDefault(t *testing.T) {
	cfg := integrationConfig()
	d := New(cfg, nil)

	dir := d.RemoteBaseDirForServer("api-server", config.Server{})
	if dir != "/app/api-server" {
		t.Errorf("expected /app/api-server, got %s", dir)
	}
}

func TestStatusChecksJSON(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "status=running\npid=1234\nuptime=01:23\nmem=512000\ndeployment=20260401-120000\n"}, nil
	}
	d := New(cfg, mock)

	jsonStr, err := d.StatusJSON("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("StatusJSON failed: %v", err)
	}
	if !strings.Contains(jsonStr, "running") {
		t.Errorf("expected 'running' in JSON, got: %s", jsonStr)
	}
}

func TestListDeployments(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "  * 20260401-120000 (current)\n    20260331-100000\n"}, nil
	}
	d := New(cfg, mock)

	err := d.ListDeployments("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("ListDeployments failed: %v", err)
	}
}

func TestListDeploymentsJSON(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "20260401-120000:current\n20260331-100000:\n"}, nil
	}
	d := New(cfg, mock)

	jsonStr, err := d.ListDeploymentsJSON("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("ListDeploymentsJSON failed: %v", err)
	}
	if !strings.Contains(jsonStr, "20260401-120000") {
		t.Errorf("expected deployment in JSON, got: %s", jsonStr)
	}
}

func TestForceUnlock(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "LOCK_RELEASED\n"}, nil
	}
	d := New(cfg, mock)

	err := d.ForceUnlock("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("ForceUnlock failed: %v", err)
	}
}

func TestWithLock(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "mkdir") {
			return &ssh.ExecResult{Host: host, Stdout: "LOCK_ACQUIRED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "LOCK_RELEASED\n"}, nil
	}
	d := New(cfg, mock)

	called := false
	err := d.WithLock("dev", "api-server", 0, "deploy", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock failed: %v", err)
	}
	if !called {
		t.Error("expected function to be called within lock")
	}
}

func TestThreadDump(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "THREADDUMP_OK pid=1234\n"}, nil
	}
	d := New(cfg, mock)

	err := d.ThreadDump("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("ThreadDump failed: %v", err)
	}
}

func TestThreadDumpNonJavaModule(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].Type = "node"
	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	err := d.ThreadDump("dev", "api-server", 0)
	if err == nil {
		t.Fatal("expected error for non-java module")
	}
	if !strings.Contains(err.Error(), "only supported for java") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRestart(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "STOPPED") || strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STOPPED\n"}, nil
		}
		if strings.Contains(command, "HEALTHY") || strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Restart("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Restart failed: %v", err)
	}
}

func TestExecHook(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "hook output\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	// Should not panic
	d.execHook(env, "10.0.1.10", "pre_deploy", "echo hook")

	cmds := mock.GetCommands()
	found := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "echo hook") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hook command to be executed")
	}
}

func TestPrefixWriter(t *testing.T) {
	// Create a temp file to act as writer
	tmpFile, err := os.CreateTemp(t.TempDir(), "pw-test-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}

	pw := &prefixWriter{
		prefix: "[server] ",
		out:    tmpFile,
	}

	_, err = pw.Write([]byte("line one\nline two\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	tmpFile.Close()

	data, _ := os.ReadFile(tmpFile.Name())
	content := string(data)

	if !strings.Contains(content, "[server] line one") {
		t.Errorf("expected prefixed line one, got: %s", content)
	}
	if !strings.Contains(content, "[server] line two") {
		t.Errorf("expected prefixed line two, got: %s", content)
	}
}

// multiServerConfig returns a config with 2 servers for multi-server tests
func multiServerConfig() *config.Config {
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
					{Number: 2, Host: "10.0.1.11"},
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

func TestStartRolling(t *testing.T) {
	cfg := multiServerConfig()
	mock := &ssh.MockExecutor{}

	var startOrder []string
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "start.sh") {
			startOrder = append(startOrder, host)
		}
		// Health check returns HEALTHY
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.StartRolling("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("StartRolling failed: %v", err)
	}

	// Verify sequential start: both servers started
	if len(startOrder) != 2 {
		t.Fatalf("expected 2 servers started sequentially, got %d", len(startOrder))
	}
	if startOrder[0] != "10.0.1.10" || startOrder[1] != "10.0.1.11" {
		t.Errorf("expected start order [10.0.1.10, 10.0.1.11], got %v", startOrder)
	}

	// Verify health checks were performed (at least one nc -z per server)
	cmds := mock.GetCommands()
	healthChecks := 0
	for _, cmd := range cmds {
		if strings.Contains(cmd, "nc -z") {
			healthChecks++
		}
	}
	if healthChecks < 2 {
		t.Errorf("expected at least 2 health checks (one per server), got %d", healthChecks)
	}
}

func TestStartRollingHealthFailAborts(t *testing.T) {
	cfg := multiServerConfig()
	mock := &ssh.MockExecutor{}

	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		// Health check always fails — return FAILED (not UNHEALTHY, which contains "HEALTHY")
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "FAILED\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.StartRolling("dev", "api-server", 0)
	if err == nil {
		t.Fatal("expected error when health check fails during rolling start")
	}
	if !strings.Contains(err.Error(), "health check failed") {
		t.Errorf("expected health check failure error, got: %v", err)
	}
}

func TestLogsForServers(t *testing.T) {
	cfg := multiServerConfig()
	mock := &ssh.MockExecutor{}

	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "tail") {
			return &ssh.ExecResult{
				Host:   host,
				Stdout: "log from " + host + "\n",
			}, nil
		}
		// resolveLogPath detection
		return &ssh.ExecResult{Host: host, Stdout: "/app/api-server/log/std.log\n"}, nil
	}
	d := New(cfg, mock)

	servers := cfg.Environments["dev"].Servers
	err := d.LogsForServers("dev", "api-server", servers, "", 50, false)
	if err != nil {
		t.Fatalf("LogsForServers failed: %v", err)
	}

	cmds := mock.GetCommands()
	tailCount := 0
	for _, cmd := range cmds {
		if strings.Contains(cmd, "tail") {
			tailCount++
		}
	}
	// Each server: 1 resolveLogPath + 1 tail = 2 tail-containing commands per server,
	// but resolveLogPath uses "if [ -f" not "tail", so just count tail commands
	if tailCount < 2 {
		t.Errorf("expected at least 2 tail commands (one per server), got %d", tailCount)
	}
}

func TestLogsForServersEnvNotFound(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	err := d.LogsForServers("nonexistent", "api-server", nil, "", 50, false)
	if err == nil {
		t.Fatal("expected error for nonexistent environment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestLogsSingleNonFollow(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "single server log line 1\nsingle server log line 2\n"}, nil
	}
	d := New(cfg, mock)

	// Single server, non-follow path
	err := d.Logs("dev", "api-server", 1, "", 100, false)
	if err != nil {
		t.Fatalf("Logs single non-follow failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundTail := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "tail -n 100") {
			foundTail = true
			break
		}
	}
	if !foundTail {
		t.Errorf("expected 'tail -n 100' command, got: %s", strings.Join(cmds, "\n"))
	}
}

func TestUploadCert(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	// Create a temp cert file
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.pem")
	os.WriteFile(certPath, []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"), 0644)

	err := d.UploadCert("dev", "api-server", 0, certPath)
	if err != nil {
		t.Fatalf("UploadCert failed: %v", err)
	}

	// Verify mkdir was called for cert directory
	cmds := mock.GetCommands()
	foundMkdir := false
	foundChmod := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "mkdir") && strings.Contains(cmd, "cert") {
			foundMkdir = true
		}
		if strings.Contains(cmd, "chmod 600") {
			foundChmod = true
		}
	}
	if !foundMkdir {
		t.Error("expected mkdir command for cert directory")
	}
	if !foundChmod {
		t.Error("expected chmod 600 command for cert file")
	}

	// Verify upload was called
	if len(mock.Uploads) == 0 {
		t.Error("expected upload to be called for cert file")
	}
}

func TestUploadCertFileNotFound(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	err := d.UploadCert("dev", "api-server", 0, "/nonexistent/cert.pem")
	if err == nil {
		t.Fatal("expected error for nonexistent cert file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestListDeploymentsJSONFormat(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "20260401-120000:current\n20260331-100000:\n20260330-090000:\n"}, nil
	}
	d := New(cfg, mock)

	jsonStr, err := d.ListDeploymentsJSON("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("ListDeploymentsJSON failed: %v", err)
	}

	// Verify JSON structure
	if !strings.Contains(jsonStr, `"timestamp"`) {
		t.Errorf("expected 'timestamp' field in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"current"`) {
		t.Errorf("expected 'current' field in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"20260401-120000"`) {
		t.Errorf("expected timestamp value in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `true`) {
		t.Errorf("expected 'true' for current deployment, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `false`) {
		t.Errorf("expected 'false' for non-current deployment, got: %s", jsonStr)
	}
}

func TestLogsMultiServerNonFollow(t *testing.T) {
	cfg := multiServerConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "tail") {
			return &ssh.ExecResult{Host: host, Stdout: "multi log from " + host + "\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "/app/api-server/log/std.log\n"}, nil
	}
	d := New(cfg, mock)

	// 2 servers -> uses logsMulti path
	err := d.Logs("dev", "api-server", 0, "", 50, false)
	if err != nil {
		t.Fatalf("Logs multi-server failed: %v", err)
	}
}

func TestLogsMultiModule(t *testing.T) {
	cfg := multiServerConfig()
	// Add a second module
	cfg.Modules["worker"] = &config.Module{
		Type: "node",
		Port: 3000,
	}

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "tail") {
			return &ssh.ExecResult{Host: host, Stdout: "module log from " + host + "\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "/app/api-server/log/std.log\n"}, nil
	}
	d := New(cfg, mock)

	// 2 servers, 2 modules
	servers := []config.Server{
		{Number: 1, Host: "10.0.1.10"},
		{Number: 2, Host: "10.0.1.11"},
	}
	moduleNames := []string{"api-server", "worker"}

	err := d.LogsMultiModule("dev", servers, moduleNames, "", 50, false)
	if err != nil {
		t.Fatalf("LogsMultiModule failed: %v", err)
	}

	cmds := mock.GetCommands()
	tailCount := 0
	for _, cmd := range cmds {
		if strings.Contains(cmd, "tail -n 50") {
			tailCount++
		}
	}
	if tailCount < 2 {
		t.Errorf("expected at least 2 tail commands for multi-module logs, got %d", tailCount)
	}
}

func TestLogsMultiModuleEnvNotFound(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	err := d.LogsMultiModule("nonexistent", nil, nil, "", 50, false)
	if err == nil {
		t.Fatal("expected error for nonexistent environment")
	}
}

func TestHealthCheckLogType(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{
		Type:     "log",
		Target:   "Started .* in .* seconds",
		Timeout:  2,
		Interval: 1,
		Retries:  1,
	}

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "grep") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err != nil {
		t.Fatalf("expected log health check to pass: %v", err)
	}
}

func TestHealthCheckCommandType(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{
		Type:     "command",
		Target:   "curl -sf http://localhost:8080/ping",
		Timeout:  2,
		Interval: 1,
		Retries:  1,
	}

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "curl") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err != nil {
		t.Fatalf("expected command health check to pass: %v", err)
	}
}

func TestHealthCheckPortFallback(t *testing.T) {
	cfg := integrationConfig()
	// No explicit health check type but port is set - should auto-create TCP check
	cfg.Modules["api-server"].HealthCheck = config.HealthCheckConfig{
		Timeout:  2,
		Interval: 1,
		Retries:  1,
	}
	cfg.Modules["api-server"].Port = 9090

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "nc -z") && strings.Contains(command, "9090") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	err := d.waitForHealthy(env, "10.0.1.10", "api-server")
	if err != nil {
		t.Fatalf("expected port-fallback health check to pass: %v", err)
	}
}

func TestStopWithStillRunningProcess(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "STILL_RUNNING") {
			return &ssh.ExecResult{Host: host, Stdout: "STILL_RUNNING\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Stop("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Should have sent SIGKILL
	cmds := mock.GetCommands()
	foundKill := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "kill -9") {
			foundKill = true
			break
		}
	}
	if !foundKill {
		t.Error("expected kill -9 command for still running process")
	}
}

func TestStartWithHooks(t *testing.T) {
	cfg := integrationConfig()
	mod := cfg.Modules["api-server"]
	mod.Hooks = config.HooksConfig{
		PreStart:  "echo pre-start",
		PostStart: "echo post-start",
	}

	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "nc -z") {
			return &ssh.ExecResult{Host: host, Stdout: "HEALTHY\n"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Start("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Start with hooks failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundPre := false
	foundPost := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "echo pre-start") {
			foundPre = true
		}
		if strings.Contains(cmd, "echo post-start") {
			foundPost = true
		}
	}
	if !foundPre {
		t.Error("expected pre_start hook to be executed")
	}
	if !foundPost {
		t.Error("expected post_start hook to be executed")
	}
}

func TestStopWithHooks(t *testing.T) {
	cfg := integrationConfig()
	mod := cfg.Modules["api-server"]
	mod.Hooks = config.HooksConfig{
		PreStop:  "echo pre-stop",
		PostStop: "echo post-stop",
	}

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
		t.Fatalf("Stop with hooks failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundPre := false
	foundPost := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "echo pre-stop") {
			foundPre = true
		}
		if strings.Contains(cmd, "echo post-stop") {
			foundPost = true
		}
	}
	if !foundPre {
		t.Error("expected pre_stop hook to be executed")
	}
	if !foundPost {
		t.Error("expected post_stop hook to be executed")
	}
}

func TestInitWithDataDirs(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].DataDirs = []string{"data", "cache"}
	mock := &ssh.MockExecutor{}
	d := New(cfg, mock)

	err := d.Init("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Init with data dirs failed: %v", err)
	}

	cmds := mock.GetCommands()
	if len(cmds) == 0 {
		t.Fatal("expected commands")
	}
	mkdirCmd := cmds[0]
	if !strings.Contains(mkdirCmd, "data") {
		t.Error("expected data dir in mkdir command")
	}
	if !strings.Contains(mkdirCmd, "cache") {
		t.Error("expected cache dir in mkdir command")
	}
}

func TestExecHookFailure(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "failing-hook") {
			return &ssh.ExecResult{Host: host, Stdout: "", ExitCode: 1, Stderr: "hook error"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	env := cfg.Environments["dev"]
	// Should not panic with non-zero exit code
	d.execHook(env, "10.0.1.10", "test_hook", "failing-hook")
}

func TestProvisionKafkaModule(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].Type = "kafka"
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	opts := ProvisionOptions{
		InstallTools: true,
	}
	err := d.Provision("dev", "api-server", 0, opts)
	if err != nil {
		t.Fatalf("Provision kafka module failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundKafkaDir := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "kafka-logs") {
			foundKafkaDir = true
			break
		}
	}
	if !foundKafkaDir {
		t.Error("expected kafka-logs directory creation")
	}
}

func TestProvisionRedisModule(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].Type = "redis"
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	opts := ProvisionOptions{}
	err := d.Provision("dev", "api-server", 0, opts)
	if err != nil {
		t.Fatalf("Provision redis module failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundDataDir := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "mkdir") && strings.Contains(cmd, "data") {
			foundDataDir = true
			break
		}
	}
	if !foundDataDir {
		t.Error("expected data directory creation for redis")
	}
}

func TestProvisionWithDataDirs(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].DataDirs = []string{"data", "cache"}
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	opts := ProvisionOptions{}
	err := d.Provision("dev", "api-server", 0, opts)
	if err != nil {
		t.Fatalf("Provision with data dirs failed: %v", err)
	}

	cmds := mock.GetCommands()
	foundDataDir := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "mkdir") && strings.Contains(cmd, "data") && strings.Contains(cmd, "cache") {
			foundDataDir = true
			break
		}
	}
	if !foundDataDir {
		t.Error("expected data dirs creation")
	}
}

func TestStopNoPortModule(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].Port = 0
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Stop("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("Stop no-port module failed: %v", err)
	}
}

func TestStartFailsWithExitCode(t *testing.T) {
	cfg := integrationConfig()
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		if strings.Contains(command, "start.sh") {
			return &ssh.ExecResult{Host: host, ExitCode: 1, Stderr: "port already in use"}, nil
		}
		return &ssh.ExecResult{Host: host, Stdout: "OK\n"}, nil
	}
	d := New(cfg, mock)

	err := d.Start("dev", "api-server", 0)
	if err == nil {
		t.Fatal("expected error when start returns non-zero exit code")
	}
	if !strings.Contains(err.Error(), "exit code") {
		t.Errorf("expected exit code error, got: %v", err)
	}
}

func TestStatusJSONNoPort(t *testing.T) {
	cfg := integrationConfig()
	cfg.Modules["api-server"].Port = 0
	mock := &ssh.MockExecutor{}
	mock.ExecFn = func(env *config.Environment, host, command string) (*ssh.ExecResult, error) {
		return &ssh.ExecResult{Host: host, Stdout: "20260401-120000\n"}, nil
	}
	d := New(cfg, mock)

	jsonStr, err := d.StatusJSON("dev", "api-server", 0)
	if err != nil {
		t.Fatalf("StatusJSON no port failed: %v", err)
	}
	if !strings.Contains(jsonStr, "unknown") {
		t.Errorf("expected 'unknown' status for no-port module, got: %s", jsonStr)
	}
}
