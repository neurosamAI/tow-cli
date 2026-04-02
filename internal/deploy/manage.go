package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
)

// Cleanup removes old deployment directories, keeping the N most recent
func (d *Deployer) Cleanup(envName, moduleName string, serverNum int, keep int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	if keep < 1 {
		keep = 5
	}

	results := d.RunParallel(servers, env, func(srv config.Server) error {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		logger.ServerAction(srv.Host, "Cleaning up old deployments for %s (keeping %d)", moduleName, keep)

		cleanupCmd := fmt.Sprintf(`
CURRENT=$(readlink %s/current 2>/dev/null | xargs basename 2>/dev/null)
REMOVED=0
for dir in $(ls -1t %s/deploy/ 2>/dev/null | tail -n +%d); do
    if [ "$dir" != "$CURRENT" ]; then
        rm -rf "%s/deploy/$dir"
        REMOVED=$((REMOVED + 1))
        echo "removed: $dir"
    fi
done
echo "CLEANUP_DONE removed=$REMOVED"
`, baseDir, baseDir, keep+1, baseDir)

		result, err := d.ssh.Exec(env, srv.Host, cleanupCmd)
		if err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		for _, line := range strings.Split(result.Stdout, "\n") {
			if strings.HasPrefix(line, "removed:") {
				logger.Debug("[%s] %s", srv.Host, line)
			}
			if strings.HasPrefix(line, "CLEANUP_DONE") {
				logger.Success("[%s] %s", srv.Host, line)
			}
		}

		return nil
	})

	return CheckParallelResults(results)
}

// Download copies a file from a remote server to local
func (d *Deployer) Download(envName, moduleName string, serverNum int, remotePath, localDir string) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	srv := servers[0]
	baseDir := d.RemoteBaseDirForServer(moduleName, srv)

	// If remotePath is relative, resolve it against module base dir
	if !filepath.IsAbs(remotePath) {
		remotePath = filepath.Join(baseDir, remotePath)
	}

	if localDir == "" {
		localDir = fmt.Sprintf("download/%s-%d/%s", envName, srv.Number, time.Now().Format("20060102-150405"))
	}

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("creating local directory: %w", err)
	}

	logger.ServerAction(srv.Host, "Downloading %s → %s", remotePath, localDir)

	if err := d.ssh.Download(env, srv.Host, remotePath, localDir); err != nil {
		return fmt.Errorf("[%s] download failed: %w", srv.Host, err)
	}

	logger.Success("[%s] Downloaded to %s", srv.Host, localDir)
	return nil
}

// ThreadDump triggers a thread dump on Java/Spring Boot modules
func (d *Deployer) ThreadDump(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]
	if mod.Type != "springboot" && mod.Type != "java" {
		return fmt.Errorf("threaddump is only supported for java/springboot modules (got: %s)", mod.Type)
	}

	for _, srv := range servers {
		logger.ServerAction(srv.Host, "Triggering thread dump for %s", moduleName)

		var dumpCmd string
		if mod.Port > 0 {
			dumpCmd = fmt.Sprintf(`
PID=$(lsof -i :%d -t 2>/dev/null | head -1)
if [ -z "$PID" ]; then
    echo "ERROR: no process found on port %d"
    exit 1
fi
kill -3 $PID
echo "THREADDUMP_OK pid=$PID"
`, mod.Port, mod.Port)
		} else {
			dumpCmd = fmt.Sprintf(`
PID=$(pgrep -f '%s' | head -1)
if [ -z "$PID" ]; then
    echo "ERROR: no process found for %s"
    exit 1
fi
kill -3 $PID
echo "THREADDUMP_OK pid=$PID"
`, moduleName, moduleName)
		}

		result, err := d.ssh.Exec(env, srv.Host, dumpCmd)
		if err != nil {
			return fmt.Errorf("[%s] thread dump failed: %w", srv.Host, err)
		}
		if strings.Contains(result.Stdout, "ERROR:") {
			return fmt.Errorf("[%s] %s", srv.Host, strings.TrimSpace(result.Stdout))
		}

		logger.Success("[%s] %s — check application logs for dump output", srv.Host, strings.TrimSpace(result.Stdout))
	}

	return nil
}

// UploadCert uploads SSL/TLS certificates to remote servers
func (d *Deployer) UploadCert(envName, moduleName string, serverNum int, certPath string) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found: %s", certPath)
	}

	for _, srv := range servers {
		remoteCertDir := filepath.Join(d.remoteBaseDir(moduleName), "cert")

		// Create cert directory
		mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteCertDir)
		d.ssh.Exec(env, srv.Host, mkdirCmd)

		remotePath := filepath.Join(remoteCertDir, filepath.Base(certPath))
		logger.ServerAction(srv.Host, "Uploading certificate %s → %s", filepath.Base(certPath), remotePath)

		if err := d.ssh.Upload(env, srv.Host, certPath, remotePath); err != nil {
			return fmt.Errorf("[%s] cert upload failed: %w", srv.Host, err)
		}

		// Set restrictive permissions
		chmodCmd := fmt.Sprintf("chmod 600 %s", remotePath)
		d.ssh.Exec(env, srv.Host, chmodCmd)

		logger.Success("[%s] Certificate uploaded to %s", srv.Host, remotePath)
	}

	return nil
}
