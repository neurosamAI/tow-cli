package ssh

import (
	"testing"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
)

func TestNewManager(t *testing.T) {
	m := NewManager(false)

	if m.InsecureHostKey {
		t.Error("expected InsecureHostKey to be false")
	}
	if m.DryRun {
		t.Error("expected DryRun to be false")
	}
	if m.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", m.MaxRetries)
	}
	if m.RetryDelay != 2*time.Second {
		t.Errorf("expected RetryDelay 2s, got %v", m.RetryDelay)
	}
	if m.CommandTimeout != 10*time.Minute {
		t.Errorf("expected CommandTimeout 10m, got %v", m.CommandTimeout)
	}
	if m.MaxConcurrent != 20 {
		t.Errorf("expected MaxConcurrent 20, got %d", m.MaxConcurrent)
	}
}

func TestNewManagerInsecure(t *testing.T) {
	m := NewManager(true)
	if !m.InsecureHostKey {
		t.Error("expected InsecureHostKey to be true")
	}
}

func TestConnectionKey(t *testing.T) {
	key := connectionKey("10.0.1.10", 22, "ec2-user")
	expected := "ec2-user@10.0.1.10:22"
	if key != expected {
		t.Errorf("expected %s, got %s", expected, key)
	}
}

func TestConnectionKeyCustomPort(t *testing.T) {
	key := connectionKey("example.com", 2222, "deploy")
	expected := "deploy@example.com:2222"
	if key != expected {
		t.Errorf("expected %s, got %s", expected, key)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		input   string
		hasHome bool
	}{
		{"~/.ssh/key.pem", true},
		{"/absolute/path/key.pem", false},
		{"relative/path/key.pem", false},
		{"", false},
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		if tt.hasHome && result == tt.input {
			t.Errorf("expected ~ to be expanded for %s", tt.input)
		}
		if !tt.hasHome && result != tt.input {
			t.Errorf("expected no change for %s, got %s", tt.input, result)
		}
	}
}

func TestExecDryRun(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	result, err := m.Exec(nil, "10.0.1.10", "echo hello")
	if err != nil {
		t.Errorf("expected no error in dry-run, got %v", err)
	}
	if result.Host != "10.0.1.10" {
		t.Errorf("expected host 10.0.1.10, got %s", result.Host)
	}
}

func TestManagerClose(t *testing.T) {
	m := NewManager(false)
	// Close should not panic on empty manager
	m.Close()
}

func TestHostKeyCallbackInsecure(t *testing.T) {
	m := NewManager(true)
	cb := m.hostKeyCallback()
	if cb == nil {
		t.Error("expected non-nil callback for insecure mode")
	}
}

func TestHostKeyCallbackSecure(t *testing.T) {
	m := NewManager(false)
	cb := m.hostKeyCallback()
	if cb == nil {
		t.Error("expected non-nil callback for secure mode")
	}
}

// --- Dry-run tests for all SSH operations ---

func TestExecStreamDryRun(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	err := m.ExecStream(nil, "10.0.1.10", "echo hello", nil, nil)
	if err != nil {
		t.Errorf("expected no error in dry-run ExecStream, got %v", err)
	}
}

func TestUploadDryRun(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	err := m.Upload(nil, "10.0.1.10", "/local/file.tar.gz", "/remote/file.tar.gz")
	if err != nil {
		t.Errorf("expected no error in dry-run Upload, got %v", err)
	}
}

func TestDownloadDryRun(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	err := m.Download(&config.Environment{
		SSHUser:    "ec2-user",
		SSHPort:    22,
		SSHKeyPath: "~/.ssh/test.pem",
	}, "10.0.1.10", "/remote/file.tar.gz", "/local/dir")
	if err != nil {
		t.Errorf("expected no error in dry-run Download, got %v", err)
	}
}

func TestUploadDirDryRun(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	err := m.UploadDir(&config.Environment{
		SSHUser:    "ec2-user",
		SSHPort:    22,
		SSHKeyPath: "~/.ssh/test.pem",
	}, "10.0.1.10", "/local/dir", "/remote/dir")
	if err != nil {
		t.Errorf("expected no error in dry-run UploadDir, got %v", err)
	}
}

// --- ConnectionKey tests ---

func TestConnectionKeyVariousInputs(t *testing.T) {
	tests := []struct {
		host     string
		port     int
		user     string
		expected string
	}{
		{"10.0.1.10", 22, "ec2-user", "ec2-user@10.0.1.10:22"},
		{"example.com", 2222, "deploy", "deploy@example.com:2222"},
		{"192.168.1.1", 22, "root", "root@192.168.1.1:22"},
		{"myhost", 0, "user", "user@myhost:0"},
		{"", 22, "", "@:22"},
	}

	for _, tt := range tests {
		result := connectionKey(tt.host, tt.port, tt.user)
		if result != tt.expected {
			t.Errorf("connectionKey(%q, %d, %q) = %q, expected %q",
				tt.host, tt.port, tt.user, result, tt.expected)
		}
	}
}

// --- ExpandPath tests ---

func TestExpandPathTableDriven(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expanded  bool // whether ~ should be expanded
		unchanged bool // whether output should equal input
	}{
		{"tilde path", "~/.ssh/key.pem", true, false},
		{"tilde only", "~", true, false},
		{"tilde slash", "~/", true, false},
		{"absolute path", "/absolute/path/key.pem", false, true},
		{"relative path", "relative/path/key.pem", false, true},
		{"empty string", "", false, true},
		{"dot path", "./key.pem", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.unchanged && result != tt.input {
				t.Errorf("expected no change for %q, got %q", tt.input, result)
			}
			if tt.expanded && result == tt.input {
				t.Errorf("expected ~ to be expanded for %q", tt.input)
			}
			if tt.expanded && len(result) > 0 && result[0] == '~' {
				t.Errorf("expanded path still starts with ~: %q", result)
			}
		})
	}
}

// --- sshCommandOpts tests ---

func TestSSHCommandOpts(t *testing.T) {
	m := NewManager(false)
	opts := m.sshCommandOpts("/path/to/key", 22)
	if opts != "ssh -i /path/to/key -p 22" {
		t.Errorf("unexpected opts: %q", opts)
	}
}

func TestSSHCommandOptsInsecure(t *testing.T) {
	m := NewManager(true)
	opts := m.sshCommandOpts("/path/to/key", 2222)
	if opts != "ssh -i /path/to/key -p 2222 -o StrictHostKeyChecking=no" {
		t.Errorf("unexpected opts: %q", opts)
	}
}

func TestSSHCommandOptsCustomPort(t *testing.T) {
	m := NewManager(false)
	opts := m.sshCommandOpts("~/.ssh/id_rsa", 9922)
	if opts != "ssh -i ~/.ssh/id_rsa -p 9922" {
		t.Errorf("unexpected opts: %q", opts)
	}
}

// --- resolveAuth tests ---

func TestResolveAuthDefaults(t *testing.T) {
	m := NewManager(false)
	env := &config.Environment{
		SSHUser:    "ec2-user",
		SSHPort:    22,
		SSHKeyPath: "~/.ssh/key.pem",
	}

	user, port, keyPath, authCfg := m.resolveAuth(env, "", nil)
	if user != "ec2-user" {
		t.Errorf("expected ec2-user, got %q", user)
	}
	if port != 22 {
		t.Errorf("expected 22, got %d", port)
	}
	if keyPath != "~/.ssh/key.pem" {
		t.Errorf("expected key path, got %q", keyPath)
	}
	if authCfg != nil {
		t.Error("expected nil authCfg")
	}
}

func TestResolveAuthWithModuleSSH(t *testing.T) {
	m := NewManager(false)
	env := &config.Environment{
		SSHUser:    "ec2-user",
		SSHPort:    22,
		SSHKeyPath: "~/.ssh/key.pem",
	}
	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {
				SSH: &config.SSHConfig{
					User:    "deploy",
					Port:    2222,
					KeyPath: "/custom/key",
					Auth:    "key",
				},
			},
		},
	}

	user, port, keyPath, authCfg := m.resolveAuth(env, "api", cfg)
	if user != "deploy" {
		t.Errorf("expected deploy, got %q", user)
	}
	if port != 2222 {
		t.Errorf("expected 2222, got %d", port)
	}
	if keyPath != "/custom/key" {
		t.Errorf("expected /custom/key, got %q", keyPath)
	}
	if authCfg == nil {
		t.Fatal("expected non-nil authCfg")
	}
	if authCfg.Auth != "key" {
		t.Errorf("expected auth 'key', got %q", authCfg.Auth)
	}
}

func TestResolveAuthPartialOverride(t *testing.T) {
	m := NewManager(false)
	env := &config.Environment{
		SSHUser:    "ec2-user",
		SSHPort:    22,
		SSHKeyPath: "~/.ssh/key.pem",
	}
	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {
				SSH: &config.SSHConfig{
					User: "deploy",
					// Port and KeyPath not set — should use env defaults
				},
			},
		},
	}

	user, port, keyPath, _ := m.resolveAuth(env, "api", cfg)
	if user != "deploy" {
		t.Errorf("expected deploy, got %q", user)
	}
	if port != 22 {
		t.Errorf("expected env port 22 (not overridden), got %d", port)
	}
	if keyPath != "~/.ssh/key.pem" {
		t.Errorf("expected env key path (not overridden), got %q", keyPath)
	}
}

func TestResolveAuthNoSSHConfig(t *testing.T) {
	m := NewManager(false)
	env := &config.Environment{
		SSHUser:    "ubuntu",
		SSHPort:    2222,
		SSHKeyPath: "/my/key",
	}
	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {
				// No SSH config
			},
		},
	}

	user, port, keyPath, authCfg := m.resolveAuth(env, "api", cfg)
	if user != "ubuntu" {
		t.Errorf("expected ubuntu, got %q", user)
	}
	if port != 2222 {
		t.Errorf("expected 2222, got %d", port)
	}
	if keyPath != "/my/key" {
		t.Errorf("expected /my/key, got %q", keyPath)
	}
	if authCfg != nil {
		t.Error("expected nil authCfg when module has no SSH config")
	}
}

func TestResolveAuthNilConfig(t *testing.T) {
	m := NewManager(false)
	env := &config.Environment{
		SSHUser:    "ec2-user",
		SSHPort:    22,
		SSHKeyPath: "~/.ssh/key.pem",
	}

	user, port, keyPath, authCfg := m.resolveAuth(env, "api", nil)
	if user != "ec2-user" {
		t.Errorf("expected ec2-user, got %q", user)
	}
	if port != 22 {
		t.Errorf("expected 22, got %d", port)
	}
	if keyPath != "~/.ssh/key.pem" {
		t.Errorf("expected key path, got %q", keyPath)
	}
	if authCfg != nil {
		t.Error("expected nil authCfg with nil config")
	}
}

func TestResolveAuthEmptyModuleName(t *testing.T) {
	m := NewManager(false)
	env := &config.Environment{
		SSHUser:    "ec2-user",
		SSHPort:    22,
		SSHKeyPath: "~/.ssh/key.pem",
	}
	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {
				SSH: &config.SSHConfig{User: "deploy"},
			},
		},
	}

	user, _, _, authCfg := m.resolveAuth(env, "", cfg)
	if user != "ec2-user" {
		t.Errorf("expected ec2-user (no module lookup), got %q", user)
	}
	if authCfg != nil {
		t.Error("expected nil authCfg with empty module name")
	}
}

// --- Manager defaults ---

func TestNewManagerDefaults(t *testing.T) {
	m := NewManager(false)

	if m.connections == nil {
		t.Error("expected connections map to be initialized")
	}
	if m.connSem == nil {
		t.Error("expected connSem to be initialized")
	}
	if cap(m.connSem) != 20 {
		t.Errorf("expected connSem capacity 20, got %d", cap(m.connSem))
	}
}

// --- ExecResult struct ---

func TestExecResult(t *testing.T) {
	r := &ExecResult{
		Stdout:   "output",
		Stderr:   "error",
		ExitCode: 1,
		Host:     "10.0.0.1",
	}

	if r.Stdout != "output" {
		t.Errorf("expected 'output', got %q", r.Stdout)
	}
	if r.Stderr != "error" {
		t.Errorf("expected 'error', got %q", r.Stderr)
	}
	if r.ExitCode != 1 {
		t.Errorf("expected 1, got %d", r.ExitCode)
	}
	if r.Host != "10.0.0.1" {
		t.Errorf("expected '10.0.0.1', got %q", r.Host)
	}
}

func TestExecDryRunWithNilEnv(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	// Should not panic with nil env in dry-run
	result, err := m.Exec(nil, "host1", "ls -la")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Host != "host1" {
		t.Errorf("expected host1, got %q", result.Host)
	}
	if result.Stdout != "" {
		t.Errorf("expected empty stdout in dry-run, got %q", result.Stdout)
	}
}

func TestExecDryRunReturnsZeroExitCode(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	result, _ := m.Exec(nil, "host", "command")
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0 in dry-run, got %d", result.ExitCode)
	}
}

func TestManagerCloseMultipleTimes(t *testing.T) {
	m := NewManager(false)
	// Should not panic when called multiple times
	m.Close()
	m.Close()
	m.Close()
}
