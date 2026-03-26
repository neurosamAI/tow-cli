package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/neurosamAI/tow-cli/internal/config"
)

func TestSubstituteVars(t *testing.T) {
	tests := []struct {
		cmd      string
		env      string
		module   string
		vars     map[string]string
		expected string
	}{
		{
			cmd:      "./gradlew :${MODULE}:bootJar -Pprofile=${ENV}",
			env:      "prod",
			module:   "api-server",
			vars:     nil,
			expected: "./gradlew :api-server:bootJar -Pprofile=prod",
		},
		{
			cmd:      "java ${JAVA_OPTS} -jar lib/${MODULE}.jar",
			env:      "dev",
			module:   "api",
			vars:     map[string]string{"JAVA_OPTS": "-Xms512m -Xmx1024m"},
			expected: "java -Xms512m -Xmx1024m -jar lib/api.jar",
		},
		{
			cmd:      "echo hello",
			env:      "dev",
			module:   "test",
			vars:     nil,
			expected: "echo hello",
		},
		{
			cmd:      "${CUSTOM_BUILD}",
			env:      "prod",
			module:   "app",
			vars:     map[string]string{"CUSTOM_BUILD": "make build"},
			expected: "make build",
		},
	}

	for _, tt := range tests {
		result := substituteVars(tt.cmd, tt.env, tt.module, tt.vars)
		if result != tt.expected {
			t.Errorf("substituteVars(%q) = %q, expected %q", tt.cmd, result, tt.expected)
		}
	}
}

func TestSubstituteVarsMultiple(t *testing.T) {
	vars := map[string]string{
		"JAVA_OPTS":      "-Xmx2g",
		"SPRING_PROFILE": "production",
	}
	cmd := "java ${JAVA_OPTS} -Dspring.profiles.active=${SPRING_PROFILE} -jar ${MODULE}.jar"
	result := substituteVars(cmd, "prod", "api", vars)

	if !strings.Contains(result, "-Xmx2g") {
		t.Error("expected JAVA_OPTS substitution")
	}
	if !strings.Contains(result, "production") {
		t.Error("expected SPRING_PROFILE substitution")
	}
	if !strings.Contains(result, "api.jar") {
		t.Error("expected MODULE substitution")
	}
}

func TestSetDryRun(t *testing.T) {
	SetDryRun(true)
	if !dryRun {
		t.Error("expected dryRun to be true")
	}
	SetDryRun(false)
	if dryRun {
		t.Error("expected dryRun to be false")
	}
}

// --- Additional substituteVars Tests ---

func TestSubstituteVarsEmpty(t *testing.T) {
	result := substituteVars("", "prod", "api", nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSubstituteVarsNoPlaceholders(t *testing.T) {
	result := substituteVars("echo hello world", "prod", "api", nil)
	if result != "echo hello world" {
		t.Errorf("expected unchanged, got %q", result)
	}
}

func TestSubstituteVarsOnlyENV(t *testing.T) {
	result := substituteVars("deploy to ${ENV}", "production", "api", nil)
	if result != "deploy to production" {
		t.Errorf("expected 'deploy to production', got %q", result)
	}
}

func TestSubstituteVarsOnlyMODULE(t *testing.T) {
	result := substituteVars("build ${MODULE}", "prod", "my-api", nil)
	if result != "build my-api" {
		t.Errorf("expected 'build my-api', got %q", result)
	}
}

func TestSubstituteVarsMultipleOccurrences(t *testing.T) {
	result := substituteVars("${MODULE} ${MODULE} ${ENV} ${ENV}", "prod", "api", nil)
	if result != "api api prod prod" {
		t.Errorf("expected 'api api prod prod', got %q", result)
	}
}

func TestSubstituteVarsEmptyVarsMap(t *testing.T) {
	result := substituteVars("${ENV} ${MODULE}", "dev", "svc", map[string]string{})
	if result != "dev svc" {
		t.Errorf("expected 'dev svc', got %q", result)
	}
}

func TestSubstituteVarsUnknownVar(t *testing.T) {
	// Variables not in the map should remain unchanged
	result := substituteVars("${UNKNOWN_VAR}", "prod", "api", map[string]string{"OTHER": "val"})
	if result != "${UNKNOWN_VAR}" {
		t.Errorf("expected '${UNKNOWN_VAR}' unchanged, got %q", result)
	}
}

func TestSubstituteVarsComplexCommand(t *testing.T) {
	vars := map[string]string{
		"JAVA_OPTS":      "-Xmx4g -XX:+UseG1GC",
		"SPRING_PROFILE": "production",
		"DB_HOST":        "db.internal",
	}
	cmd := "java ${JAVA_OPTS} -Dspring.profiles.active=${SPRING_PROFILE} -Ddb.host=${DB_HOST} -jar ${MODULE}.jar"
	result := substituteVars(cmd, "prod", "api", vars)

	expected := "java -Xmx4g -XX:+UseG1GC -Dspring.profiles.active=production -Ddb.host=db.internal -jar api.jar"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// --- runLocalCmd with DryRun ---

func TestRunLocalCmdDryRun(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	err := runLocalCmd("echo hello")
	if err != nil {
		t.Errorf("expected no error in dry-run mode, got %v", err)
	}
}

func TestRunLocalCmdDryRunComplexCommand(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	err := runLocalCmd("./gradlew :api:bootJar -Pprofile=prod && tar czf build/api.tar.gz -C api/build/libs .")
	if err != nil {
		t.Errorf("expected no error in dry-run mode, got %v", err)
	}
}

// --- Pipeline runSteps ---

func TestRunStepsAllSuccess(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", BaseDir: "/app"},
	}
	p := New(cfg, nil)

	callOrder := []string{}
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Step1", func() error { callOrder = append(callOrder, "1"); return nil }},
		{"Step2", func() error { callOrder = append(callOrder, "2"); return nil }},
		{"Step3", func() error { callOrder = append(callOrder, "3"); return nil }},
	}

	err := p.runSteps(steps)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(callOrder) != 3 {
		t.Errorf("expected 3 steps executed, got %d", len(callOrder))
	}
	if callOrder[0] != "1" || callOrder[1] != "2" || callOrder[2] != "3" {
		t.Errorf("expected order 1,2,3, got %v", callOrder)
	}
}

func TestRunStepsFailsAtMiddle(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", BaseDir: "/app"},
	}
	p := New(cfg, nil)

	callOrder := []string{}
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Build", func() error { callOrder = append(callOrder, "build"); return nil }},
		{"Package", func() error { return fmt.Errorf("package failed") }},
		{"Upload", func() error { callOrder = append(callOrder, "upload"); return nil }},
	}

	err := p.runSteps(steps)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Package") {
		t.Errorf("expected error about Package step, got %v", err)
	}
	// Upload should not have been called
	if len(callOrder) != 1 {
		t.Errorf("expected only build to run, got %v", callOrder)
	}
}

func TestRunStepsEmptyList(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", BaseDir: "/app"},
	}
	p := New(cfg, nil)

	steps := []struct {
		name string
		fn   func() error
	}{}

	err := p.runSteps(steps)
	if err != nil {
		t.Fatalf("expected no error for empty steps, got %v", err)
	}
}

// --- Pipeline Build ---

func TestBuildModuleNotFound(t *testing.T) {
	cfg := &config.Config{
		Modules: map[string]*config.Module{},
	}
	p := New(cfg, nil)

	err := p.Build("nonexistent", "dev")
	if err == nil {
		t.Fatal("expected error for nonexistent module")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildNoBuildCmd(t *testing.T) {
	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {Type: "generic", BuildCmd: ""},
		},
	}
	p := New(cfg, nil)

	err := p.Build("api", "dev")
	if err != nil {
		t.Fatalf("expected no error when no build cmd, got %v", err)
	}
}

func TestBuildWithDryRun(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {Type: "generic", BuildCmd: "echo building"},
		},
	}
	p := New(cfg, nil)

	err := p.Build("api", "dev")
	if err != nil {
		t.Fatalf("expected no error in dry-run, got %v", err)
	}
}

func TestBuildWithVariables(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {
				Type:      "springboot",
				BuildCmd:  "./gradlew :${MODULE}:bootJar -Pprofile=${ENV}",
				Variables: map[string]string{"EXTRA": "value"},
			},
		},
	}
	p := New(cfg, nil)

	err := p.Build("api", "prod")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestBuildWithHooks(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {
				Type:     "generic",
				BuildCmd: "echo build",
				Hooks: config.HooksConfig{
					PreBuild:  "echo pre",
					PostBuild: "echo post",
				},
			},
		},
	}
	p := New(cfg, nil)

	err := p.Build("api", "dev")
	if err != nil {
		t.Fatalf("expected no error with hooks in dry-run, got %v", err)
	}
}

// --- Pipeline Package ---

func TestPackageModuleNotFound(t *testing.T) {
	cfg := &config.Config{
		Modules: map[string]*config.Module{},
	}
	p := New(cfg, nil)

	err := p.Package("nonexistent", "dev")
	if err == nil {
		t.Fatal("expected error for nonexistent module")
	}
}

func TestPackageWithDryRun(t *testing.T) {
	SetDryRun(true)
	defer SetDryRun(false)

	cfg := &config.Config{
		Modules: map[string]*config.Module{
			"api": {
				Type:         "generic",
				ArtifactPath: "build/api.tar.gz",
			},
		},
	}
	p := New(cfg, nil)

	err := p.Package("api", "dev")
	if err != nil {
		t.Fatalf("expected no error in dry-run, got %v", err)
	}
}

// --- New Pipeline ---

func TestNewPipeline(t *testing.T) {
	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test"},
	}
	p := New(cfg, nil)
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if p.cfg != cfg {
		t.Error("expected cfg to be set")
	}
	if p.Rolling {
		t.Error("expected Rolling to be false by default")
	}
}
