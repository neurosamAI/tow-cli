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
