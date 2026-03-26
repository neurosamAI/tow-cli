package deploy

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
)

// Default commands that require branch verification
var defaultBranchCommands = []string{
	"deploy", "auto", "upload", "install", "start", "stop", "restart", "rollback",
}

// CheckBranch verifies the current git branch against the environment's branch policy
func CheckBranch(cfg *config.Config, envName, command string) error {
	env, ok := cfg.Environments[envName]
	if !ok {
		return nil
	}

	// Advanced policy takes precedence
	if env.BranchPolicy != nil {
		return checkBranchPolicy(env.BranchPolicy, envName, command)
	}

	// Simple mode: just check env.Branch
	if env.Branch == "" || env.Branch == "*" {
		return nil
	}

	currentBranch, err := getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to detect git branch: %w", err)
	}

	if currentBranch != env.Branch {
		return fmt.Errorf("environment %q requires branch %q, but current branch is %q",
			envName, env.Branch, currentBranch)
	}

	logger.Debug("Branch check passed: %s", currentBranch)
	return nil
}

func checkBranchPolicy(policy *config.BranchPolicy, envName, command string) error {
	if policy.Skip {
		return nil
	}

	if len(policy.Allowed) == 0 {
		return nil
	}

	// Check if the current command requires branch verification
	if len(policy.Commands) > 0 {
		found := false
		for _, cmd := range policy.Commands {
			if cmd == command {
				found = true
				break
			}
		}
		if !found {
			return nil // This command doesn't require branch check
		}
	} else {
		// Default: check for all mutating commands
		found := false
		for _, cmd := range defaultBranchCommands {
			if cmd == command {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	currentBranch, err := getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to detect git branch: %w", err)
	}

	for _, pattern := range policy.Allowed {
		if pattern == "*" {
			return nil
		}
		matched, err := filepath.Match(pattern, currentBranch)
		if err != nil {
			logger.Warn("Invalid branch pattern %q: %v", pattern, err)
			continue
		}
		if matched {
			logger.Debug("Branch check passed: %s (matched pattern: %s)", currentBranch, pattern)
			return nil
		}
	}

	return fmt.Errorf("environment %q does not allow branch %q (allowed: %s)",
		envName, currentBranch, strings.Join(policy.Allowed, ", "))
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
