package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/ssh"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Read or stream logs from one or multiple servers",
		Long: `Read or stream log output from module servers.

Examples:
  tow logs -e prod -m kafka -n 20              # last 20 lines from first server
  tow logs -e prod -m kafka -s kafka-1 -F      # stream from specific server
  tow logs -e prod -m kafka --all -F           # stream from ALL servers (multiplexed)
  tow logs -e prod -m kafka -s kafka-1,kafka-3 # multiple specific servers
  tow logs -e prod -m kafka -f ERROR           # filter for ERROR
  tow logs --preset infra-logs -F              # use saved preset
  tow logs --list-presets                       # show saved presets`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle preset listing
			listPresets, _ := cmd.Flags().GetBool("list-presets")
			if listPresets {
				return showPresets()
			}

			deletePreset, _ := cmd.Flags().GetString("delete-preset")
			if deletePreset != "" {
				return removePreset(deletePreset)
			}

			filter, _ := cmd.Flags().GetString("filter")
			lines, _ := cmd.Flags().GetInt("lines")
			follow, _ := cmd.Flags().GetBool("follow")

			// Handle preset loading
			presetName, _ := cmd.Flags().GetString("preset")
			if presetName != "" {
				return runPreset(cmd, presetName, filter, lines, follow)
			}

			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			envName, _ := cmd.Flags().GetString("environment")
			modName, _ := cmd.Flags().GetString("module")
			serverFlag, _ := cmd.Flags().GetString("server")
			allServers, _ := cmd.Flags().GetBool("all")

			if envName == "" {
				return fmt.Errorf("environment (-e) is required")
			}
			if modName == "" {
				return fmt.Errorf("module (-m) is required")
			}

			deployer := deploy.New(cfg, sshMgr)

			// Multi-module support: -m kafka,zookeeper
			if strings.Contains(modName, ",") {
				moduleNames := strings.Split(modName, ",")
				var allTargetServers []config.Server
				var serverModuleMap []string // parallel array: module name per server

				for _, mn := range moduleNames {
					mn = strings.TrimSpace(mn)
					servers, _, err := cfg.GetServersForModule(envName, mn, 0)
					if err != nil {
						logger.Warn("Module %s: %v", mn, err)
						continue
					}
					for _, srv := range servers {
						allTargetServers = append(allTargetServers, srv)
						serverModuleMap = append(serverModuleMap, mn)
					}
				}

				if len(allTargetServers) == 0 {
					return fmt.Errorf("no servers found for modules: %s", modName)
				}

				return deployer.LogsMultiModule(envName, allTargetServers, serverModuleMap, filter, lines, follow)
			}

			var logsErr error
			if allServers {
				logsErr = deployer.Logs(envName, modName, 0, filter, lines, follow)
			} else if serverFlag != "" && strings.Contains(serverFlag, ",") {
				serverNames := strings.Split(serverFlag, ",")
				servers, _, err := cfg.GetServersForModule(envName, modName, 0)
				if err != nil {
					return err
				}
				var filtered []config.Server
				for _, srv := range servers {
					for _, name := range serverNames {
						if srv.ID() == strings.TrimSpace(name) {
							filtered = append(filtered, srv)
						}
					}
				}
				if len(filtered) == 0 {
					return fmt.Errorf("no matching servers found for: %s", serverFlag)
				}
				logsErr = deployer.LogsForServers(envName, modName, filtered, filter, lines, follow)
			} else {
				serverNum := 0
				if serverFlag != "" {
					fmt.Sscanf(serverFlag, "%d", &serverNum)
				}
				logsErr = deployer.Logs(envName, modName, serverNum, filter, lines, follow)
			}

			// Save preset if requested
			savePresetName, _ := cmd.Flags().GetString("save-preset")
			if savePresetName != "" && logsErr == nil {
				saveLogPreset(savePresetName, envName, modName, serverFlag, allServers, filter, lines)
			}

			return logsErr
		},
	}
	cmd.Flags().StringP("filter", "f", "", "grep filter for log output")
	cmd.Flags().IntP("lines", "n", 20, "number of tail lines")
	cmd.Flags().BoolP("follow", "F", false, "follow log output (stream mode)")
	cmd.Flags().BoolP("all", "A", false, "show logs from all servers")
	cmd.Flags().String("preset", "", "use a saved log preset")
	cmd.Flags().String("save-preset", "", "save current log config as a preset")
	cmd.Flags().Bool("list-presets", false, "list saved presets")
	cmd.Flags().String("delete-preset", "", "delete a saved preset")
	return cmd
}

// --- Log Presets ---

type logPreset struct {
	Env     string `yaml:"env"`
	Module  string `yaml:"module"`
	Servers string `yaml:"servers"` // comma-separated or "all"
	Filter  string `yaml:"filter,omitempty"`
	Lines   int    `yaml:"lines"`
}

type presetFile struct {
	Presets map[string]logPreset `yaml:"presets"`
}

func presetsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tow", "presets.yaml")
}

func loadPresets() (presetFile, error) {
	var pf presetFile
	pf.Presets = make(map[string]logPreset)

	data, err := os.ReadFile(presetsPath())
	if err != nil {
		return pf, nil // file doesn't exist = empty presets
	}
	yaml.Unmarshal(data, &pf)
	if pf.Presets == nil {
		pf.Presets = make(map[string]logPreset)
	}
	return pf, nil
}

func savePresets(pf presetFile) error {
	dir := filepath.Dir(presetsPath())
	os.MkdirAll(dir, 0755)
	data, err := yaml.Marshal(pf)
	if err != nil {
		return err
	}
	return os.WriteFile(presetsPath(), data, 0644)
}

func saveLogPreset(name, env, mod, servers string, all bool, filter string, lines int) {
	pf, _ := loadPresets()
	s := servers
	if all {
		s = "all"
	}
	pf.Presets[name] = logPreset{Env: env, Module: mod, Servers: s, Filter: filter, Lines: lines}
	if err := savePresets(pf); err != nil {
		logger.Warn("Failed to save preset: %v", err)
	} else {
		logger.Success("Preset saved: %s", name)
	}
}

func showPresets() error {
	pf, _ := loadPresets()
	if len(pf.Presets) == 0 {
		fmt.Println("No presets saved. Create one with: tow logs ... --save-preset NAME")
		return nil
	}
	for name, p := range pf.Presets {
		fmt.Printf("  %-20s  env=%s module=%s servers=%s", name, p.Env, p.Module, p.Servers)
		if p.Filter != "" {
			fmt.Printf(" filter=%q", p.Filter)
		}
		fmt.Println()
	}
	return nil
}

func removePreset(name string) error {
	pf, _ := loadPresets()
	if _, ok := pf.Presets[name]; !ok {
		return fmt.Errorf("preset %q not found", name)
	}
	delete(pf.Presets, name)
	if err := savePresets(pf); err != nil {
		return err
	}
	logger.Success("Preset deleted: %s", name)
	return nil
}

func runPreset(cmd *cobra.Command, name, filterOverride string, linesOverride int, followOverride bool) error {
	pf, _ := loadPresets()
	p, ok := pf.Presets[name]
	if !ok {
		return fmt.Errorf("preset %q not found. Use --list-presets to see available", name)
	}

	cfg, sshMgr, err := loadContextFromPath(cmd)
	if err != nil {
		return err
	}
	defer sshMgr.Close()

	filter := p.Filter
	if filterOverride != "" {
		filter = filterOverride
	}
	lines := p.Lines
	if linesOverride > 0 {
		lines = linesOverride
	}

	deployer := deploy.New(cfg, sshMgr)

	if p.Servers == "all" {
		return deployer.Logs(p.Env, p.Module, 0, filter, lines, followOverride)
	}

	if strings.Contains(p.Servers, ",") {
		serverNames := strings.Split(p.Servers, ",")
		servers, _, err := cfg.GetServersForModule(p.Env, p.Module, 0)
		if err != nil {
			return err
		}
		var filtered []config.Server
		for _, srv := range servers {
			for _, n := range serverNames {
				if srv.ID() == strings.TrimSpace(n) {
					filtered = append(filtered, srv)
				}
			}
		}
		return deployer.LogsForServers(p.Env, p.Module, filtered, filter, lines, followOverride)
	}

	serverNum := 0
	fmt.Sscanf(p.Servers, "%d", &serverNum)
	return deployer.Logs(p.Env, p.Module, serverNum, filter, lines, followOverride)
}

func loadContextFromPath(cmd *cobra.Command) (*config.Config, *ssh.Manager, error) {
	return loadContext(cmd)
}

func newSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [flags] -- <command>",
		Short: "Execute a command on remote servers",
		Long: `Run ad-hoc commands on module servers without interactive login.

Examples:
  tow ssh -e prod -m api-server -- "free -h"
  tow ssh -e prod -m kafka --all -- "df -h"
  tow ssh -e prod -m kafka -s kafka-1,kafka-2 -- "cat /etc/os-release"
  tow ssh -e prod -m api-server -- "tail -5 /var/log/syslog"`,
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			envName, _ := cmd.Flags().GetString("environment")
			modName, _ := cmd.Flags().GetString("module")
			serverFlag, _ := cmd.Flags().GetString("server")
			allServers, _ := cmd.Flags().GetBool("all")

			if envName == "" || modName == "" {
				return fmt.Errorf("environment (-e) and module (-m) are required")
			}
			if len(args) == 0 {
				return fmt.Errorf("command is required after --")
			}

			remoteCmd := strings.Join(args, " ")

			servers, env, err := cfg.GetServersForModule(envName, modName, 0)
			if err != nil {
				return err
			}

			// Filter servers
			if !allServers && serverFlag != "" {
				names := strings.Split(serverFlag, ",")
				var filtered []config.Server
				for _, srv := range servers {
					for _, n := range names {
						if srv.ID() == strings.TrimSpace(n) {
							filtered = append(filtered, srv)
						}
					}
				}
				servers = filtered
			} else if !allServers {
				servers = servers[:1] // default: first server only
			}

			if len(servers) == 0 {
				return fmt.Errorf("no matching servers")
			}

			// Execute on each server with prefix
			colorReset := logger.ColorReset
			colors := logger.ServerColors

			for i, srv := range servers {
				result, err := sshMgr.Exec(env, srv.Host, remoteCmd)
				if err != nil {
					logger.Error("[%s] %v", srv.ID(), err)
					continue
				}

				if len(servers) > 1 {
					color := colors[i%len(colors)]
					prefix := fmt.Sprintf("%s[%s]%s ", color, srv.ID(), colorReset)
					for _, line := range strings.Split(strings.TrimRight(result.Stdout, "\n"), "\n") {
						if line != "" {
							fmt.Printf("%s%s\n", prefix, line)
						}
					}
				} else {
					fmt.Print(result.Stdout)
				}

				if result.Stderr != "" {
					fmt.Fprint(os.Stderr, result.Stderr)
				}
			}

			return nil
		},
	}
	cmd.Flags().BoolP("all", "A", false, "execute on all servers")
	return cmd
}

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show what will be deployed (changes since last deployment)",
		Long: `Compare local code against the currently deployed version.

Shows git log and diff since the deployed commit, helping you review
what will change before running tow deploy.

Examples:
  tow diff -e prod -m api-server
  tow diff -e prod -m api-server --stat    # file-level summary only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			envName, modName, serverNum, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			statOnly, _ := cmd.Flags().GetBool("stat")

			servers, env, err := cfg.GetServersForModule(envName, modName, serverNum)
			if err != nil {
				return err
			}

			srv := servers[0]
			deployer := deploy.New(cfg, sshMgr)
			baseDir := deployer.RemoteBaseDirForServer(modName, srv)

			// Get currently deployed version info
			deployInfoCmd := fmt.Sprintf(`
CURRENT=$(readlink %s/current 2>/dev/null | xargs basename 2>/dev/null)
if [ -f "%s/current/.tow-deploy-info" ]; then
    cat "%s/current/.tow-deploy-info"
else
    echo "deploy_ts=$CURRENT"
    echo "commit=unknown"
fi
`, baseDir, baseDir, baseDir)

			result, err := sshMgr.Exec(env, srv.Host, deployInfoCmd)
			if err != nil {
				return fmt.Errorf("failed to get deployment info: %w", err)
			}

			// Parse deployed commit
			deployedCommit := ""
			deployedTS := ""
			for _, line := range strings.Split(result.Stdout, "\n") {
				if strings.HasPrefix(line, "commit=") {
					deployedCommit = strings.TrimPrefix(line, "commit=")
				}
				if strings.HasPrefix(line, "deploy_ts=") {
					deployedTS = strings.TrimPrefix(line, "deploy_ts=")
				}
			}

			fmt.Printf("Environment:  %s\n", envName)
			fmt.Printf("Module:       %s\n", modName)
			fmt.Printf("Server:       %s (%s)\n", srv.ID(), srv.Host)
			fmt.Printf("Deployed:     %s\n", deployedTS)

			if deployedCommit == "" || deployedCommit == "unknown" {
				fmt.Printf("Deployed commit: unknown (deploy was done before tow tracking)\n\n")
				fmt.Println("Showing recent local commits instead:")

				gitLog := exec.Command("git", "log", "--oneline", "-10")
				gitLog.Stdout = os.Stdout
				gitLog.Stderr = os.Stderr
				return gitLog.Run()
			}

			fmt.Printf("Deployed commit: %s\n\n", deployedCommit)

			// Show changes since deployed commit
			fmt.Println("Changes since last deploy:")
			fmt.Println(strings.Repeat("─", 50))

			if statOnly {
				gitDiff := exec.Command("git", "diff", "--stat", deployedCommit+"..HEAD")
				gitDiff.Stdout = os.Stdout
				gitDiff.Stderr = os.Stderr
				return gitDiff.Run()
			}

			gitLog := exec.Command("git", "log", "--oneline", deployedCommit+"..HEAD")
			gitLog.Stdout = os.Stdout
			gitLog.Stderr = os.Stderr
			if err := gitLog.Run(); err != nil {
				return err
			}

			fmt.Println()
			gitDiff := exec.Command("git", "diff", "--stat", deployedCommit+"..HEAD")
			gitDiff.Stdout = os.Stdout
			gitDiff.Stderr = os.Stderr
			return gitDiff.Run()
		},
	}
	cmd.Flags().Bool("stat", false, "show file-level summary only")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose common issues before deploying",
		Long:  `Pre-flight diagnostics: checks config, SSH connectivity, remote directories, disk space, and build tools.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			insecure, _ := cmd.Flags().GetBool("insecure")
			if verbose {
				logger.SetLevel(logger.DebugLevel)
			}

			envName, _ := cmd.Flags().GetString("environment")
			modName, _ := cmd.Flags().GetString("module")

			if envName == "" {
				return fmt.Errorf("environment (-e) is required")
			}

			passed := 0
			failed := 0

			check := func(name string, fn func() error) {
				if err := fn(); err != nil {
					fmt.Printf("  %s✗%s %s — %v\n", logger.ColorRed, logger.ColorReset, name, err)
					failed++
				} else {
					fmt.Printf("  %s✓%s %s\n", logger.ColorGreen, logger.ColorReset, name)
					passed++
				}
			}

			// 1. Config valid
			check("tow.yaml is valid", func() error {
				_, err := config.Load(cfgPath)
				return err
			})

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			// 2. Environment exists
			env, ok := cfg.Environments[envName]
			check(fmt.Sprintf("Environment '%s' exists", envName), func() error {
				if !ok {
					return fmt.Errorf("not found in config")
				}
				return nil
			})
			if !ok {
				fmt.Printf("\n%d passed, %d failed\n", passed, failed)
				return nil
			}

			// 3. SSH key
			check("SSH key exists", func() error {
				keyPath := env.SSHKeyPath
				if keyPath == "" {
					return fmt.Errorf("ssh_key_path not set")
				}
				if keyPath[0] == '~' {
					if home, err := os.UserHomeDir(); err == nil {
						keyPath = filepath.Join(home, keyPath[1:])
					}
				}
				if _, err := os.Stat(keyPath); os.IsNotExist(err) {
					return fmt.Errorf("%s not found", keyPath)
				}
				return nil
			})

			// 4. Servers exist
			check(fmt.Sprintf("Servers configured (%d)", len(env.Servers)), func() error {
				if len(env.Servers) == 0 {
					return fmt.Errorf("no servers in environment")
				}
				return nil
			})

			// Get target modules
			modules := []string{}
			if modName != "" {
				modules = append(modules, modName)
			} else {
				for name := range cfg.Modules {
					modules = append(modules, name)
				}
			}

			// 5. SSH connectivity (test first server of first module)
			sshMgr := ssh.NewManager(insecure)
			defer sshMgr.Close()

			if len(env.Servers) > 0 {
				srv := env.Servers[0]
				check(fmt.Sprintf("SSH connection to %s", srv.Host), func() error {
					result, err := sshMgr.Exec(env, srv.Host, "echo OK")
					if err != nil {
						return err
					}
					if !strings.Contains(result.Stdout, "OK") {
						return fmt.Errorf("unexpected response")
					}
					return nil
				})

				// 6. Remote directory + disk space
				for _, modN := range modules {
					servers, _, err := cfg.GetServersForModule(envName, modN, 0)
					if err != nil || len(servers) == 0 {
						continue
					}
					s := servers[0]
					deployer := deploy.New(cfg, sshMgr)
					baseDir := deployer.RemoteBaseDirForServer(modN, s)

					check(fmt.Sprintf("Remote dir exists: %s (%s)", baseDir, s.Host), func() error {
						result, err := sshMgr.Exec(env, s.Host, fmt.Sprintf("test -d %s && echo EXISTS || echo MISSING", baseDir))
						if err != nil {
							return err
						}
						if strings.Contains(result.Stdout, "MISSING") {
							return fmt.Errorf("directory not found — run: tow setup")
						}
						return nil
					})
					break // only check first module's first server
				}

				check(fmt.Sprintf("Disk space on %s", srv.Host), func() error {
					result, err := sshMgr.Exec(env, srv.Host, "df -h / | tail -1 | awk '{print $4}'")
					if err != nil {
						return err
					}
					fmt.Printf("    Available: %s\n", strings.TrimSpace(result.Stdout))
					return nil
				})
			}

			// 7. Branch check
			if modName != "" {
				check("Branch policy", func() error {
					return deploy.CheckBranch(cfg, envName, "deploy")
				})
			}

			// 8. No active lock
			if modName != "" && len(env.Servers) > 0 {
				servers, _, _ := cfg.GetServersForModule(envName, modName, 0)
				if len(servers) > 0 {
					srv := servers[0]
					deployer := deploy.New(cfg, sshMgr)
					baseDir := deployer.RemoteBaseDirForServer(modName, srv)
					check("No active deploy lock", func() error {
						result, err := sshMgr.Exec(env, srv.Host, fmt.Sprintf("test -d %s/.tow-lock && echo LOCKED || echo FREE", baseDir))
						if err != nil {
							return err
						}
						if strings.Contains(result.Stdout, "LOCKED") {
							return fmt.Errorf("deploy lock active — run: tow unlock")
						}
						return nil
					})
				}
			}

			fmt.Printf("\n%d passed, %d failed\n", passed, failed)
			if failed > 0 {
				return fmt.Errorf("%d check(s) failed", failed)
			}
			return nil
		},
	}
}

func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show deployment metrics from audit log",
		Long: `Analyze .tow/audit.log to show deployment frequency, success rate, and trends.

Examples:
  tow metrics
  tow metrics -e prod
  tow metrics -e prod -m api-server
  tow metrics --days 30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envFilter, _ := cmd.Flags().GetString("environment")
			modFilter, _ := cmd.Flags().GetString("module")
			days, _ := cmd.Flags().GetInt("days")

			auditFile := ".tow/audit.log"
			data, err := os.ReadFile(auditFile)
			if err != nil {
				return fmt.Errorf("no audit log found at %s — deploy something first", auditFile)
			}

			type entry struct {
				time   time.Time
				env    string
				module string
				action string
			}

			cutoff := time.Now().AddDate(0, 0, -days)
			var entries []entry
			moduleCounts := make(map[string]int)
			actionCounts := make(map[string]int)
			dayCounts := make(map[string]int)

			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					continue
				}

				parts := strings.Split(line, " | ")
				if len(parts) < 5 {
					continue
				}

				ts, err := time.Parse("2006-01-02T15:04:05Z", strings.TrimSpace(parts[0]))
				if err != nil {
					continue
				}

				if ts.Before(cutoff) {
					continue
				}

				e := entry{time: ts}
				for _, p := range parts[1:] {
					kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
					if len(kv) != 2 {
						continue
					}
					switch kv[0] {
					case "env":
						e.env = kv[1]
					case "module":
						e.module = kv[1]
					case "action":
						e.action = kv[1]
					}
				}

				if envFilter != "" && e.env != envFilter {
					continue
				}
				if modFilter != "" && e.module != modFilter {
					continue
				}

				entries = append(entries, e)
				moduleCounts[e.module]++
				actionCounts[e.action]++
				dayCounts[ts.Weekday().String()[:3]]++
			}

			if len(entries) == 0 {
				fmt.Println("No deployments found in the specified period.")
				return nil
			}

			// Summary
			fmt.Printf("\nDeployments (last %d days):\n", days)
			fmt.Printf("  Total:        %d\n", len(entries))

			// By action
			fmt.Printf("\nBy action:\n")
			for action, count := range actionCounts {
				fmt.Printf("  %-12s  %d\n", action, count)
			}

			// By module
			fmt.Printf("\nBy module:\n")
			maxCount := 0
			for _, c := range moduleCounts {
				if c > maxCount {
					maxCount = c
				}
			}
			for mod, count := range moduleCounts {
				bar := strings.Repeat("█", count*20/max(maxCount, 1))
				fmt.Printf("  %-30s  %s %d\n", mod, bar, count)
			}

			// By day of week
			fmt.Printf("\nBy day:\n")
			weekdays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
			maxDay := 0
			for _, c := range dayCounts {
				if c > maxDay {
					maxDay = c
				}
			}
			for _, day := range weekdays {
				count := dayCounts[day]
				if count > 0 {
					bar := strings.Repeat("█", count*20/max(maxDay, 1))
					fmt.Printf("  %s  %s %d\n", day, bar, count)
				}
			}

			fmt.Println()
			return nil
		},
	}
	cmd.Flags().Int("days", 30, "number of days to analyze")
	return cmd
}
