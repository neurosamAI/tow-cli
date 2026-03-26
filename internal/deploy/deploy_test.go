package deploy

import (
	"strings"
	"testing"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Project: config.ProjectConfig{
			Name:    "test-project",
			BaseDir: "/app",
		},
		Environments: map[string]*config.Environment{
			"dev": {
				SSHUser:    "testuser",
				SSHPort:    22,
				SSHKeyPath: "~/.ssh/test.pem",
				Servers: []config.Server{
					{Number: 1, Host: "10.0.1.10"},
					{Number: 2, Host: "10.0.1.11"},
				},
			},
			"prod": {
				SSHUser:    "ec2-user",
				SSHPort:    22,
				SSHKeyPath: "~/.ssh/prod.pem",
				Branch:     "main",
				Servers: []config.Server{
					{Number: 1, Host: "52.78.100.1", Modules: []string{"api-server"}},
					{Number: 2, Host: "52.78.100.2", Modules: []string{"api-server"}},
					{Number: 3, Host: "52.78.100.3", Modules: []string{"kafka"}},
				},
			},
		},
		Modules: map[string]*config.Module{
			"api-server": {
				Type:     "springboot",
				Port:     8080,
				BuildCmd: "./gradlew :api-server:bootJar",
				HealthCheck: config.HealthCheckConfig{
					Type:     "http",
					Target:   "http://localhost:8080/actuator/health",
					Timeout:  10,
					Interval: 2,
					Retries:  5,
				},
			},
			"kafka": {
				Type: "kafka",
				Port: 9092,
			},
		},
		Retention: config.RetentionConfig{
			Keep:        5,
			AutoCleanup: true,
		},
	}
}

func TestDeployTimestamp(t *testing.T) {
	ts := deployTimestamp()
	if len(ts) != 15 { // 20060102-150405
		t.Errorf("expected timestamp length 15, got %d: %s", len(ts), ts)
	}
	if !strings.Contains(ts, "-") {
		t.Errorf("expected timestamp to contain '-': %s", ts)
	}
}

func TestRemoteBaseDir(t *testing.T) {
	cfg := testConfig()
	d := New(cfg, nil)

	dir := d.remoteBaseDir("api-server")
	if dir != "/app/api-server" {
		t.Errorf("expected /app/api-server, got %s", dir)
	}
}

func TestRemoteBaseDirDefault(t *testing.T) {
	cfg := testConfig()
	cfg.Project.BaseDir = ""
	d := New(cfg, nil)

	dir := d.remoteBaseDir("api-server")
	if dir != "/app/api-server" {
		t.Errorf("expected /app/api-server (default), got %s", dir)
	}
}

func TestWriteAuditLog(t *testing.T) {
	cfg := testConfig()
	d := New(cfg, nil)

	// Should not panic even without proper setup
	d.WriteAuditLog("dev", "api-server", "deploy", "test")
}

func TestNotifyNoConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Notifications = nil
	d := New(cfg, nil)

	// Should not panic when no notifications configured
	d.Notify("dev", "api-server", "deploy_start", "test")
}

func TestNotifyWithConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Notifications = []config.NotificationConfig{
		{Type: "webhook", URL: "https://example.com/hook"},
		{Type: "slack", URL: "https://hooks.slack.com/test"},
		{Type: "discord", URL: "https://discord.com/api/webhooks/test"},
		{Type: "telegram", URL: "123456:ABC", ChatID: "-100123"},
		{Type: "unknown", URL: "https://example.com"},
	}
	d := New(cfg, nil)

	// Should not panic — notifications are async/fire-and-forget
	d.Notify("dev", "api-server", "deploy_start", "test message")
	time.Sleep(100 * time.Millisecond) // Let goroutines run
}

func TestProvisionOptions(t *testing.T) {
	opts := ProvisionOptions{
		Timezone:     "Asia/Seoul",
		Locale:       "en_US.UTF-8",
		InstallJRE:   true,
		InstallTools: true,
	}

	if opts.Timezone != "Asia/Seoul" {
		t.Errorf("expected Asia/Seoul, got %s", opts.Timezone)
	}
	if !opts.InstallJRE {
		t.Error("expected InstallJRE to be true")
	}
}
