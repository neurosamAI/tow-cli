package deploy

import (
	"testing"
)

func TestLockInfo(t *testing.T) {
	info := LockInfo{
		User:      "testuser",
		Host:      "10.0.1.10",
		Timestamp: "2026-03-26T14:30:00Z",
		Command:   "deploy",
	}

	if info.User != "testuser" {
		t.Errorf("expected testuser, got %s", info.User)
	}
	if info.Command != "deploy" {
		t.Errorf("expected deploy, got %s", info.Command)
	}
}
