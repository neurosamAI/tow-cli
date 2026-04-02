package ssh

import (
	"io"
	"sync"

	"github.com/neurosamAI/tow-cli/internal/config"
)

// MockExecutor is a test double for the Executor interface.
// It records all commands executed and allows custom behavior via function fields.
type MockExecutor struct {
	DryRun       bool
	ExecFn       func(env *config.Environment, host, command string) (*ExecResult, error)
	ExecStreamFn func(env *config.Environment, host, command string, stdout, stderr io.Writer) error
	UploadFn     func(env *config.Environment, host, localPath, remotePath string) error
	DownloadFn   func(env *config.Environment, host, remotePath, localDir string) error
	UploadDirFn  func(env *config.Environment, host, localDir, remoteDir string) error

	mu       sync.Mutex
	Commands []string // record all commands executed via Exec
	Uploads  []string // record all upload paths
}

func (m *MockExecutor) Exec(env *config.Environment, host, command string) (*ExecResult, error) {
	m.mu.Lock()
	m.Commands = append(m.Commands, command)
	m.mu.Unlock()

	if m.ExecFn != nil {
		return m.ExecFn(env, host, command)
	}
	return &ExecResult{Host: host, Stdout: "OK\n"}, nil
}

func (m *MockExecutor) ExecStream(env *config.Environment, host, command string, stdout, stderr io.Writer) error {
	m.mu.Lock()
	m.Commands = append(m.Commands, command)
	m.mu.Unlock()

	if m.ExecStreamFn != nil {
		return m.ExecStreamFn(env, host, command, stdout, stderr)
	}
	return nil
}

func (m *MockExecutor) Upload(env *config.Environment, host, localPath, remotePath string) error {
	m.mu.Lock()
	m.Uploads = append(m.Uploads, localPath+" -> "+host+":"+remotePath)
	m.mu.Unlock()

	if m.UploadFn != nil {
		return m.UploadFn(env, host, localPath, remotePath)
	}
	return nil
}

func (m *MockExecutor) Download(env *config.Environment, host, remotePath, localDir string) error {
	if m.DownloadFn != nil {
		return m.DownloadFn(env, host, remotePath, localDir)
	}
	return nil
}

func (m *MockExecutor) UploadDir(env *config.Environment, host, localDir, remoteDir string) error {
	m.mu.Lock()
	m.Uploads = append(m.Uploads, localDir+" -> "+host+":"+remoteDir)
	m.mu.Unlock()

	if m.UploadDirFn != nil {
		return m.UploadDirFn(env, host, localDir, remoteDir)
	}
	return nil
}

func (m *MockExecutor) IsDryRun() bool { return m.DryRun }

func (m *MockExecutor) Close() {}

// GetCommands returns a copy of all recorded commands (thread-safe).
func (m *MockExecutor) GetCommands() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmds := make([]string, len(m.Commands))
	copy(cmds, m.Commands)
	return cmds
}
