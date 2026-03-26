package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// mergeLocal loads a tow.local.yaml file (if it exists) and merges it on top
// of the main config. For a main file named "tow.yaml", the local override
// file is "tow.local.yaml". For "custom.yaml", it would be "custom.local.yaml".
func mergeLocal(cfg *Config, mainPath string) error {
	dir := filepath.Dir(mainPath)
	base := filepath.Base(mainPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	localPath := filepath.Join(dir, name+".local"+ext)

	data, err := os.ReadFile(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no local file, nothing to merge
		}
		return fmt.Errorf("reading local config %s: %w", localPath, err)
	}

	// Expand environment variables in the local file as well
	expanded := os.ExpandEnv(string(data))

	local := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), local); err != nil {
		return fmt.Errorf("parsing local config %s: %w", localPath, err)
	}

	// Merge project fields (override non-empty)
	if local.Project.Name != "" {
		cfg.Project.Name = local.Project.Name
	}
	if local.Project.Version != "" {
		cfg.Project.Version = local.Project.Version
	}
	if local.Project.BaseDir != "" {
		cfg.Project.BaseDir = local.Project.BaseDir
	}

	// Merge defaults (override non-empty)
	if local.Defaults.SSHUser != "" {
		cfg.Defaults.SSHUser = local.Defaults.SSHUser
	}
	if local.Defaults.SSHPort != 0 {
		cfg.Defaults.SSHPort = local.Defaults.SSHPort
	}
	if local.Defaults.SSHKeyPath != "" {
		cfg.Defaults.SSHKeyPath = local.Defaults.SSHKeyPath
	}
	if local.Defaults.DeployDir != "" {
		cfg.Defaults.DeployDir = local.Defaults.DeployDir
	}
	if local.Defaults.HealthCheck.Type != "" {
		cfg.Defaults.HealthCheck = local.Defaults.HealthCheck
	}

	// Merge environments
	for envName, localEnv := range local.Environments {
		existing, ok := cfg.Environments[envName]
		if !ok {
			// New environment from local config
			if cfg.Environments == nil {
				cfg.Environments = make(map[string]*Environment)
			}
			cfg.Environments[envName] = localEnv
			continue
		}
		// Merge into existing environment
		if localEnv.SSHUser != "" {
			existing.SSHUser = localEnv.SSHUser
		}
		if localEnv.SSHPort != 0 {
			existing.SSHPort = localEnv.SSHPort
		}
		if localEnv.SSHKeyPath != "" {
			existing.SSHKeyPath = localEnv.SSHKeyPath
		}
		if localEnv.Branch != "" {
			existing.Branch = localEnv.Branch
		}
		if localEnv.BranchPolicy != nil {
			existing.BranchPolicy = localEnv.BranchPolicy
		}
		if len(localEnv.Variables) > 0 {
			if existing.Variables == nil {
				existing.Variables = make(map[string]string)
			}
			for k, v := range localEnv.Variables {
				existing.Variables[k] = v
			}
		}
		if len(localEnv.Servers) > 0 {
			existing.Servers = localEnv.Servers
		}
	}

	// Merge modules
	for modName, localMod := range local.Modules {
		existing, ok := cfg.Modules[modName]
		if !ok {
			// New module from local config
			if cfg.Modules == nil {
				cfg.Modules = make(map[string]*Module)
			}
			cfg.Modules[modName] = localMod
			continue
		}
		// Merge into existing module (override non-empty fields)
		if localMod.Type != "" {
			existing.Type = localMod.Type
		}
		if localMod.Port != 0 {
			existing.Port = localMod.Port
		}
		if localMod.BuildCmd != "" {
			existing.BuildCmd = localMod.BuildCmd
		}
		if localMod.ArtifactPath != "" {
			existing.ArtifactPath = localMod.ArtifactPath
		}
		if len(localMod.PackageIncludes) > 0 {
			existing.PackageIncludes = localMod.PackageIncludes
		}
		if localMod.HealthCheck.Type != "" {
			existing.HealthCheck = localMod.HealthCheck
		}
		if localMod.StartCmd != "" {
			existing.StartCmd = localMod.StartCmd
		}
		if localMod.StopCmd != "" {
			existing.StopCmd = localMod.StopCmd
		}
		if localMod.StatusCmd != "" {
			existing.StatusCmd = localMod.StatusCmd
		}
		if localMod.LogPath != "" {
			existing.LogPath = localMod.LogPath
		}
		if localMod.DeployDir != "" {
			existing.DeployDir = localMod.DeployDir
		}
		if localMod.ConfigDir != "" {
			existing.ConfigDir = localMod.ConfigDir
		}
		if len(localMod.DataDirs) > 0 {
			existing.DataDirs = localMod.DataDirs
		}
		if len(localMod.Variables) > 0 {
			if existing.Variables == nil {
				existing.Variables = make(map[string]string)
			}
			for k, v := range localMod.Variables {
				existing.Variables[k] = v
			}
		}
		if localMod.SSH != nil {
			existing.SSH = localMod.SSH
		}
	}

	return nil
}
