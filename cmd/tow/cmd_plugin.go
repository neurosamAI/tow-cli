package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/module"

	"github.com/spf13/cobra"
)

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
