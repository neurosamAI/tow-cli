package deploy

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
)

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
