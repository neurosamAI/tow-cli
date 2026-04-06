package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/pipeline"
	"github.com/neurosamAI/tow-cli/internal/ssh"
	_ "github.com/neurosamAI/tow-cli/plugins" // auto-register bundled plugins

	"github.com/spf13/cobra"
)

var (
	Version   = "0.4.0"
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
// When interactive is true, missing -m or -s flags trigger interactive pickers
// instead of returning an error.
func resolveTargets(cmd *cobra.Command, cfg *config.Config) (string, string, int, error) {
	return resolveTargetsWithMode(cmd, cfg, false)
}

// resolveTargetsInteractive is like resolveTargets but shows pickers when flags are omitted.
func resolveTargetsInteractive(cmd *cobra.Command, cfg *config.Config) (string, string, int, error) {
	return resolveTargetsWithMode(cmd, cfg, true)
}

func resolveTargetsWithMode(cmd *cobra.Command, cfg *config.Config, interactive bool) (string, string, int, error) {
	envName, _ := cmd.Flags().GetString("environment")
	modName, _ := cmd.Flags().GetString("module")
	serverFlag, _ := cmd.Flags().GetString("server")

	if envName == "" {
		return "", "", 0, fmt.Errorf("environment (-e) is required")
	}

	envCfg, ok := cfg.Environments[envName]
	if !ok {
		return "", "", 0, fmt.Errorf("environment %q not found in config", envName)
	}

	// Non-interactive: require both flags
	if !interactive {
		if modName == "" {
			return "", "", 0, fmt.Errorf("module (-m) is required")
		}

		serverNum := 0
		if serverFlag != "" {
			if n, err := fmt.Sscanf(serverFlag, "%d", &serverNum); n == 0 || err != nil {
				serverNum = -1 // signal to use name
			}
		}
		return envName, modName, serverNum, nil
	}

	// Interactive: pick missing values

	// Case 1: server given but not module -> pick module from server's list
	if modName == "" && serverFlag != "" {
		var targetSrv *config.Server
		for i := range envCfg.Servers {
			if envCfg.Servers[i].ID() == serverFlag {
				targetSrv = &envCfg.Servers[i]
				break
			}
		}
		if targetSrv == nil {
			return "", "", 0, fmt.Errorf("server %q not found in environment %q", serverFlag, envName)
		}
		if len(targetSrv.Modules) == 0 {
			return "", "", 0, fmt.Errorf("server %q has no modules assigned", serverFlag)
		}
		picked, err := pickModule(targetSrv.Modules)
		if err != nil {
			return "", "", 0, err
		}
		return envName, picked, targetSrv.Number, nil
	}

	// Case 2: module given but not server -> pick server if multiple
	if modName != "" && serverFlag == "" {
		servers, _, err := cfg.GetServersForModule(envName, modName, 0)
		if err != nil {
			return "", "", 0, err
		}
		serverNum := 0
		if len(servers) > 1 {
			srv, err := pickServer(servers)
			if err != nil {
				return "", "", 0, err
			}
			serverNum = srv.Number
		}
		return envName, modName, serverNum, nil
	}

	// Case 3: neither module nor server -> pick module first, then server
	if modName == "" && serverFlag == "" {
		var moduleNames []string
		for name := range cfg.Modules {
			moduleNames = append(moduleNames, name)
		}
		if len(moduleNames) == 0 {
			return "", "", 0, fmt.Errorf("no modules configured")
		}
		picked, err := pickModule(moduleNames)
		if err != nil {
			return "", "", 0, err
		}
		modName = picked

		servers, _, err := cfg.GetServersForModule(envName, modName, 0)
		if err != nil {
			return "", "", 0, err
		}
		serverNum := 0
		if len(servers) > 1 {
			srv, err := pickServer(servers)
			if err != nil {
				return "", "", 0, err
			}
			serverNum = srv.Number
		}
		return envName, modName, serverNum, nil
	}

	// Case 4: both given -> parse server flag
	serverNum := 0
	if serverFlag != "" {
		if n, e := fmt.Sscanf(serverFlag, "%d", &serverNum); n == 0 || e != nil {
			serverNum = -1
		}
	}
	return envName, modName, serverNum, nil
}

// pickServer prompts the user to select a server when multiple match and no -s flag is given.
// Returns the selected server's name. If only one server, returns it directly.
func pickServer(servers []config.Server) (config.Server, error) {
	if len(servers) == 1 {
		return servers[0], nil
	}

	fmt.Fprintf(os.Stderr, "\nMultiple servers found:\n")
	for i, srv := range servers {
		fmt.Fprintf(os.Stderr, "  [%d] %-20s (%s)\n", i+1, srv.ID(), srv.Host)
	}
	fmt.Fprintf(os.Stderr, "\nSelect server [1-%d]: ", len(servers))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(servers) {
		return config.Server{}, fmt.Errorf("invalid selection: %s", input)
	}

	return servers[idx-1], nil
}

// pickModule prompts the user to select a module from a list
func pickModule(modules []string) (string, error) {
	if len(modules) == 1 {
		return modules[0], nil
	}

	fmt.Fprintf(os.Stderr, "\nMultiple modules available:\n")
	for i, mod := range modules {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, mod)
	}
	fmt.Fprintf(os.Stderr, "\nSelect module [1-%d]: ", len(modules))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(modules) {
		return "", fmt.Errorf("invalid selection: %s", input)
	}

	return modules[idx-1], nil
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
		logger.ColorYellow, command, moduleName, strings.ToUpper(envName), logger.ColorReset)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
