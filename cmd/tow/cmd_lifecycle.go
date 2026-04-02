package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/logger"

	"github.com/spf13/cobra"
)

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
