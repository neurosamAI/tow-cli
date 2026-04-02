package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
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
