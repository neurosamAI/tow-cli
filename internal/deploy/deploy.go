package deploy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/module"
	"github.com/neurosamAI/tow-cli/internal/ssh"
)

// Deployer handles deployment operations for modules
type Deployer struct {
	cfg *config.Config
	ssh *ssh.Manager
}

// New creates a new Deployer
func New(cfg *config.Config, sshMgr *ssh.Manager) *Deployer {
	return &Deployer{
		cfg: cfg,
		ssh: sshMgr,
	}
}

// deployTimestamp generates a deployment directory name
func deployTimestamp() string {
	return time.Now().Format("20060102-150405")
}

// remoteBaseDir returns the base directory for a module on the remote server
func (d *Deployer) remoteBaseDir(moduleName string) string {
	return d.RemoteBaseDirForServer(moduleName, config.Server{})
}

// RemoteBaseDirForServer returns the base directory with server-aware path resolution.
// Supports deploy_path patterns: "{module}" (default) or "{module}-{server}" (legacy)
func (d *Deployer) RemoteBaseDirForServer(moduleName string, srv config.Server) string {
	baseDir := d.cfg.Project.BaseDir
	if baseDir == "" {
		baseDir = "/app"
	}

	pattern := d.cfg.Defaults.DeployPath
	if pattern == "" {
		pattern = "{module}"
	}

	dirName := strings.ReplaceAll(pattern, "{module}", moduleName)
	// {server} → number first (for legacy compat), then name as fallback
	if srv.Number > 0 {
		dirName = strings.ReplaceAll(dirName, "{server}", fmt.Sprintf("%d", srv.Number))
	} else if srv.Name != "" {
		dirName = strings.ReplaceAll(dirName, "{server}", srv.Name)
	} else {
		dirName = strings.ReplaceAll(dirName, "-{server}", "")
		dirName = strings.ReplaceAll(dirName, "{server}", "")
	}

	return filepath.Join(baseDir, dirName)
}

// Init initializes the server directory structure for a module
func (d *Deployer) Init(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]

	results := d.RunParallel(servers, env, func(srv config.Server) error {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		logger.ServerAction(srv.Host, "Initializing %s", moduleName)

		dirs := []string{
			baseDir,
			filepath.Join(baseDir, "upload"),
			filepath.Join(baseDir, "deploy"),
			filepath.Join(baseDir, "logs"),
			filepath.Join(baseDir, "conf"),
		}

		for _, dataDir := range mod.DataDirs {
			dirs = append(dirs, filepath.Join(baseDir, dataDir))
		}

		mkdirCmd := "mkdir -p " + strings.Join(dirs, " ")
		result, err := d.ssh.Exec(env, srv.Host, mkdirCmd)
		if err != nil {
			return fmt.Errorf("init failed: %w", err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("init failed: %s", result.Stderr)
		}

		logger.Success("[%s] Directory structure created for %s", srv.Host, moduleName)
		return nil
	})

	return CheckParallelResults(results)
}

// Upload transfers the package to target servers
func (d *Deployer) Upload(envName, moduleName string, serverNum int, filePath string) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]
	if filePath == "" {
		filePath = mod.ArtifactPath
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if d.ssh.DryRun {
			logger.Info("[DRY-RUN] Artifact not found (skipping): %s", filePath)
		} else {
			return fmt.Errorf("artifact not found: %s", filePath)
		}
	}

	results := d.RunParallel(servers, env, func(srv config.Server) error {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		remotePath := filepath.Join(baseDir, "upload", filepath.Base(filePath))
		logger.ServerAction(srv.Host, "Uploading %s → %s", filepath.Base(filePath), remotePath)

		if err := d.ssh.Upload(env, srv.Host, filePath, remotePath); err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		logger.Success("[%s] Upload complete", srv.Host)
		return nil
	})

	return CheckParallelResults(results)
}

// Install extracts the uploaded package and creates the symlink
func (d *Deployer) Install(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]
	ts := deployTimestamp()

	packageFile := filepath.Base(mod.ArtifactPath)
	if packageFile == "" {
		packageFile = moduleName + ".tar.gz"
	}

	results := d.RunParallel(servers, env, func(srv config.Server) error {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		deployDir := filepath.Join(baseDir, "deploy", ts)
		logger.ServerAction(srv.Host, "Installing %s (deploy/%s)", moduleName, ts)

		if mod.Hooks.PreDeploy != "" {
			d.execHook(env, srv.Host, "pre_deploy", mod.Hooks.PreDeploy)
		}

		installCmd := fmt.Sprintf(`
set -e
mkdir -p %s
cd %s
tar xzf %s/upload/%s 2>/dev/null || cp %s/upload/%s . 2>/dev/null || true

# Create symlink: current → deploy/{timestamp}
cd %s
rm -f current
ln -s deploy/%s current

echo "DEPLOY_OK"
`, deployDir, deployDir, baseDir, packageFile, baseDir, packageFile,
			baseDir, ts)

		result, err := d.ssh.Exec(env, srv.Host, installCmd)
		if err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		if !strings.Contains(result.Stdout, "DEPLOY_OK") {
			return fmt.Errorf("install failed: %s%s", result.Stdout, result.Stderr)
		}

		// Write deployment metadata (git commit, branch, timestamp)
		gitCommit, gitBranch, gitMsg := getGitInfo()
		deployInfoCmd := fmt.Sprintf(`cat > %s/current/.tow-deploy-info << 'TOWEOF'
deploy_ts=%s
commit=%s
branch=%s
message=%s
user=%s
TOWEOF
`, baseDir, ts, gitCommit, gitBranch, gitMsg, os.Getenv("USER"))
		d.ssh.Exec(env, srv.Host, deployInfoCmd)

		configPath := d.cfg.GetConfigPathByName(moduleName, envName, srv.Name, srv.Number)
		if configPath != "" {
			if info, err := os.Stat(configPath); err == nil && info.IsDir() {
				// Expand ${VAR} in config files before uploading (secrets stay in env vars, not in git)
				expandedDir, expandErr := expandConfigDir(configPath)
				uploadDir := configPath
				if expandErr != nil {
					logger.Warn("[%s] Config variable expansion failed, uploading raw: %v", srv.Host, expandErr)
				} else {
					uploadDir = expandedDir
					defer os.RemoveAll(expandedDir)
				}

				remoteConfDir := filepath.Join(baseDir, "conf")
				logger.ServerAction(srv.Host, "Uploading config from %s", configPath)
				if err := d.ssh.UploadDir(env, srv.Host, uploadDir, remoteConfDir); err != nil {
					logger.Warn("[%s] Config upload failed: %v", srv.Host, err)
				}
			}
		}

		logger.Success("[%s] Installed → deploy/%s (symlinked to current)", srv.Host, ts)

		if mod.Hooks.PostDeploy != "" {
			d.execHook(env, srv.Host, "post_deploy", mod.Hooks.PostDeploy)
		}

		return nil
	})

	return CheckParallelResults(results)
}

// Start starts the module on target servers
func (d *Deployer) Start(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]

	results := d.RunParallel(servers, env, func(srv config.Server) error {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)

		startCmd := mod.StartCmd
		if startCmd == "" {
			startCmd = fmt.Sprintf("%s/current/bin/start.sh", baseDir)
		}

		logger.ServerAction(srv.Host, "Starting %s", moduleName)

		if mod.Hooks.PreStart != "" {
			d.execHook(env, srv.Host, "pre_start", mod.Hooks.PreStart)
		}

		cmd := fmt.Sprintf("cd %s/current && %s", baseDir, startCmd)
		result, err := d.ssh.Exec(env, srv.Host, cmd)
		if err != nil {
			return fmt.Errorf("start failed: %w", err)
		}
		if result.ExitCode != 0 {
			logger.Error("[%s] Start returned exit code %d: %s", srv.Host, result.ExitCode, result.Stderr)
			return fmt.Errorf("start failed with exit code %d", result.ExitCode)
		}

		if err := d.waitForHealthy(env, srv.Host, moduleName); err != nil {
			logger.Warn("[%s] Health check failed: %v", srv.Host, err)
		} else {
			logger.Success("[%s] %s started and healthy", srv.Host, moduleName)
		}

		if mod.Hooks.PostStart != "" {
			d.execHook(env, srv.Host, "post_start", mod.Hooks.PostStart)
		}

		return nil
	})

	return CheckParallelResults(results)
}

// StartRolling starts the module on servers one at a time, verifying health before proceeding
func (d *Deployer) StartRolling(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]

	for i, srv := range servers {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)

		startCmd := mod.StartCmd
		if startCmd == "" {
			startCmd = fmt.Sprintf("%s/current/bin/start.sh", baseDir)
		}

		logger.Info("[%d/%d] Rolling start on %s", i+1, len(servers), srv.Host)

		if mod.Hooks.PreStart != "" {
			d.execHook(env, srv.Host, "pre_start", mod.Hooks.PreStart)
		}

		cmd := fmt.Sprintf("cd %s/current && %s", baseDir, startCmd)
		result, err := d.ssh.Exec(env, srv.Host, cmd)
		if err != nil {
			return fmt.Errorf("[%s] start failed: %w", srv.Host, err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("[%s] start failed with exit code %d: %s", srv.Host, result.ExitCode, result.Stderr)
		}

		// Must pass health check before moving to next server
		if err := d.waitForHealthy(env, srv.Host, moduleName); err != nil {
			return fmt.Errorf("[%s] health check failed during rolling start, aborting: %w", srv.Host, err)
		}

		logger.Success("[%s] %s started and healthy (%d/%d)", srv.Host, moduleName, i+1, len(servers))

		if mod.Hooks.PostStart != "" {
			d.execHook(env, srv.Host, "post_start", mod.Hooks.PostStart)
		}
	}

	return nil
}

// Stop stops the module on target servers
func (d *Deployer) Stop(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]

	results := d.RunParallel(servers, env, func(srv config.Server) error {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)

		stopCmd := mod.StopCmd
		if stopCmd == "" {
			stopCmd = fmt.Sprintf("%s/current/bin/stop.sh", baseDir)
		}

		logger.ServerAction(srv.Host, "Stopping %s", moduleName)

		if mod.Hooks.PreStop != "" {
			d.execHook(env, srv.Host, "pre_stop", mod.Hooks.PreStop)
		}

		cmd := fmt.Sprintf("cd %s/current && %s 2>/dev/null || true", baseDir, stopCmd)
		_, err := d.ssh.Exec(env, srv.Host, cmd)
		if err != nil {
			logger.Warn("[%s] Stop command error (may be already stopped): %v", srv.Host, err)
		}

		if mod.Port > 0 {
			checkCmd := fmt.Sprintf("sleep 2 && ! lsof -i :%d -t >/dev/null 2>&1 && echo 'STOPPED' || echo 'STILL_RUNNING'", mod.Port)
			result, _ := d.ssh.Exec(env, srv.Host, checkCmd)
			if strings.Contains(result.Stdout, "STILL_RUNNING") {
				logger.Warn("[%s] Process still running on port %d, sending SIGKILL...", srv.Host, mod.Port)
				killCmd := fmt.Sprintf("lsof -i :%d -t 2>/dev/null | xargs kill -9 2>/dev/null || true", mod.Port)
				d.ssh.Exec(env, srv.Host, killCmd)
			} else {
				logger.Success("[%s] %s stopped", srv.Host, moduleName)
			}
		} else {
			logger.Success("[%s] %s stop command sent", srv.Host, moduleName)
		}

		if mod.Hooks.PostStop != "" {
			d.execHook(env, srv.Host, "post_stop", mod.Hooks.PostStop)
		}

		return nil
	})

	return CheckParallelResults(results)
}

// Restart stops then starts a module
func (d *Deployer) Restart(envName, moduleName string, serverNum int) error {
	logger.Header("Restarting %s in %s", moduleName, envName)

	if err := d.Stop(envName, moduleName, serverNum); err != nil {
		logger.Warn("Stop returned error (continuing): %v", err)
	}
	return d.Start(envName, moduleName, serverNum)
}

// Status checks the module status on target servers
func (d *Deployer) Status(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	mod := d.cfg.Modules[moduleName]

	for _, srv := range servers {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		statusCmd := mod.StatusCmd
		if statusCmd == "" {
			// Try handler default first
			if handler, err := module.Get(mod.Type); err == nil {
				statusCmd = handler.DefaultStatusCmd(baseDir, mod.Port)
			}
		}
		if statusCmd == "" {
			if mod.Port > 0 {
				statusCmd = fmt.Sprintf(`
PID=$(lsof -i :%d -t 2>/dev/null | head -1)
if [ -n "$PID" ]; then
    UPTIME=$(ps -o etime= -p $PID 2>/dev/null | tr -d ' ')
    MEM=$(ps -o rss= -p $PID 2>/dev/null | tr -d ' ')
    echo "RUNNING pid=$PID uptime=$UPTIME mem=${MEM}KB"
else
    echo "STOPPED"
fi

# Show current deployment
CURRENT=$(readlink %s/current 2>/dev/null || echo "none")
echo "deployment=$CURRENT"
`, mod.Port, baseDir)
			} else {
				statusCmd = fmt.Sprintf(`
CURRENT=$(readlink %s/current 2>/dev/null || echo "none")
echo "deployment=$CURRENT"
`, baseDir)
			}
		}

		result, err := d.ssh.Exec(env, srv.Host, statusCmd)
		if err != nil {
			logger.Error("[%s] Status check failed: %v", srv.Host, err)
			continue
		}

		fmt.Printf("  [%s] %s:\n", srv.Host, srv.ID())
		for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
			if line != "" {
				fmt.Printf("    %s\n", line)
			}
		}
	}

	return nil
}

// StatusJSON returns status data as JSON for machine-readable output
func (d *Deployer) StatusJSON(envName, moduleName string, serverNum int) (string, error) {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return "", err
	}

	mod := d.cfg.Modules[moduleName]

	type ServerStatus struct {
		Host       string `json:"host"`
		Server     string `json:"server"`
		Status     string `json:"status"`
		PID        string `json:"pid,omitempty"`
		Uptime     string `json:"uptime,omitempty"`
		Memory     string `json:"memory,omitempty"`
		Deployment string `json:"deployment"`
	}

	var statuses []ServerStatus

	for _, srv := range servers {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		ss := ServerStatus{Host: srv.Host, Server: srv.ID()}

		if mod.Port > 0 {
			checkCmd := fmt.Sprintf(`
PID=$(lsof -i :%d -t 2>/dev/null | head -1)
if [ -n "$PID" ]; then
    UPTIME=$(ps -o etime= -p $PID 2>/dev/null | tr -d ' ')
    MEM=$(ps -o rss= -p $PID 2>/dev/null | tr -d ' ')
    echo "status=running"
    echo "pid=$PID"
    echo "uptime=$UPTIME"
    echo "mem=$MEM"
else
    echo "status=stopped"
fi
CURRENT=$(readlink %s/current 2>/dev/null | xargs basename 2>/dev/null || echo "none")
echo "deployment=$CURRENT"
`, mod.Port, baseDir)
			result, err := d.ssh.Exec(env, srv.Host, checkCmd)
			if err != nil {
				ss.Status = "error"
			} else {
				for _, line := range strings.Split(result.Stdout, "\n") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) != 2 {
						continue
					}
					switch parts[0] {
					case "status":
						ss.Status = parts[1]
					case "pid":
						ss.PID = parts[1]
					case "uptime":
						ss.Uptime = parts[1]
					case "mem":
						ss.Memory = parts[1] + "KB"
					case "deployment":
						ss.Deployment = parts[1]
					}
				}
			}
		} else {
			deployCmd := fmt.Sprintf(`readlink %s/current 2>/dev/null | xargs basename 2>/dev/null || echo "none"`, baseDir)
			result, _ := d.ssh.Exec(env, srv.Host, deployCmd)
			ss.Status = "unknown"
			ss.Deployment = strings.TrimSpace(result.Stdout)
		}

		statuses = append(statuses, ss)
	}

	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Rollback switches the current symlink to a previous deployment
func (d *Deployer) Rollback(envName, moduleName string, serverNum int, target string) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	for _, srv := range servers {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		logger.ServerAction(srv.Host, "Rolling back %s", moduleName)

		var rollbackCmd string
		if target != "" {
			rollbackCmd = fmt.Sprintf(`
set -e
if [ ! -d "%s/deploy/%s" ]; then
    echo "ERROR: deployment %s not found"
    exit 1
fi
cd %s
rm -f current
ln -s deploy/%s current
echo "ROLLBACK_OK to %s"
`, baseDir, target, target, baseDir, target, target)
		} else {
			rollbackCmd = fmt.Sprintf(`
set -e
CURRENT=$(readlink %s/current 2>/dev/null | xargs basename)
PREVIOUS=$(ls -1t %s/deploy/ | grep -v "^$CURRENT$" | head -1)

if [ -z "$PREVIOUS" ]; then
    echo "ERROR: no previous deployment found"
    exit 1
fi

cd %s
rm -f current
ln -s deploy/$PREVIOUS current
echo "ROLLBACK_OK from $CURRENT to $PREVIOUS"
`, baseDir, baseDir, baseDir)
		}

		result, err := d.ssh.Exec(env, srv.Host, rollbackCmd)
		if err != nil {
			return fmt.Errorf("[%s] rollback failed: %w", srv.Host, err)
		}
		if strings.Contains(result.Stdout, "ERROR:") {
			return fmt.Errorf("[%s] %s", srv.Host, strings.TrimSpace(result.Stdout))
		}

		logger.Success("[%s] %s", srv.Host, strings.TrimSpace(result.Stdout))
	}

	// Restart after rollback
	logger.Info("Restarting after rollback...")
	return d.Restart(envName, moduleName, serverNum)
}

// serverLogColors assigns colors to server prefixes for multi-server log output
var serverLogColors = []string{
	"\033[36m", // cyan
	"\033[33m", // yellow
	"\033[32m", // green
	"\033[35m", // magenta
	"\033[34m", // blue
	"\033[91m", // bright red
	"\033[92m", // bright green
	"\033[93m", // bright yellow
	"\033[94m", // bright blue
	"\033[95m", // bright magenta
}

// resolveLogPath returns the log file path for a server, auto-detecting rotated files
func (d *Deployer) resolveLogPath(env *config.Environment, srv config.Server, moduleName string) string {
	mod := d.cfg.Modules[moduleName]
	baseDir := d.RemoteBaseDirForServer(moduleName, srv)

	logPath := mod.LogPath
	if logPath == "" {
		logDir := d.cfg.Defaults.LogDir
		if logDir == "" {
			logDir = "log"
		}
		logFile := d.cfg.Defaults.LogFile
		if logFile == "" {
			logFile = "std.log"
		}
		logPath = filepath.Join(baseDir, logDir, logFile)
	}

	// Auto-detect latest rotated log file
	detectCmd := fmt.Sprintf(`
if [ -f "%s" ]; then echo "%s"
else LATEST=$(ls -1t %s/log/std*.log %s/log/*.log 2>/dev/null | head -1)
  if [ -n "$LATEST" ]; then echo "$LATEST"; else echo "%s"; fi
fi`, logPath, logPath, baseDir, baseDir, logPath)

	result, err := d.ssh.Exec(env, srv.Host, detectCmd)
	if err == nil && strings.TrimSpace(result.Stdout) != "" {
		return strings.TrimSpace(result.Stdout)
	}
	return logPath
}

// Logs reads or streams log output from a module (single or multi-server)
func (d *Deployer) Logs(envName, moduleName string, serverNum int, filter string, lines int, follow bool) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	// Single server — simple path
	if len(servers) == 1 {
		return d.logsSingle(env, servers[0], moduleName, filter, lines, follow)
	}

	// Multi-server — multiplexed output with color-coded prefixes
	return d.logsMulti(env, servers, moduleName, filter, lines, follow)
}

// LogsForServers reads/streams logs from a pre-filtered list of servers
func (d *Deployer) LogsForServers(envName, moduleName string, servers []config.Server, filter string, lines int, follow bool) error {
	env, ok := d.cfg.Environments[envName]
	if !ok {
		return fmt.Errorf("environment %q not found", envName)
	}

	if len(servers) == 1 {
		return d.logsSingle(env, servers[0], moduleName, filter, lines, follow)
	}
	return d.logsMulti(env, servers, moduleName, filter, lines, follow)
}

// logsSingle handles log reading/streaming for a single server
func (d *Deployer) logsSingle(env *config.Environment, srv config.Server, moduleName, filter string, lines int, follow bool) error {
	logPath := d.resolveLogPath(env, srv, moduleName)

	if follow {
		tailCmd := fmt.Sprintf("tail -n %d -f %s", lines, logPath)
		if filter != "" {
			tailCmd += fmt.Sprintf(" | grep --line-buffered '%s'", filter)
		}
		logger.Info("Streaming logs from %s@%s:%s", env.SSHUser, srv.Host, logPath)
		logger.Info("Press Ctrl+C to stop\n")
		return d.ssh.ExecStream(env, srv.Host, tailCmd, os.Stdout, os.Stderr)
	}

	tailCmd := fmt.Sprintf("tail -n %d %s", lines, logPath)
	if filter != "" {
		tailCmd += fmt.Sprintf(" | grep '%s'", filter)
	}
	logger.Info("Reading logs from %s@%s:%s", env.SSHUser, srv.Host, logPath)
	result, err := d.ssh.Exec(env, srv.Host, tailCmd)
	if err != nil {
		return fmt.Errorf("[%s] failed to read logs: %w", srv.Host, err)
	}
	fmt.Print(result.Stdout)
	return nil
}

// logsMulti handles multiplexed log output from multiple servers
func (d *Deployer) logsMulti(env *config.Environment, servers []config.Server, moduleName, filter string, lines int, follow bool) error {
	colorReset := "\033[0m"

	if !follow {
		// Non-follow: read from each server sequentially with prefix
		for i, srv := range servers {
			logPath := d.resolveLogPath(env, srv, moduleName)
			color := serverLogColors[i%len(serverLogColors)]
			prefix := fmt.Sprintf("%s[%s]%s ", color, srv.ID(), colorReset)

			tailCmd := fmt.Sprintf("tail -n %d %s", lines, logPath)
			if filter != "" {
				tailCmd += fmt.Sprintf(" | grep '%s'", filter)
			}

			result, err := d.ssh.Exec(env, srv.Host, tailCmd)
			if err != nil {
				logger.Warn("[%s] failed to read logs: %v", srv.ID(), err)
				continue
			}

			for _, line := range strings.Split(strings.TrimRight(result.Stdout, "\n"), "\n") {
				if line != "" {
					fmt.Printf("%s%s\n", prefix, line)
				}
			}
		}
		return nil
	}

	// Follow mode: concurrent SSH streams with color-coded prefix
	logger.Info("Streaming logs from %d servers. Press Ctrl+C to stop.\n", len(servers))

	var wg sync.WaitGroup
	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, s config.Server) {
			defer wg.Done()

			logPath := d.resolveLogPath(env, s, moduleName)
			color := serverLogColors[idx%len(serverLogColors)]
			prefix := fmt.Sprintf("%s[%s]%s ", color, s.ID(), colorReset)

			tailCmd := fmt.Sprintf("tail -n %d -f %s", lines, logPath)
			if filter != "" {
				tailCmd += fmt.Sprintf(" | grep --line-buffered '%s'", filter)
			}

			// Create a prefixed writer that adds server name to each line
			pw := &prefixWriter{prefix: prefix, out: os.Stdout}
			if err := d.ssh.ExecStream(env, s.Host, tailCmd, pw, pw); err != nil {
				logger.Warn("[%s] log stream ended: %v", s.ID(), err)
			}
		}(i, srv)
	}

	wg.Wait()
	return nil
}

// prefixWriter wraps an io.Writer and prepends a prefix to each line
type prefixWriter struct {
	prefix string
	out    *os.File
	buf    []byte
	mu     sync.Mutex
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.buf = append(pw.buf, p...)

	for {
		idx := bytes.IndexByte(pw.buf, '\n')
		if idx < 0 {
			break
		}
		line := pw.buf[:idx]
		pw.buf = pw.buf[idx+1:]

		if len(line) > 0 {
			fmt.Fprintf(pw.out, "%s%s\n", pw.prefix, string(line))
		}
	}

	return len(p), nil
}

// ListDeployments shows deployment history for a module
func (d *Deployer) ListDeployments(envName, moduleName string, serverNum int) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	srv := servers[0]
	baseDir := d.RemoteBaseDirForServer(moduleName, srv)

	listCmd := fmt.Sprintf(`
CURRENT=$(readlink %s/current 2>/dev/null | xargs basename 2>/dev/null)
for dir in $(ls -1t %s/deploy/ 2>/dev/null); do
    if [ "$dir" = "$CURRENT" ]; then
        echo "  * $dir (current)"
    else
        echo "    $dir"
    fi
done
`, baseDir, baseDir)

	result, err := d.ssh.Exec(env, srv.Host, listCmd)
	if err != nil {
		return fmt.Errorf("[%s] list deployments failed: %w", srv.Host, err)
	}

	fmt.Printf("Deployments for %s on %s:\n%s", moduleName, srv.Host, result.Stdout)
	return nil
}

// ListDeploymentsJSON returns deployment list as JSON
func (d *Deployer) ListDeploymentsJSON(envName, moduleName string, serverNum int) (string, error) {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return "", err
	}

	srv := servers[0]
	baseDir := d.RemoteBaseDirForServer(moduleName, srv)

	listCmd := fmt.Sprintf(`
CURRENT=$(readlink %s/current 2>/dev/null | xargs basename 2>/dev/null)
for dir in $(ls -1t %s/deploy/ 2>/dev/null); do
    if [ "$dir" = "$CURRENT" ]; then
        echo "$dir:current"
    else
        echo "$dir:"
    fi
done
`, baseDir, baseDir)

	result, err := d.ssh.Exec(env, srv.Host, listCmd)
	if err != nil {
		return "", err
	}

	type DeployEntry struct {
		Timestamp string `json:"timestamp"`
		Current   bool   `json:"current"`
	}

	var entries []DeployEntry
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		entries = append(entries, DeployEntry{
			Timestamp: parts[0],
			Current:   len(parts) > 1 && parts[1] == "current",
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

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

// WriteAuditLog writes a deployment event to a local audit log file
func (d *Deployer) WriteAuditLog(envName, moduleName, action, detail string) {
	auditDir := ".tow"
	auditFile := filepath.Join(auditDir, "audit.log")

	os.MkdirAll(auditDir, 0755)

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "unknown"
	}

	entry := fmt.Sprintf("%s | user=%s | env=%s | module=%s | action=%s | %s\n",
		time.Now().Format("2006-01-02T15:04:05Z"), currentUser, envName, moduleName, action, detail)

	f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
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

// Provision sets up a new server with basic requirements
func (d *Deployer) Provision(envName, moduleName string, serverNum int, opts ProvisionOptions) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	for _, srv := range servers {
		logger.Header("Provisioning %s (server-%d)", srv.Host, srv.Number)

		// Step 1: Set timezone
		if opts.Timezone != "" {
			logger.ServerAction(srv.Host, "Setting timezone to %s", opts.Timezone)
			cmd := fmt.Sprintf("sudo timedatectl set-timezone %s 2>/dev/null || sudo ln -sf /usr/share/zoneinfo/%s /etc/localtime", opts.Timezone, opts.Timezone)
			if _, err := d.ssh.Exec(env, srv.Host, cmd); err != nil {
				logger.Warn("[%s] Failed to set timezone: %v", srv.Host, err)
			}
		}

		// Step 2: Set locale
		if opts.Locale != "" {
			logger.ServerAction(srv.Host, "Setting locale to %s", opts.Locale)
			cmd := fmt.Sprintf(`sudo localectl set-locale LANG=%s 2>/dev/null || echo "LANG=%s" | sudo tee /etc/locale.conf >/dev/null`, opts.Locale, opts.Locale)
			if _, err := d.ssh.Exec(env, srv.Host, cmd); err != nil {
				logger.Warn("[%s] Failed to set locale: %v", srv.Host, err)
			}
		}

		// Step 3: Install JRE if needed
		if opts.InstallJRE {
			logger.ServerAction(srv.Host, "Installing JRE")
			cmd := `
if command -v java &>/dev/null; then
    echo "JRE_ALREADY_INSTALLED: $(java -version 2>&1 | head -1)"
else
    if command -v apt-get &>/dev/null; then
        sudo apt-get update -qq && sudo apt-get install -y -qq default-jre
    elif command -v yum &>/dev/null; then
        sudo yum install -y java-17-amazon-corretto-headless 2>/dev/null || sudo yum install -y java-17-openjdk-headless
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y java-17-openjdk-headless
    else
        echo "ERROR: No package manager found"
        exit 1
    fi
    echo "JRE_INSTALLED: $(java -version 2>&1 | head -1)"
fi
`
			result, err := d.ssh.Exec(env, srv.Host, cmd)
			if err != nil {
				logger.Warn("[%s] Failed to install JRE: %v", srv.Host, err)
			} else {
				for _, line := range strings.Split(result.Stdout, "\n") {
					if strings.HasPrefix(line, "JRE_") {
						logger.Success("[%s] %s", srv.Host, line)
					}
				}
			}
		}

		// Step 4: Install essential tools
		if opts.InstallTools {
			logger.ServerAction(srv.Host, "Installing essential tools")
			cmd := `
if command -v apt-get &>/dev/null; then
    sudo apt-get install -y -qq lsof nc curl tar gzip
elif command -v yum &>/dev/null; then
    sudo yum install -y lsof nc curl tar gzip
fi
echo "TOOLS_OK"
`
			d.ssh.Exec(env, srv.Host, cmd)
		}

		// Step 5: Module-specific initialization
		mod := d.cfg.Modules[moduleName]
		if mod != nil {
			d.provisionModule(env, srv, moduleName, mod)
		}

		// Step 6: Create base directory structure
		logger.ServerAction(srv.Host, "Creating base directories")
		if err := d.Init(envName, moduleName, srv.Number); err != nil {
			logger.Warn("[%s] Init failed: %v", srv.Host, err)
		}

		logger.Success("[%s] Provisioning complete", srv.Host)
	}

	return nil
}

// provisionModule handles module-type-specific server initialization
func (d *Deployer) provisionModule(env *config.Environment, srv config.Server, moduleName string, mod *config.Module) {
	baseDir := d.RemoteBaseDirForServer(moduleName, srv)

	// Check if there's a plugin definition with provision steps
	pluginDef := module.GetPluginDef(mod.Type)
	if pluginDef != nil && (len(pluginDef.Provision.Packages) > 0 || len(pluginDef.Provision.Directories) > 0 || len(pluginDef.Provision.Commands) > 0) {
		logger.ServerAction(srv.Host, "Running plugin provisioning for %s (%s)", moduleName, mod.Type)

		// Install packages
		if len(pluginDef.Provision.Packages) > 0 {
			pkgList := strings.Join(pluginDef.Provision.Packages, " ")
			cmd := fmt.Sprintf(`
if command -v apt-get &>/dev/null; then
    sudo apt-get install -y -qq %s
elif command -v yum &>/dev/null; then
    sudo yum install -y %s
elif command -v dnf &>/dev/null; then
    sudo dnf install -y %s
fi
`, pkgList, pkgList, pkgList)
			d.ssh.Exec(env, srv.Host, cmd)
		}

		// Create directories
		if len(pluginDef.Provision.Directories) > 0 {
			var dirs []string
			for _, dir := range pluginDef.Provision.Directories {
				expanded := strings.ReplaceAll(dir, "{{BASE_DIR}}", baseDir)
				expanded = strings.ReplaceAll(expanded, "{{MODULE}}", moduleName)
				dirs = append(dirs, expanded)
			}
			d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " ")))
		}

		// Run custom commands
		for _, cmd := range pluginDef.Provision.Commands {
			expanded := strings.ReplaceAll(cmd, "{{BASE_DIR}}", baseDir)
			expanded = strings.ReplaceAll(expanded, "{{MODULE}}", moduleName)
			expanded = strings.ReplaceAll(expanded, "{{PORT}}", fmt.Sprintf("%d", mod.Port))
			d.ssh.Exec(env, srv.Host, expanded)
		}

		logger.Success("[%s] Plugin provisioning complete for %s", srv.Host, mod.Type)
	} else {
		// Built-in handler provisioning
		switch mod.Type {
		case "kafka":
			logger.ServerAction(srv.Host, "Setting up Kafka directories")
			d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s/data/kafka-logs", baseDir))

		case "redis":
			logger.ServerAction(srv.Host, "Setting up Redis directories")
			d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s/data", baseDir))

		case "springboot", "java":
			logger.ServerAction(srv.Host, "Verifying Java for %s", moduleName)
			result, _ := d.ssh.Exec(env, srv.Host, "java -version 2>&1 | head -1")
			if result != nil && result.Stdout != "" {
				logger.Success("[%s] Java: %s", srv.Host, strings.TrimSpace(result.Stdout))
			} else {
				logger.Warn("[%s] Java not found — run with --jre flag to install", srv.Host)
			}
		}
	}

	// Create persistent data directories for all module types
	if len(mod.DataDirs) > 0 {
		var dirs []string
		for _, dir := range mod.DataDirs {
			dirs = append(dirs, fmt.Sprintf("%s/%s", baseDir, dir))
		}
		d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " ")))
	}
}

// ProvisionOptions configures what to provision on a server
type ProvisionOptions struct {
	Timezone     string // e.g., "Asia/Seoul"
	Locale       string // e.g., "en_US.UTF-8"
	InstallJRE   bool
	InstallTools bool // lsof, nc, curl, etc.
}

// Notify sends a notification about a deployment event
func (d *Deployer) Notify(envName, moduleName, event, message string) {
	cfg := d.cfg
	if cfg.Notifications == nil {
		return
	}

	for _, n := range cfg.Notifications {
		var curlCmd string
		text := fmt.Sprintf("[%s] %s/%s: %s — %s", cfg.Project.Name, envName, moduleName, event, message)

		switch n.Type {
		case "webhook":
			payload := fmt.Sprintf(`{"project":"%s","environment":"%s","module":"%s","event":"%s","message":"%s","timestamp":"%s"}`,
				cfg.Project.Name, envName, moduleName, event, message, time.Now().Format(time.RFC3339))
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, n.URL)
		case "slack":
			payload := fmt.Sprintf(`{"text":"%s"}`, text)
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, n.URL)
		case "discord":
			payload := fmt.Sprintf(`{"content":"%s"}`, text)
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, n.URL)
		case "telegram":
			// Telegram Bot API: URL format should be https://api.telegram.org/bot<TOKEN>/sendMessage
			// n.URL = bot token, n.ChatID = chat ID
			// Or use full webhook URL: https://api.telegram.org/bot{TOKEN}/sendMessage?chat_id={CHAT_ID}
			payload := fmt.Sprintf(`{"chat_id":"%s","text":"%s","parse_mode":"HTML"}`, n.ChatID, text)
			apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.URL)
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, apiURL)
		default:
			continue
		}
		go func(cmd string) {
			_ = runShell(cmd)
		}(curlCmd)
	}
}

// waitForHealthy polls until the service is healthy or timeout
func (d *Deployer) waitForHealthy(env *config.Environment, host, moduleName string) error {
	mod := d.cfg.Modules[moduleName]
	hc := mod.HealthCheck

	if hc.Type == "" && mod.Port > 0 {
		hc.Type = "tcp"
		hc.Target = fmt.Sprintf(":%d", mod.Port)
	}

	if hc.Type == "" {
		logger.Debug("No health check configured for %s, skipping", moduleName)
		return nil
	}

	timeout := time.Duration(hc.Timeout) * time.Second
	interval := time.Duration(hc.Interval) * time.Second
	maxRetries := hc.Retries
	if maxRetries <= 0 {
		maxRetries = int(timeout.Seconds() / interval.Seconds())
	}

	logger.Info("Waiting for %s to become healthy (timeout: %s, max retries: %d)...", moduleName, timeout, maxRetries)

	attempt := 0
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) && attempt < maxRetries {
		attempt++
		var checkCmd string
		switch hc.Type {
		case "tcp":
			port := mod.Port
			checkCmd = fmt.Sprintf("nc -z localhost %d 2>/dev/null && echo 'HEALTHY' || echo 'UNHEALTHY'", port)
		case "http":
			checkCmd = fmt.Sprintf("curl -sf %s >/dev/null 2>&1 && echo 'HEALTHY' || echo 'UNHEALTHY'", hc.Target)
		case "log":
			baseDir := d.remoteBaseDir(moduleName)
			logPath := mod.LogPath
			if logPath == "" {
				logDir := d.cfg.Defaults.LogDir
				if logDir == "" {
					logDir = "log"
				}
				logFile := d.cfg.Defaults.LogFile
				if logFile == "" {
					logFile = "std.log"
				}
				logPath = filepath.Join(baseDir, logDir, logFile)
			}
			checkCmd = fmt.Sprintf("grep -q '%s' %s 2>/dev/null && echo 'HEALTHY' || echo 'UNHEALTHY'", hc.Target, logPath)
		case "command":
			checkCmd = fmt.Sprintf("%s && echo 'HEALTHY' || echo 'UNHEALTHY'", hc.Target)
		default:
			return fmt.Errorf("unknown health check type: %s", hc.Type)
		}

		result, err := d.ssh.Exec(env, host, checkCmd)
		if err == nil && strings.Contains(result.Stdout, "HEALTHY") {
			return nil
		}

		time.Sleep(interval)
	}

	return fmt.Errorf("health check timeout after %s", timeout)
}

// execHook runs a lifecycle hook
func (d *Deployer) execHook(env *config.Environment, host, name, command string) {
	logger.Debug("[%s] Running hook: %s", host, name)
	result, err := d.ssh.Exec(env, host, command)
	if err != nil {
		logger.Warn("[%s] Hook %s failed: %v", host, name, err)
	} else if result.ExitCode != 0 {
		logger.Warn("[%s] Hook %s exited with code %d: %s", host, name, result.ExitCode, result.Stderr)
	}
}

// runShell runs a shell command locally (for notifications)
// getGitInfo returns current commit hash, branch, and last commit message
func getGitInfo() (commit, branch, message string) {
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "log", "-1", "--format=%s").Output(); err == nil {
		message = strings.TrimSpace(string(out))
	}
	return
}

func runShell(command string) error {
	return exec.Command("sh", "-c", command).Run()
}

// expandConfigDir copies a config directory to a temp location,
// expanding ${VAR} environment variables in all text files.
// This allows secrets to live in env vars instead of config files committed to git.
func expandConfigDir(srcDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "tow-config-*")
	if err != nil {
		return "", err
	}

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(srcDir, path)
		destPath := filepath.Join(tmpDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		content := string(data)

		// Only expand if file contains ${...} patterns (skip binaries)
		if strings.Contains(content, "${") {
			content = os.ExpandEnv(content)
		}

		return os.WriteFile(destPath, []byte(content), info.Mode())
	})

	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}
