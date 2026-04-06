package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNewCommands verifies that all command constructors return valid commands
// without panicking and with expected properties.
func TestNewCommands(t *testing.T) {
	commands := []struct {
		name string
		fn   func() *cobra.Command
	}{
		{"init", newInitCmd},
		{"validate", newValidateCmd},
		{"build", newBuildCmd},
		{"package", newPackageCmd},
		{"deploy", newDeployCmd},
		{"auto", newAutoCmd},
		{"start", newStartCmd},
		{"stop", newStopCmd},
		{"restart", newRestartCmd},
		{"status", newStatusCmd},
		{"rollback", newRollbackCmd},
		{"logs", newLogsCmd},
		{"setup", newSetupCmd},
		{"upload", newUploadCmd},
		{"install", newInstallCmd},
		{"list", newListCmd},
		{"login", newLoginCmd},
		{"unlock", newUnlockCmd},
		{"cleanup", newCleanupCmd},
		{"download", newDownloadCmd},
		{"provision", newProvisionCmd},
		{"threaddump", newThreadDumpCmd},
		{"plugin", newPluginCmd},
		{"ssh", newSSHCmd},
		{"diff", newDiffCmd},
		{"config", newConfigCmd},
		{"metrics", newMetricsCmd},
		{"doctor", newDoctorCmd},
		{"mcp-server", newMCPServerCmd},
	}

	for _, tt := range commands {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.fn()
			if cmd == nil {
				t.Fatalf("expected non-nil command for %s", tt.name)
			}
			if cmd.Use == "" {
				t.Errorf("expected non-empty Use for %s", tt.name)
			}
			if cmd.Short == "" {
				t.Errorf("expected non-empty Short for %s", tt.name)
			}
		})
	}
}
