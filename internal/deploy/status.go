package deploy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/module"
)

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
