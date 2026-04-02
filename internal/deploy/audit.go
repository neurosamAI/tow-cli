package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WriteAuditLog writes a deployment event to a local audit log file
func (d *Deployer) WriteAuditLog(envName, moduleName, action, detail string) {
	auditDir := ".tow"
	auditFile := filepath.Join(auditDir, "audit.log")

	os.MkdirAll(auditDir, 0755)

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "unknown"
	}

	entry := fmt.Sprintf("%s | user=%s | env=%s | module=%s | action=%s | %s\n",
		time.Now().Format("2006-01-02T15:04:05Z"), currentUser, envName, moduleName, action, detail)

	f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
}

// getGitInfo returns current commit hash, branch, and last commit message
func getGitInfo() (commit, branch, message string) {
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "log", "-1", "--format=%s").Output(); err == nil {
		message = strings.TrimSpace(string(out))
	}
	return
}
