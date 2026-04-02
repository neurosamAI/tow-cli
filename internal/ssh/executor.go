package ssh

import (
	"io"

	"github.com/neurosamAI/tow-cli/internal/config"
)

// Executor defines the interface for SSH operations.
// Implementations: Manager (real SSH), MockExecutor (testing)
type Executor interface {
	Exec(env *config.Environment, host, command string) (*ExecResult, error)
	ExecStream(env *config.Environment, host, command string, stdout, stderr io.Writer) error
	Upload(env *config.Environment, host, localPath, remotePath string) error
	Download(env *config.Environment, host, remotePath, localDir string) error
	UploadDir(env *config.Environment, host, localDir, remoteDir string) error
	IsDryRun() bool
	Close()
}
