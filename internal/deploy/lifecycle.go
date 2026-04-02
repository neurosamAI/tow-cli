package deploy

import (
	"fmt"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
)

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
