package main

import (
	"fmt"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"

	"github.com/spf13/cobra"
)

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
