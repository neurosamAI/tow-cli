package ssh

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/neurosamAI/tow-cli/internal/config"
)

func TestMockExecutorRecordsCommands(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{
		SSHUser: "testuser",
		SSHPort: 22,
	}

	commands := []string{"echo hello", "ls -la", "whoami"}
	for _, cmd := range commands {
		_, err := mock.Exec(env, "10.0.0.1", cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	recorded := mock.GetCommands()
	if len(recorded) != 3 {
		t.Fatalf("expected 3 recorded commands, got %d", len(recorded))
	}

	for i, cmd := range commands {
		if recorded[i] != cmd {
			t.Errorf("command %d: expected %q, got %q", i, cmd, recorded[i])
		}
	}
}

func TestMockExecutorCustomExecFn(t *testing.T) {
	mock := &MockExecutor{
		ExecFn: func(env *config.Environment, host, command string) (*ExecResult, error) {
			if command == "fail" {
				return nil, fmt.Errorf("command failed: %s", command)
			}
			return &ExecResult{
				Host:   host,
				Stdout: "custom output for: " + command,
			}, nil
		},
	}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	// Success case
	result, err := mock.Exec(env, "host1", "echo test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "custom output for: echo test" {
		t.Errorf("unexpected stdout: %q", result.Stdout)
	}

	// Failure case
	_, err = mock.Exec(env, "host1", "fail")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "command failed: fail" {
		t.Errorf("unexpected error: %v", err)
	}

	// Both commands should be recorded
	cmds := mock.GetCommands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 recorded commands, got %d", len(cmds))
	}
}

func TestMockExecutorIsDryRun(t *testing.T) {
	tests := []struct {
		name     string
		dryRun   bool
		expected bool
	}{
		{"dry run enabled", true, true},
		{"dry run disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockExecutor{DryRun: tt.dryRun}
			if mock.IsDryRun() != tt.expected {
				t.Errorf("expected IsDryRun() = %v, got %v", tt.expected, mock.IsDryRun())
			}
		})
	}
}

func TestManagerIsDryRun(t *testing.T) {
	tests := []struct {
		name     string
		dryRun   bool
		expected bool
	}{
		{"manager dry run enabled", true, true},
		{"manager dry run disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(false)
			m.DryRun = tt.dryRun
			if m.IsDryRun() != tt.expected {
				t.Errorf("expected IsDryRun() = %v, got %v", tt.expected, m.IsDryRun())
			}
		})
	}
}

func TestExecDryRunReturnsEmptyResult(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	result, err := m.Exec(nil, "10.0.1.10", "echo test")
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if result.Host != "10.0.1.10" {
		t.Errorf("expected host 10.0.1.10, got %q", result.Host)
	}
	if result.Stdout != "" {
		t.Errorf("expected empty stdout in dry-run, got %q", result.Stdout)
	}
	if result.Stderr != "" {
		t.Errorf("expected empty stderr in dry-run, got %q", result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestExecStreamDryRunNilWriters(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	// Should not panic with nil writers
	err := m.ExecStream(nil, "10.0.1.10", "echo test", nil, nil)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestExecStreamDryRunWithWriters(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	var stdout, stderr bytes.Buffer
	err := m.ExecStream(nil, "10.0.1.10", "echo test", &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	// In dry-run, nothing should be written
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
}

func TestMockExecutorExecStreamRecords(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	err := mock.ExecStream(env, "host1", "tail -f /var/log/app.log", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmds := mock.GetCommands()
	if len(cmds) != 1 || cmds[0] != "tail -f /var/log/app.log" {
		t.Errorf("expected recorded command, got: %v", cmds)
	}
}

func TestMockExecutorExecStreamCustomFn(t *testing.T) {
	var captured string
	mock := &MockExecutor{
		ExecStreamFn: func(env *config.Environment, host, command string, stdout, stderr io.Writer) error {
			captured = command
			if stdout != nil {
				stdout.Write([]byte("stream output\n"))
			}
			return nil
		},
	}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	var buf bytes.Buffer
	err := mock.ExecStream(env, "host1", "my-command", &buf, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != "my-command" {
		t.Errorf("expected 'my-command', got %q", captured)
	}
	if buf.String() != "stream output\n" {
		t.Errorf("expected stream output, got %q", buf.String())
	}
}

func TestMockExecutorUploadRecords(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	err := mock.Upload(env, "host1", "/local/file.tar.gz", "/remote/file.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.Uploads) != 1 {
		t.Fatalf("expected 1 upload recorded, got %d", len(mock.Uploads))
	}
	if mock.Uploads[0] != "/local/file.tar.gz -> host1:/remote/file.tar.gz" {
		t.Errorf("unexpected upload record: %s", mock.Uploads[0])
	}
}

func TestMockExecutorUploadCustomFn(t *testing.T) {
	mock := &MockExecutor{
		UploadFn: func(env *config.Environment, host, localPath, remotePath string) error {
			return fmt.Errorf("upload denied")
		},
	}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	err := mock.Upload(env, "host1", "/local/f", "/remote/f")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "upload denied" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMockExecutorDownloadDefault(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	err := mock.Download(env, "host1", "/remote/file", "/local/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMockExecutorDownloadCustomFn(t *testing.T) {
	mock := &MockExecutor{
		DownloadFn: func(env *config.Environment, host, remotePath, localDir string) error {
			return fmt.Errorf("download failed")
		},
	}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	err := mock.Download(env, "host1", "/remote/file", "/local/dir")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockExecutorUploadDirRecords(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	err := mock.UploadDir(env, "host1", "/local/dir", "/remote/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.Uploads) != 1 {
		t.Fatalf("expected 1 upload recorded, got %d", len(mock.Uploads))
	}
	expected := "/local/dir -> host1:/remote/dir"
	if mock.Uploads[0] != expected {
		t.Errorf("expected %q, got %q", expected, mock.Uploads[0])
	}
}

func TestMockExecutorUploadDirCustomFn(t *testing.T) {
	mock := &MockExecutor{
		UploadDirFn: func(env *config.Environment, host, localDir, remoteDir string) error {
			return fmt.Errorf("upload dir denied")
		},
	}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	err := mock.UploadDir(env, "host1", "/local/dir", "/remote/dir")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockExecutorCloseMultiple(t *testing.T) {
	mock := &MockExecutor{}
	// Should not panic when called multiple times
	mock.Close()
	mock.Close()
	mock.Close()
}

func TestMockExecutorDefaultExecReturnsOK(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	result, err := mock.Exec(env, "myhost", "any-command")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "OK\n" {
		t.Errorf("expected default 'OK\\n', got %q", result.Stdout)
	}
	if result.Host != "myhost" {
		t.Errorf("expected host 'myhost', got %q", result.Host)
	}
}

func TestMockExecutorGetCommandsThreadSafe(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	// Run concurrent commands
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			mock.Exec(env, "host", fmt.Sprintf("cmd-%d", n))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	cmds := mock.GetCommands()
	if len(cmds) != 10 {
		t.Errorf("expected 10 commands, got %d", len(cmds))
	}
}

func TestUploadDryRunNilEnv(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	err := m.Upload(nil, "10.0.1.10", "/local/file.tar.gz", "/remote/file.tar.gz")
	if err != nil {
		t.Fatalf("expected no error in dry-run Upload with nil env: %v", err)
	}
}

func TestUploadDirDryRunNilEnv(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	err := m.UploadDir(nil, "10.0.1.10", "/local/dir", "/remote/dir")
	if err != nil {
		t.Fatalf("expected no error in dry-run UploadDir with nil env: %v", err)
	}
}

func TestDownloadDryRunNilEnv(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	err := m.Download(nil, "10.0.1.10", "/remote/file", "/local/dir")
	if err != nil {
		t.Fatalf("expected no error in dry-run Download with nil env: %v", err)
	}
}

func TestExecDryRunMultipleCommands(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	commands := []string{"ls -la", "whoami", "df -h", "free -m"}
	for _, cmd := range commands {
		result, err := m.Exec(nil, "host1", cmd)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", cmd, err)
		}
		if result.Host != "host1" {
			t.Errorf("expected host1, got %q", result.Host)
		}
	}
}

func TestExecStreamDryRunMultipleHosts(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	hosts := []string{"host1", "host2", "host3"}
	for _, host := range hosts {
		err := m.ExecStream(nil, host, "tail -f /var/log/app.log", nil, nil)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", host, err)
		}
	}
}

func TestUploadDryRunMultipleFiles(t *testing.T) {
	m := NewManager(false)
	m.DryRun = true

	files := []struct {
		local  string
		remote string
	}{
		{"/local/a.tar.gz", "/remote/a.tar.gz"},
		{"/local/b.jar", "/remote/b.jar"},
		{"/local/c.war", "/remote/c.war"},
	}

	for _, f := range files {
		err := m.Upload(nil, "host1", f.local, f.remote)
		if err != nil {
			t.Fatalf("unexpected error uploading %q: %v", f.local, err)
		}
	}
}

func TestManagerDryRunSetAfterCreation(t *testing.T) {
	m := NewManager(true)
	if m.DryRun {
		t.Error("DryRun should be false when created with NewManager")
	}
	m.DryRun = true
	if !m.IsDryRun() {
		t.Error("expected IsDryRun() true after setting")
	}
}

func TestNewManagerCustomValues(t *testing.T) {
	m := NewManager(false)
	m.MaxRetries = 5
	m.MaxConcurrent = 50

	if m.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", m.MaxRetries)
	}
	if m.MaxConcurrent != 50 {
		t.Errorf("expected MaxConcurrent 50, got %d", m.MaxConcurrent)
	}
}

func TestMockExecutorExecStreamDefaultNoError(t *testing.T) {
	mock := &MockExecutor{}
	env := &config.Environment{SSHUser: "test", SSHPort: 22}

	// Default ExecStream with actual writers should not error
	var stdout, stderr bytes.Buffer
	err := mock.ExecStream(env, "host1", "cmd", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default impl writes nothing
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout from default ExecStream")
	}
}

func TestHostKeyCallbackInsecureReturnNonNil(t *testing.T) {
	m := NewManager(true)
	cb := m.hostKeyCallback()
	if cb == nil {
		t.Error("insecure callback should not be nil")
	}
	// Insecure callback should accept any key
	err := cb("example.com:22", nil, nil)
	if err != nil {
		t.Errorf("insecure callback should accept any key, got: %v", err)
	}
}

func TestExpandPathEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSame bool
	}{
		{"just tilde", "~", false},
		{"tilde with trailing content", "~/", false},
		{"tilde deep path", "~/.ssh/keys/deploy.pem", false},
		{"no tilde absolute", "/etc/ssh/ssh_config", true},
		{"no tilde relative", "keys/my.pem", true},
		{"empty", "", true},
		{"current dir", ".", true},
		{"parent dir", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.wantSame && result != tt.input {
				t.Errorf("expected same path %q, got %q", tt.input, result)
			}
			if !tt.wantSame && result == tt.input {
				t.Errorf("expected ~ expansion for %q", tt.input)
			}
		})
	}
}
