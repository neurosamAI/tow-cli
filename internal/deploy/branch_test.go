package deploy

import (
	"testing"

	"github.com/neurosamAI/tow-cli/internal/config"
)

func TestCheckBranchNoRestriction(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["dev"].Branch = ""

	err := CheckBranch(cfg, "dev", "deploy")
	if err != nil {
		t.Errorf("expected no error for unrestricted branch, got %v", err)
	}
}

func TestCheckBranchWithPolicy(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["prod"].BranchPolicy = &config.BranchPolicy{
		Allowed:  []string{"main", "release/*"},
		Commands: []string{"deploy", "auto"},
		Skip:     false,
	}

	// Commands not in the policy list should pass
	err := CheckBranch(cfg, "prod", "status")
	if err != nil {
		t.Errorf("expected no error for non-restricted command, got %v", err)
	}
}

func TestCheckBranchPolicySkip(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["prod"].BranchPolicy = &config.BranchPolicy{
		Skip: true,
	}

	err := CheckBranch(cfg, "prod", "deploy")
	if err != nil {
		t.Errorf("expected no error with skip=true, got %v", err)
	}
}

func TestCheckBranchNonExistentEnv(t *testing.T) {
	cfg := testConfig()

	err := CheckBranch(cfg, "staging", "deploy")
	if err != nil {
		t.Errorf("expected no error for non-existent env, got %v", err)
	}
}

func TestCheckBranchWildcard(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["dev"].Branch = "*"

	err := CheckBranch(cfg, "dev", "deploy")
	if err != nil {
		t.Errorf("expected no error for wildcard branch, got %v", err)
	}
}

func TestCheckBranchPolicyEmptyAllowed(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["prod"].BranchPolicy = &config.BranchPolicy{
		Allowed: []string{},
	}

	err := CheckBranch(cfg, "prod", "deploy")
	if err != nil {
		t.Errorf("expected no error for empty allowed list, got %v", err)
	}
}

func TestCheckBranchPolicyNonMutatingCommand(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["prod"].BranchPolicy = &config.BranchPolicy{
		Allowed: []string{"main"},
		// No commands specified — defaults to mutating commands
	}

	// "status" is not in the default mutating commands list
	err := CheckBranch(cfg, "prod", "status")
	if err != nil {
		t.Errorf("expected no error for non-mutating command, got %v", err)
	}

	// "logs" is not in the default mutating commands list
	err = CheckBranch(cfg, "prod", "logs")
	if err != nil {
		t.Errorf("expected no error for logs command, got %v", err)
	}
}

func TestCheckBranchPolicyCustomCommandsNotMatching(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["prod"].BranchPolicy = &config.BranchPolicy{
		Allowed:  []string{"main"},
		Commands: []string{"deploy", "auto"},
	}

	// "upload" is not in the custom commands list, so it should pass
	err := CheckBranch(cfg, "prod", "upload")
	if err != nil {
		t.Errorf("expected no error for non-listed command, got %v", err)
	}

	// "status" is not in the custom commands list
	err = CheckBranch(cfg, "prod", "status")
	if err != nil {
		t.Errorf("expected no error for status command, got %v", err)
	}
}

func TestCheckBranchEmptyBranch(t *testing.T) {
	cfg := testConfig()
	cfg.Environments["prod"].Branch = ""
	cfg.Environments["prod"].BranchPolicy = nil

	err := CheckBranch(cfg, "prod", "deploy")
	if err != nil {
		t.Errorf("expected no error for empty branch, got %v", err)
	}
}

func TestJoinLines(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected string
	}{
		{"single", []string{"line1"}, "line1"},
		{"multiple", []string{"line1", "line2", "line3"}, "line1\n  line2\n  line3"},
		{"empty", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinLines(tt.lines)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
