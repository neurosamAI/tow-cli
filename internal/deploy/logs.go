package deploy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
)

// serverLogColors aliases logger.ServerColors for multi-server log output
var serverLogColors = logger.ServerColors

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

// LogsMultiModule streams logs from multiple modules across their servers
func (d *Deployer) LogsMultiModule(envName string, servers []config.Server, moduleNames []string, filter string, lines int, follow bool) error {
	env, ok := d.cfg.Environments[envName]
	if !ok {
		return fmt.Errorf("environment %q not found", envName)
	}

	colorReset := logger.ColorReset

	if !follow {
		for i, srv := range servers {
			modName := moduleNames[i]
			logPath := d.resolveLogPath(env, srv, modName)
			color := serverLogColors[i%len(serverLogColors)]
			prefix := fmt.Sprintf("%s[%s/%s]%s ", color, modName, srv.ID(), colorReset)

			tailCmd := fmt.Sprintf("tail -n %d %s", lines, logPath)
			if filter != "" {
				tailCmd += fmt.Sprintf(" | grep '%s'", filter)
			}

			result, err := d.ssh.Exec(env, srv.Host, tailCmd)
			if err != nil {
				logger.Warn("[%s/%s] failed to read logs: %v", modName, srv.ID(), err)
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

	// Follow mode
	logger.Info("Streaming logs from %d module/server pairs. Press Ctrl+C to stop.\n", len(servers))

	var wg sync.WaitGroup
	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, s config.Server, modName string) {
			defer wg.Done()

			logPath := d.resolveLogPath(env, s, modName)
			color := serverLogColors[idx%len(serverLogColors)]
			prefix := fmt.Sprintf("%s[%s/%s]%s ", color, modName, s.ID(), colorReset)

			tailCmd := fmt.Sprintf("tail -n %d -f %s", lines, logPath)
			if filter != "" {
				tailCmd += fmt.Sprintf(" | grep --line-buffered '%s'", filter)
			}

			pw := &prefixWriter{prefix: prefix, out: os.Stdout}
			if err := d.ssh.ExecStream(env, s.Host, tailCmd, pw, pw); err != nil {
				logger.Warn("[%s/%s] log stream ended: %v", modName, s.ID(), err)
			}
		}(i, srv, moduleNames[i])
	}

	wg.Wait()
	return nil
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
	colorReset := logger.ColorReset

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
