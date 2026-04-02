package main

import (
	"fmt"
	"os"

	mcpserver "github.com/neurosamAI/tow-cli/integrations/mcp-server"
	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/initializer"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/ssh"

	"github.com/spf13/cobra"
)

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

			envName, modName, serverNum, err := resolveTargetsInteractive(cmd, cfg)
			if err != nil {
				return err
			}

			sshMgr := ssh.NewManager(insecure)
			return sshMgr.InteractiveLogin(cfg, envName, modName, serverNum)
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

			env, mod, server, err := resolveTargetsInteractive(cmd, cfg)
			if err != nil {
				return err
			}

			deployer := deploy.New(cfg, sshMgr)
			return deployer.ThreadDump(env, mod, server)
		},
	}
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
