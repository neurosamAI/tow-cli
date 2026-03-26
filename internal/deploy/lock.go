package deploy

import (
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/ssh"
)

const lockDir = ".tow-lock"

// LockInfo contains information about who acquired the lock
type LockInfo struct {
	User      string
	Host      string
	Timestamp string
	Command   string
}

// AcquireLock creates a deploy lock on the target servers
func AcquireLock(sshMgr *ssh.Manager, env *config.Environment, host, baseDir, command string) error {
	lockPath := fmt.Sprintf("%s/%s", baseDir, lockDir)

	currentUser := "unknown"
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}
	ts := time.Now().Format("2006-01-02T15:04:05Z")

	// Try to create lock directory atomically (mkdir fails if already exists)
	lockCmd := fmt.Sprintf(`
if mkdir "%s" 2>/dev/null; then
    echo "user=%s" > "%s/info"
    echo "time=%s" >> "%s/info"
    echo "cmd=%s" >> "%s/info"
    echo "LOCK_ACQUIRED"
else
    echo "LOCK_EXISTS"
    cat "%s/info" 2>/dev/null
fi
`, lockPath, currentUser, lockPath, ts, lockPath, command, lockPath, lockPath)

	result, err := sshMgr.Exec(env, host, lockCmd)
	if err != nil {
		return fmt.Errorf("[%s] failed to acquire lock: %w", host, err)
	}

	if strings.Contains(result.Stdout, "LOCK_EXISTS") {
		lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
		info := ""
		for _, line := range lines {
			if strings.HasPrefix(line, "user=") || strings.HasPrefix(line, "time=") || strings.HasPrefix(line, "cmd=") {
				info += "  " + line + "\n"
			}
		}
		return fmt.Errorf("[%s] deploy is locked by another process:\n%sUse 'tow unlock -e ENV -m MODULE' to force release", host, info)
	}

	logger.Debug("[%s] Lock acquired", host)
	return nil
}

// ReleaseLock removes the deploy lock from the target servers
func ReleaseLock(sshMgr *ssh.Manager, env *config.Environment, host, baseDir string) {
	lockPath := fmt.Sprintf("%s/%s", baseDir, lockDir)
	unlockCmd := fmt.Sprintf(`rm -rf "%s" 2>/dev/null; echo "LOCK_RELEASED"`, lockPath)

	_, err := sshMgr.Exec(env, host, unlockCmd)
	if err != nil {
		logger.Warn("[%s] Failed to release lock: %v", host, err)
	} else {
		logger.Debug("[%s] Lock released", host)
	}
}

// ForceUnlock forcefully removes the deploy lock
func (d *Deployer) ForceUnlock(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	baseDir := d.remoteBaseDir(moduleName)

	for _, srv := range servers {
		ReleaseLock(d.ssh, env, srv.Host, baseDir)
		logger.Success("[%s] Lock released for %s", srv.Host, moduleName)
	}

	return nil
}

// WithLock executes fn while holding the deploy lock
func (d *Deployer) WithLock(envName, moduleName string, serverNum int, command string, fn func() error) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	baseDir := d.remoteBaseDir(moduleName)

	// Acquire lock on first server (coordinator)
	srv := servers[0]
	if err := AcquireLock(d.ssh, env, srv.Host, baseDir, command); err != nil {
		return err
	}
	defer ReleaseLock(d.ssh, env, srv.Host, baseDir)

	return fn()
}
