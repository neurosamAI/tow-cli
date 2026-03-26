package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	mcpserver "github.com/neurosamAI/tow-cli/integrations/mcp-server"
	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/initializer"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/pipeline"
	"github.com/neurosamAI/tow-cli/internal/ssh"

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
func confirmProdDeploy(envName, moduleName, command string) bool {
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

			_, mod, _, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			env, _ := cmd.Flags().GetString("environment")
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

			_, mod, _, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			env, _ := cmd.Flags().GetString("environment")
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
			if !yes && !confirmProdDeploy(env, mod, "deploy") {
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
			if !yes && !confirmProdDeploy(env, mod, "auto deploy") {
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

			env, mod, server, err := resolveTargets(cmd, cfg)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			deployer := deploy.New(cfg, sshMgr)

			if output == "json" {
				jsonStr, err := deployer.StatusJSON(env, mod, server)
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
				return nil
			}

			return deployer.Status(env, mod, server)
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
			if !yes && !confirmProdDeploy(env, mod, "rollback") {
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
		Short: "Stream logs from a module",
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

			filter, _ := cmd.Flags().GetString("filter")
			lines, _ := cmd.Flags().GetInt("lines")

			deployer := deploy.New(cfg, sshMgr)
			return deployer.Logs(env, mod, server, filter, lines)
		},
	}
	cmd.Flags().StringP("filter", "f", "", "grep filter for log output")
	cmd.Flags().IntP("lines", "n", 100, "number of tail lines")
	return cmd
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
