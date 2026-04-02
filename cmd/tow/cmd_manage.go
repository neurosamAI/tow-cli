package main

import (
	"fmt"

	"github.com/neurosamAI/tow-cli/internal/deploy"

	"github.com/spf13/cobra"
)

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

			env, mod, server, err := resolveTargetsInteractive(cmd, cfg)
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
