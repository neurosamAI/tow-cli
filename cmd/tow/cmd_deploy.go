package main

import (
	"fmt"

	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/pipeline"

	"github.com/spf13/cobra"
)

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
