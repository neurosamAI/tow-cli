package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	mcpserver "github.com/neurosamAI/tow-cli/integrations/mcp-server"
	"github.com/neurosamAI/tow-cli/internal/config"

	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/initializer"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/module"
	"github.com/neurosamAI/tow-cli/internal/pipeline"
	"github.com/neurosamAI/tow-cli/internal/ssh"
	_ "github.com/neurosamAI/tow-cli/plugins" // auto-register bundled plugins

	"github.com/spf13/cobra"
)

var (
	Version   = "0.1.0"
	BuildDate = "dev"
	cfgFile   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "tow",
		Short: "Tow - Lightweight deployment orchestrator",
		Long: `Tow is a lightweight, agentless deployment orchestrator for teams
that manage services on bare-metal servers or cloud VMs without Kubernetes.

It provides symlink-based atomic deployments with instant rollback,
multi-environment configuration management, and SSH-based remote execution.

Created by Murry Jeong (comchangs) — https://github.com/comchangs
Supported by neurosam.AI — https://neurosam.ai`,
		Version: fmt.Sprintf("%s (built %s)", Version, BuildDate),
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "tow.yaml", "config file path")
	rootCmd.PersistentFlags().StringP("environment", "e", "", "target environment (e.g., dev, test, prod)")
	rootCmd.PersistentFlags().StringP("module", "m", "", "target module name")
	rootCmd.PersistentFlags().StringP("server", "s", "", "target server name or number (empty = all servers)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().Bool("dry-run", false, "show what would be done without executing")
	rootCmd.PersistentFlags().Bool("insecure", false, "skip SSH host key verification")

	rootCmd.AddCommand(
		newInitCmd(),
		newValidateCmd(),
		newBuildCmd(),
		newPackageCmd(),
		newDeployCmd(),
		newAutoCmd(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newStatusCmd(),
		newRollbackCmd(),
		newLogsCmd(),
		newSetupCmd(),
		newUploadCmd(),
		newInstallCmd(),
		newListCmd(),
		newLoginCmd(),
		newUnlockCmd(),
		newCleanupCmd(),
		newDownloadCmd(),
		newProvisionCmd(),
		newThreadDumpCmd(),
		newPluginCmd(),
		newSSHCmd(),
		newDiffCmd(),
		newConfigCmd(),
		newMetricsCmd(),
		newDoctorCmd(),
		newMCPServerCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// loadContext loads config and creates SSH manager for command execution
func loadContext(cmd *cobra.Command) (*config.Config, *ssh.Manager, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	insecure, _ := cmd.Flags().GetBool("insecure")

	if verbose {
		logger.SetLevel(logger.DebugLevel)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	if insecure {
		logger.Warn("SSH host key verification DISABLED (--insecure). This is vulnerable to MITM attacks.")
		logger.Warn("Not recommended for production. Use ssh-keyscan to populate ~/.ssh/known_hosts instead.")
	}

	sshMgr := ssh.NewManager(insecure)
	sshMgr.DryRun = dryRun

	if dryRun {
		logger.Info("[DRY-RUN] Dry run mode enabled — no changes will be made")
		pipeline.SetDryRun(true)
	}

	return cfg, sshMgr, nil
}

// resolveTargets resolves environment, module, and server from flags.
// Returns serverName (string) and serverNum (int) — name takes priority.
func resolveTargets(cmd *cobra.Command, cfg *config.Config) (string, string, int, error) {
	env, _ := cmd.Flags().GetString("environment")
	mod, _ := cmd.Flags().GetString("module")
	serverFlag, _ := cmd.Flags().GetString("server")

	if env == "" {
		return "", "", 0, fmt.Errorf("environment (-e) is required")
	}
	if mod == "" {
		return "", "", 0, fmt.Errorf("module (-m) is required")
	}

	if _, ok := cfg.Environments[env]; !ok {
		return "", "", 0, fmt.Errorf("environment %q not found in config", env)
	}

	// Parse server flag: number or name
	serverNum := 0
	if serverFlag != "" {
		if n, err := fmt.Sscanf(serverFlag, "%d", &serverNum); n == 0 || err != nil {
			// Not a number — treat as server name, store in serverNum=0
			// The name will be resolved in resolveServerName
			serverNum = -1 // signal to use name
		}
	}

	return env, mod, serverNum, nil
}

// confirmProdDeploy asks for user confirmation when deploying to production-like environments
func confirmProdDeploy(cmd *cobra.Command, envName, moduleName, command string) bool {
	// Skip confirmation in dry-run mode
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return true
	}

	prodEnvs := map[string]bool{"prod": true, "production": true, "live": true}
	if !prodEnvs[strings.ToLower(envName)] {
		return true
	}

	fmt.Fprintf(os.Stderr, "\n%s⚠  WARNING: You are about to %s %s in %s%s\n",
		"\033[33m", command, moduleName, strings.ToUpper(envName), "\033[0m")
	fmt.Fprintf(os.Stderr, "  Type 'yes' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	return input == "yes"
}

// withBranchCheck wraps a command function with branch verification
func withBranchCheck(cmdName string, fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfgPath, _ := cmd.Flags().GetString("config")
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		env, _ := cmd.Flags().GetString("environment")
		if env != "" {
			if err := deploy.CheckBranch(cfg, env, cmdName); err != nil {
				return err
			}
		}

		return fn(cmd, args)
	}
}

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Detect project type and generate tow.yaml",
		Long: `Scans the current directory to detect the project type (Java/Gradle,
Spring Boot, Node.js, Python, Go, etc.) and generates a tow.yaml
configuration file with sensible defaults.

Supports both single-project and multi-module (mono-repo) structures.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				logger.SetLevel(logger.DebugLevel)
			}

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			force, _ := cmd.Flags().GetBool("force")
			withAI, _ := cmd.Flags().GetBool("with-ai")
			return initializer.Init(dir, force, withAI)
		},
	}
	cmd.Flags().Bool("force", false, "overwrite existing tow.yaml")
	cmd.Flags().Bool("with-ai", false, "generate Claude Code skill and MCP server config")
	return cmd
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate tow.yaml configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				logger.SetLevel(logger.DebugLevel)
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			warnings := cfg.ValidateDetailed()

			envCount := len(cfg.Environments)
			modCount := len(cfg.Modules)
			serverCount := 0
			for _, env := range cfg.Environments {
				serverCount += len(env.Servers)
			}

			fmt.Printf("Project:      %s\n", cfg.Project.Name)
			fmt.Printf("Environments: %d\n", envCount)
			fmt.Printf("Modules:      %d\n", modCount)
			fmt.Printf("Servers:      %d (total)\n", serverCount)
			fmt.Println()

			if len(warnings) > 0 {
				fmt.Printf("Warnings (%d):\n", len(warnings))
				for _, w := range warnings {
					fmt.Printf("  - %s\n", w)
				}
				fmt.Println()
			}

			logger.Success("Configuration is valid")
			return nil
		},
	}
}

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build a module locally (compile/jar/binary)",
		Long:  `Runs the module's build command locally. Use this to build without deploying.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, _, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			p := pipeline.New(cfg, sshMgr)
			return p.Build(mod, env)
		},
	}
}

func newPackageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "package",
		Short: "Package a module into a deployment artifact (tar.gz)",
		Long:  `Creates the tar.gz artifact for a module. Use this after build, before upload.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, _, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			p := pipeline.New(cfg, sshMgr)
			return p.Package(mod, env)
		},
	}
}

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a module (package → upload → install → restart)",
		RunE: withBranchCheck("deploy", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes && !confirmProdDeploy(cmd, env, mod, "deploy") {
				return fmt.Errorf("deployment cancelled")
			}

			rolling, _ := cmd.Flags().GetBool("rolling")

			deployer := deploy.New(cfg, sshMgr)
			return deployer.WithLock(env, mod, server, "deploy", func() error {
				deployer.WriteAuditLog(env, mod, "deploy", fmt.Sprintf("rolling=%v", rolling))
				p := pipeline.New(cfg, sshMgr)
				p.Rolling = rolling
				return p.Deploy(env, mod, server)
			})
		}),
	}
	cmd.Flags().Bool("rolling", false, "use rolling deployment (one server at a time)")
	cmd.Flags().BoolP("yes", "y", false, "skip production confirmation prompt")
	return cmd
}

func newAutoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auto",
		Short: "Full pipeline: build → package → upload → install → restart",
		RunE: withBranchCheck("auto", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes && !confirmProdDeploy(cmd, env, mod, "auto deploy") {
				return fmt.Errorf("deployment cancelled")
			}

			rolling, _ := cmd.Flags().GetBool("rolling")
			autoRollback, _ := cmd.Flags().GetBool("auto-rollback")

			deployer := deploy.New(cfg, sshMgr)
			return deployer.WithLock(env, mod, server, "auto", func() error {
				deployer.WriteAuditLog(env, mod, "auto", fmt.Sprintf("rolling=%v auto-rollback=%v", rolling, autoRollback))
				p := pipeline.New(cfg, sshMgr)
				p.Rolling = rolling
				if autoRollback {
					return p.AutoWithRollback(env, mod, server)
				}
				return p.Auto(env, mod, server)
			})
		}),
	}
	cmd.Flags().Bool("rolling", false, "use rolling deployment (one server at a time)")
	cmd.Flags().Bool("auto-rollback", false, "automatically rollback if health check fails after start")
	cmd.Flags().BoolP("yes", "y", false, "skip production confirmation prompt")
	return cmd
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a module on target servers",
		RunE: withBranchCheck("start", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			rolling, _ := cmd.Flags().GetBool("rolling")
			deployer := deploy.New(cfg, sshMgr)
			if rolling {
				return deployer.StartRolling(env, mod, server)
			}
			return deployer.Start(env, mod, server)
		}),
	}
	cmd.Flags().Bool("rolling", false, "start one server at a time with health check gates")
	return cmd
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop a module on target servers",
		RunE: withBranchCheck("stop", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Stop(env, mod, server)
		}),
	}
}

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart a module (stop → start)",
		RunE: withBranchCheck("restart", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Restart(env, mod, server)
		}),
	}
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check module status on target servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			envName, _ := cmd.Flags().GetString("environment")
			modName, _ := cmd.Flags().GetString("module")
			serverFlag, _ := cmd.Flags().GetString("server")

			if envName == "" {
				return fmt.Errorf("environment (-e) is required")
			}

			output, _ := cmd.Flags().GetString("output")
			deployer := deploy.New(cfg, sshMgr)

			// If no module specified, show status of ALL modules
			if modName == "" {
				logger.Header("Status: all modules in %s", envName)
				for name := range cfg.Modules {
					serverNum := 0
					if serverFlag != "" {
						fmt.Sscanf(serverFlag, "%d", &serverNum)
					}
					// Only show modules that have servers in this env
					servers, _, err := cfg.GetServersForModule(envName, name, serverNum)
					if err != nil || len(servers) == 0 {
						continue
					}
					fmt.Printf("\n  [%s]\n", name)
					deployer.Status(envName, name, serverNum)
				}
				return nil
			}

			serverNum := 0
			if serverFlag != "" {
				fmt.Sscanf(serverFlag, "%d", &serverNum)
			}

			if output == "json" {
				jsonStr, err := deployer.StatusJSON(envName, modName, serverNum)
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
				return nil
			}

			return deployer.Status(envName, modName, serverNum)
		},
	}
	cmd.Flags().StringP("output", "o", "", "output format (json)")
	return cmd
}

func newRollbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback to a previous deployment",
		RunE: withBranchCheck("rollback", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			yes, _ := cmd.Flags().GetBool("yes")
			if !yes && !confirmProdDeploy(cmd, env, mod, "rollback") {
				return fmt.Errorf("rollback cancelled")
			}

			target, _ := cmd.Flags().GetString("target")
			interactive, _ := cmd.Flags().GetBool("interactive")

			deployer := deploy.New(cfg, sshMgr)

			// Interactive mode: list deployments and let user pick
			if interactive && target == "" {
				jsonStr, err := deployer.ListDeploymentsJSON(env, mod, server)
				if err != nil {
					return err
				}

				type entry struct {
					Timestamp string `json:"timestamp"`
					Current   bool   `json:"current"`
				}
				var entries []entry
				if err := json.Unmarshal([]byte(jsonStr), &entries); err != nil {
					return err
				}

				var options []string
				for _, e := range entries {
					label := e.Timestamp
					if e.Current {
						label += " (current)"
					}
					options = append(options, label)
				}

				if len(options) == 0 {
					return fmt.Errorf("no deployments found")
				}

				fmt.Println("Available deployments:")
				for i, opt := range options {
					fmt.Printf("  [%d] %s\n", i+1, opt)
				}
				fmt.Print("\nSelect deployment number: ")

				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)

				var idx int
				if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(entries) {
					return fmt.Errorf("invalid selection: %s", input)
				}

				selected := entries[idx-1]
				if selected.Current {
					return fmt.Errorf("cannot rollback to current deployment")
				}
				target = selected.Timestamp
				fmt.Printf("Rolling back to: %s\n", target)
			}

			return deployer.WithLock(env, mod, server, "rollback", func() error {
				deployer.WriteAuditLog(env, mod, "rollback", fmt.Sprintf("target=%s", target))
				return deployer.Rollback(env, mod, server, target)
			})
		}),
	}
	cmd.Flags().StringP("target", "t", "", "target deployment timestamp to rollback to (empty = previous)")
	cmd.Flags().BoolP("yes", "y", false, "skip production confirmation prompt")
	cmd.Flags().BoolP("interactive", "i", false, "interactively select deployment to rollback to")
	return cmd
}

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

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Initialize remote server directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Init(env, mod, server)
		},
	}
}

func newUploadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upload [file]",
		Short: "Upload a file or package to target servers",
		RunE: withBranchCheck("upload", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			var filePath string
			if len(args) > 0 {
				filePath = args[0]
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Upload(env, mod, server, filePath)
		}),
	}
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install uploaded package on target servers",
		RunE: withBranchCheck("install", func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Install(env, mod, server)
		}),
	}
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List modules, environments, or deployments",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "modules",
		Short: "List all configured modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadContext(cmd)
			if err != nil {
				return err
			}
			for name, mod := range cfg.Modules {
				fmt.Printf("  %-30s  type=%-12s  port=%d\n", name, mod.Type, mod.Port)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "envs",
		Short: "List all configured environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadContext(cmd)
			if err != nil {
				return err
			}
			for name, env := range cfg.Environments {
				fmt.Printf("  %-12s  servers=%d\n", name, len(env.Servers))
			}
			return nil
		},
	})

	deploymentsCmd := &cobra.Command{
		Use:   "deployments",
		Short: "List deployment history for a module",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			deployer := deploy.New(cfg, sshMgr)

			if output == "json" {
				jsonStr, err := deployer.ListDeploymentsJSON(env, mod, server)
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
				return nil
			}

			return deployer.ListDeployments(env, mod, server)
		},
	}
	deploymentsCmd.Flags().StringP("output", "o", "", "output format (json)")
	cmd.AddCommand(deploymentsCmd)

	return cmd
}

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "SSH into a target server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadContext(cmd)
			if err != nil {
				return err
			}
			insecure, _ := cmd.Flags().GetBool("insecure")

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			sshMgr := ssh.NewManager(insecure)
			return sshMgr.InteractiveLogin(cfg, env, mod, server)
		},
	}
}

func newUnlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlock",
		Short: "Force release a deploy lock",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.ForceUnlock(env, mod, server)
		},
	}
}

func newCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove old deployment directories",
		Long: `Remove old deployment directories from remote servers, keeping the N most recent.
The current active deployment is never removed.

By default, keeps the 5 most recent deployments (configurable via --keep or retention.keep in tow.yaml).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			keep, _ := cmd.Flags().GetInt("keep")
			if keep == 0 {
				keep = cfg.Retention.Keep
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Cleanup(env, mod, server, keep)
		},
	}
	cmd.Flags().IntP("keep", "k", 0, "number of deployments to keep (default: from config or 5)")
	return cmd
}

func newDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <remote-path>",
		Short: "Download a file from a remote server",
		Long: `Download a file from a remote server to a local directory.
If the remote path is relative, it is resolved against the module base directory.

Examples:
  tow download -e prod -m api-server logs/std.log
  tow download -e prod -m api-server /var/log/syslog -d ./local-logs/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			localDir, _ := cmd.Flags().GetString("dir")
			deployer := deploy.New(cfg, sshMgr)
			return deployer.Download(env, mod, server, args[0], localDir)
		},
	}
	cmd.Flags().StringP("dir", "d", "", "local directory to download to (default: download/{env}-{server}/{timestamp}/)")
	return cmd
}

func newProvisionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a new server with basic requirements",
		Long: `Set up a new server with timezone, locale, JRE, essential tools,
and the deployment directory structure.

This is typically run once when adding a new server to your fleet.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			timezone, _ := cmd.Flags().GetString("timezone")
			locale, _ := cmd.Flags().GetString("locale")
			installJRE, _ := cmd.Flags().GetBool("jre")
			installTools, _ := cmd.Flags().GetBool("tools")

			opts := deploy.ProvisionOptions{
				Timezone:     timezone,
				Locale:       locale,
				InstallJRE:   installJRE,
				InstallTools: installTools,
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Provision(env, mod, server, opts)
		},
	}
	cmd.Flags().String("timezone", "", "server timezone (e.g., Asia/Seoul)")
	cmd.Flags().String("locale", "", "server locale (e.g., en_US.UTF-8)")
	cmd.Flags().Bool("jre", false, "install JRE (Java Runtime)")
	cmd.Flags().Bool("tools", false, "install essential tools (lsof, nc, curl)")
	return cmd
}

func newMCPServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp-server",
		Short: "Start MCP (Model Context Protocol) server for AI agents",
		Long: `Starts a JSON-RPC server over stdio that exposes Tow deployment
operations as MCP tools. This allows AI coding assistants (Claude, Cursor,
Windsurf, etc.) to manage deployments through natural language.

Configure in Claude Desktop / Claude Code:
  {
    "mcpServers": {
      "tow": {
        "command": "tow",
        "args": ["mcp-server"],
        "env": { "TOW_CONFIG": "/path/to/tow.yaml" }
      }
    }
  }

by neurosam.AI — https://neurosam.ai`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")

			server, err := mcpserver.NewServer(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to start MCP server: %w", err)
			}

			logger.Info("Tow MCP Server started (stdio mode)")
			logger.Info("Tools available: %d", len(server.Tools()))

			return server.Run()
		},
	}
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage servers, modules, and assignments",
	}

	// --- server subcommands ---
	serverCmd := &cobra.Command{Use: "server", Short: "Manage servers"}

	serverCmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add a server to tow.local.yaml",
		Long: `Add a new server to an environment.

Examples:
  tow config server add -e prod --name kafka-4 --host 10.0.2.100
  tow config server add -e prod --name kafka-4 --host 10.0.2.100 --number 4 --modules kafka,zookeeper`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envName, _ := cmd.Flags().GetString("environment")
			name, _ := cmd.Flags().GetString("name")
			host, _ := cmd.Flags().GetString("host")
			number, _ := cmd.Flags().GetInt("number")
			modulesStr, _ := cmd.Flags().GetString("modules")

			if envName == "" || name == "" || host == "" {
				return fmt.Errorf("--environment, --name, and --host are required")
			}

			var modules []string
			if modulesStr != "" {
				modules = strings.Split(modulesStr, ",")
				for i := range modules {
					modules[i] = strings.TrimSpace(modules[i])
				}
			}

			localPath := "tow.local.yaml"
			if err := config.AddServer(localPath, envName, name, host, number, modules); err != nil {
				return err
			}
			logger.Success("Server %s (%s) added to %s in %s", name, host, envName, localPath)
			return nil
		},
	})

	serverCmd.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "Remove a server from tow.local.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			envName, _ := cmd.Flags().GetString("environment")
			name, _ := cmd.Flags().GetString("name")
			if envName == "" || name == "" {
				return fmt.Errorf("--environment and --name are required")
			}
			if err := config.RemoveServer("tow.local.yaml", envName, name); err != nil {
				return err
			}
			logger.Success("Server %s removed from %s", name, envName)
			return nil
		},
	})

	serverCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List servers in an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			envName, _ := cmd.Flags().GetString("environment")
			if envName == "" {
				return fmt.Errorf("--environment is required")
			}
			servers, err := config.ListServers("tow.local.yaml", envName)
			if err != nil {
				return err
			}
			for _, srv := range servers {
				mods := ""
				if len(srv.Modules) > 0 {
					mods = strings.Join(srv.Modules, ", ")
				}
				fmt.Printf("  %-20s  host=%-18s  modules=[%s]\n", srv.Name, srv.Host, mods)
			}
			return nil
		},
	})

	// Add flags to server subcommands
	for _, sub := range serverCmd.Commands() {
		sub.Flags().String("name", "", "server name")
		sub.Flags().String("host", "", "server IP or hostname")
		sub.Flags().Int("number", 0, "server number (for legacy deploy_path)")
		sub.Flags().String("modules", "", "comma-separated module names to assign")
	}
	cmd.AddCommand(serverCmd)

	// --- module subcommands ---
	moduleCmd := &cobra.Command{Use: "module", Short: "Manage modules"}

	moduleCmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add a module definition to tow.yaml",
		Long: `Add a new module to the project config.

Examples:
  tow config module add my-api --type springboot --port 8080
  tow config module add my-cache --type redis --port 6379 --version 7.2.4`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modType, _ := cmd.Flags().GetString("type")
			port, _ := cmd.Flags().GetInt("port")
			version, _ := cmd.Flags().GetString("version")

			if modType == "" {
				return fmt.Errorf("--type is required")
			}

			cfgPath, _ := cmd.Root().Flags().GetString("config")
			if err := config.AddModule(cfgPath, args[0], modType, port, version); err != nil {
				return err
			}
			logger.Success("Module %s (type=%s) added to %s", args[0], modType, cfgPath)
			return nil
		},
	})

	moduleCmd.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "Remove a module from tow.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, _ := cmd.Root().Flags().GetString("config")
			if err := config.RemoveModule(cfgPath, args[0]); err != nil {
				return err
			}
			logger.Success("Module %s removed from %s", args[0], cfgPath)
			return nil
		},
	})

	moduleCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadContext(cmd)
			if err != nil {
				return err
			}
			for name, mod := range cfg.Modules {
				ver := mod.Version
				if ver == "" {
					ver = "-"
				}
				fmt.Printf("  %-30s  type=%-12s  port=%-6d  version=%s\n", name, mod.Type, mod.Port, ver)
			}
			return nil
		},
	})

	// Add flags to module subcommands
	for _, sub := range moduleCmd.Commands() {
		sub.Flags().String("type", "", "module type (springboot, node, kafka, redis, etc.)")
		sub.Flags().Int("port", 0, "service port")
		sub.Flags().String("version", "", "package version (for plugin types)")
	}
	cmd.AddCommand(moduleCmd)

	// --- assign/unassign ---
	cmd.AddCommand(&cobra.Command{
		Use:   "assign",
		Short: "Assign modules to a server",
		Long:  `tow config assign -e prod --server kafka-4 --modules kafka,zookeeper,node-exporter`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envName, _ := cmd.Flags().GetString("environment")
			server, _ := cmd.Flags().GetString("server")
			modulesStr, _ := cmd.Flags().GetString("modules")
			if envName == "" || server == "" || modulesStr == "" {
				return fmt.Errorf("--environment, --server, and --modules are required")
			}
			modules := strings.Split(modulesStr, ",")
			for i := range modules {
				modules[i] = strings.TrimSpace(modules[i])
			}
			if err := config.AssignModules("tow.local.yaml", envName, server, modules); err != nil {
				return err
			}
			logger.Success("Assigned [%s] to %s in %s", modulesStr, server, envName)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "unassign",
		Short: "Remove modules from a server",
		Long:  `tow config unassign -e prod --server kafka-4 --modules zookeeper`,
		RunE: func(cmd *cobra.Command, args []string) error {
			envName, _ := cmd.Flags().GetString("environment")
			server, _ := cmd.Flags().GetString("server")
			modulesStr, _ := cmd.Flags().GetString("modules")
			if envName == "" || server == "" || modulesStr == "" {
				return fmt.Errorf("--environment, --server, and --modules are required")
			}
			modules := strings.Split(modulesStr, ",")
			for i := range modules {
				modules[i] = strings.TrimSpace(modules[i])
			}
			if err := config.UnassignModules("tow.local.yaml", envName, server, modules); err != nil {
				return err
			}
			logger.Success("Unassigned [%s] from %s in %s", modulesStr, server, envName)
			return nil
		},
	})

	// Add shared flags to assign/unassign
	for _, sub := range cmd.Commands() {
		if sub.Name() == "assign" || sub.Name() == "unassign" {
			sub.Flags().String("server", "", "server name")
			sub.Flags().String("modules", "", "comma-separated module names")
		}
	}

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
			colorReset := "\033[0m"
			colors := []string{"\033[36m", "\033[33m", "\033[32m", "\033[35m", "\033[34m"}

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

func newThreadDumpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "threaddump",
		Short: "Trigger a thread dump on Java/Spring Boot modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, sshMgr, err := loadContext(cmd)
			if err != nil {
				return err
			}
			defer sshMgr.Close()

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.ThreadDump(env, mod, server)
		},
	}
}

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage infrastructure plugins",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load plugins from all directories
			for _, dir := range module.PluginDirs() {
				module.LoadPlugins(dir)
			}

			for _, name := range module.Available() {
				def := module.GetPluginDef(name)
				if def != nil {
					ver := def.Package.DefaultVersion
					if ver == "" {
						ver = "-"
					}
					fmt.Printf("  %-25s  %s  (v%s)\n", name, def.Description, ver)
				}
			}
			return nil
		},
	})

	installCmd := &cobra.Command{
		Use:   "install [plugins...]",
		Short: "Install plugins to ~/.tow/plugins/ from bundled collection",
		Long: `Install infrastructure plugins globally. Plugins are YAML files that define
how to deploy services like Kafka, Redis, MySQL, etc.

Examples:
  tow plugin install kafka redis mongodb
  tow plugin install --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")

			// Find bundled plugins (from tow-cli source or executable dir)
			srcDirs := []string{"plugins"}
			// Also check executable's directory
			if exe, err := os.Executable(); err == nil {
				srcDirs = append(srcDirs, filepath.Join(filepath.Dir(exe), "plugins"))
			}

			var srcDir string
			for _, d := range srcDirs {
				if info, err := os.Stat(d); err == nil && info.IsDir() {
					srcDir = d
					break
				}
			}

			if srcDir == "" {
				return fmt.Errorf("bundled plugins directory not found. Download plugins from https://github.com/neurosamAI/tow-cli/tree/main/plugins")
			}

			destDir := filepath.Join(os.Getenv("HOME"), ".tow", "plugins")
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("creating plugin directory: %w", err)
			}

			entries, err := os.ReadDir(srcDir)
			if err != nil {
				return err
			}

			installed := 0
			for _, entry := range entries {
				name := strings.TrimSuffix(entry.Name(), ".yaml")
				name = strings.TrimSuffix(name, ".yml")

				if !all && len(args) > 0 {
					found := false
					for _, a := range args {
						if a == name {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
					continue
				}

				src := filepath.Join(srcDir, entry.Name())
				dst := filepath.Join(destDir, entry.Name())

				data, err := os.ReadFile(src)
				if err != nil {
					logger.Warn("Failed to read %s: %v", src, err)
					continue
				}
				if err := os.WriteFile(dst, data, 0644); err != nil {
					logger.Warn("Failed to write %s: %v", dst, err)
					continue
				}
				fmt.Printf("  Installed: %s → %s\n", name, dst)
				installed++
			}

			if installed == 0 && !all {
				fmt.Println("No matching plugins found. Available:")
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".yaml") {
						fmt.Printf("  %s\n", strings.TrimSuffix(entry.Name(), ".yaml"))
					}
				}
			} else {
				fmt.Printf("\n%d plugin(s) installed to %s\n", installed, destDir)
			}
			return nil
		},
	}
	installCmd.Flags().Bool("all", false, "install all bundled plugins")
	cmd.AddCommand(installCmd)

	// tow plugin add — install from GitHub or URL
	addCmd := &cobra.Command{
		Use:   "add <source> [sources...]",
		Short: "Add plugins from GitHub repos or URLs",
		Long: `Download and install plugin YAML files from external sources.

Sources can be:
  GitHub shorthand:  user/repo                 → fetches plugin.yaml from repo root
  GitHub with file:  user/repo/path/to/file.yaml
  Full URL:          https://example.com/my-plugin.yaml

Examples:
  tow plugin add someuser/tow-plugin-mssql
  tow plugin add myorg/infra-plugins/rabbitmq.yaml
  tow plugin add https://raw.githubusercontent.com/user/repo/main/plugin.yaml`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			destDir := filepath.Join(home, ".tow", "plugins")
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("creating plugin directory: %w", err)
			}

			for _, source := range args {
				url, fileName := resolvePluginSource(source)
				logger.Info("Downloading %s...", url)

				data, err := fetchURL(url)
				if err != nil {
					logger.Error("Failed to download %s: %v", source, err)
					continue
				}

				// Validate it's valid YAML with a name field
				var def struct {
					Name string `yaml:"name"`
				}
				if err := yaml.Unmarshal(data, &def); err != nil {
					logger.Error("Invalid plugin YAML from %s: %v", source, err)
					continue
				}
				if def.Name == "" {
					logger.Error("Plugin from %s has no 'name' field", source)
					continue
				}

				destFile := filepath.Join(destDir, fileName)
				if err := os.WriteFile(destFile, data, 0644); err != nil {
					logger.Error("Failed to write %s: %v", destFile, err)
					continue
				}

				logger.Success("Installed: %s → %s", def.Name, destFile)
			}
			return nil
		},
	}
	cmd.AddCommand(addCmd)

	// tow plugin remove
	removeCmd := &cobra.Command{
		Use:   "remove <name> [names...]",
		Short: "Remove installed plugins",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			pluginDir := filepath.Join(home, ".tow", "plugins")

			for _, name := range args {
				file := filepath.Join(pluginDir, name+".yaml")
				if _, err := os.Stat(file); os.IsNotExist(err) {
					file = filepath.Join(pluginDir, name+".yml")
				}
				if err := os.Remove(file); err != nil {
					logger.Warn("Failed to remove %s: %v", name, err)
				} else {
					logger.Success("Removed: %s", name)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(removeCmd)

	return cmd
}

// resolvePluginSource converts a source string to a download URL and filename.
// Supports: "user/repo", "user/repo/path/file.yaml", or full URL.
func resolvePluginSource(source string) (url, fileName string) {
	// Full URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		parts := strings.Split(source, "/")
		fileName = parts[len(parts)-1]
		if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
			fileName += ".yaml"
		}
		return source, fileName
	}

	parts := strings.SplitN(source, "/", 3)

	if len(parts) == 2 {
		// user/repo → try repo root plugin.yaml, then {repo-name}.yaml
		user, repo := parts[0], parts[1]
		repoName := strings.TrimPrefix(repo, "tow-plugin-")
		repoName = strings.TrimPrefix(repoName, "tow-")
		fileName = repoName + ".yaml"
		url = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", user, repo, fileName)

		// If that doesn't work, try plugin.yaml
		return url, fileName
	}

	if len(parts) == 3 {
		// user/repo/path — specific file
		user, repo, path := parts[0], parts[1], parts[2]
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			path += ".yaml"
		}
		pathParts := strings.Split(path, "/")
		fileName = pathParts[len(pathParts)-1]
		url = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", user, repo, path)
		return url, fileName
	}

	// Fallback: treat as filename
	return source, source + ".yaml"
}

// fetchURL downloads content from a URL
func fetchURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
					fmt.Printf("  \033[31m✗\033[0m %s — %v\n", name, err)
					failed++
				} else {
					fmt.Printf("  \033[32m✓\033[0m %s\n", name)
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
