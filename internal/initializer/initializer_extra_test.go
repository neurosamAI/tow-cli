package initializer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectPHPWordPress(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require": {"johnpbloch/wordpress-core": "^6.0"}}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "php" {
		t.Errorf("expected php, got %q", det.Primary().Name)
	}
}

func TestDetectRubyPuma(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("gem 'puma'\ngem 'rack'"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "ruby" {
		t.Errorf("expected ruby, got %q", det.Primary().Name)
	}
}

func TestDetectDotNetMinimalAPI(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "MyApi.csproj"), []byte(`
<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>
`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "dotnet" {
		t.Errorf("expected dotnet, got %q", det.Primary().Name)
	}
}

func TestDetectDockerfileOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node:18"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasDocker {
		t.Error("expected Docker detection")
	}
}

func TestDetectCIGitHub(t *testing.T) {
	dir := t.TempDir()
	ciDir := filepath.Join(dir, ".github", "workflows")
	os.MkdirAll(ciDir, 0755)
	os.WriteFile(filepath.Join(ciDir, "ci.yml"), []byte("name: CI"), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !det.HasCI {
		t.Error("expected CI detection")
	}
}

func TestPrimaryEmptyTypes(t *testing.T) {
	det := &DetectedProject{}
	primary := det.Primary()
	if primary.Name != "generic" {
		t.Errorf("expected generic for empty types, got %q", primary.Name)
	}
}

func TestGenerateConfigMultiModuleWithExcluded(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "platform",
		RootDir:           "/tmp/test",
		MultiModule:       true,
		ModuleNames:       []string{"api", "worker", "common"},
		DeployableModules: []string{"api", "worker"},
		Types: []ProjectType{
			{Name: "springboot", BuildTool: "gradle", Confidence: 95, Details: "Spring Boot with Gradle"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "api:") {
		t.Error("expected api module in config")
	}
	if !searchString(config, "worker:") {
		t.Error("expected worker module in config")
	}
	if !searchString(config, "common") {
		t.Error("expected comment about excluded modules")
	}
}

func TestGenerateConfigWithConfigDirPresent(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "my-app",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"my-app"},
		HasConfigDir:      true,
		Types: []ProjectType{
			{Name: "springboot", BuildTool: "gradle", Confidence: 95, Details: "Spring Boot with Gradle"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "config_dir:") {
		t.Error("expected config_dir in generated config")
	}
}

func TestInitExistingConfigNoForce(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	os.WriteFile(configPath, []byte("existing"), 0644)

	err := Init(dir, false)
	if err == nil {
		t.Fatal("expected error when tow.yaml exists and force=false")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "tow.yaml")
	os.WriteFile(configPath, []byte("old"), 0644)

	err := Init(dir, true)
	if err != nil {
		t.Fatalf("expected Init with force to succeed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	if string(data) == "old" {
		t.Error("expected config to be overwritten")
	}
}

func TestDifferenceHelper(t *testing.T) {
	a := []string{"api", "worker", "common", "batch"}
	b := []string{"api", "worker"}

	result := difference(a, b)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(result), result)
	}
}

func TestFilterDeployableModulesAllLibraries(t *testing.T) {
	dir := t.TempDir()
	// All modules are libraries - should return all instead of empty
	modules := []string{"common", "shared", "lib"}
	result := filterDeployableModules(dir, modules)

	if len(result) != 3 {
		t.Errorf("expected all modules returned when all filtered, got %d: %v", len(result), result)
	}
}

func TestDetectNodeBun(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644)

	det, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if det.Primary().Name != "node" {
		t.Errorf("expected node, got %q", det.Primary().Name)
	}
}

func TestGenerateConfigMavenSingleModule(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "my-app",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"my-app"},
		Types: []ProjectType{
			{Name: "springboot", BuildTool: "maven", Confidence: 95, Details: "Spring Boot with Maven"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "mvnw") {
		t.Error("expected maven wrapper command")
	}
}

func TestGenerateConfigYarnBuildTool(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "frontend",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"frontend"},
		Types: []ProjectType{
			{Name: "node", BuildTool: "yarn", Confidence: 85, Details: "Node.js with yarn"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "yarn") {
		t.Error("expected yarn in build command")
	}
}

func TestGenerateConfigPnpmBuildTool(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "frontend",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"frontend"},
		Types: []ProjectType{
			{Name: "node", BuildTool: "pnpm", Confidence: 85, Details: "Node.js with pnpm"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "pnpm") {
		t.Error("expected pnpm in build command")
	}
}

func TestGenerateConfigPoetryBuildTool(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "backend",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"backend"},
		Types: []ProjectType{
			{Name: "python", BuildTool: "poetry", Confidence: 85, Details: "Python with poetry"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "poetry install") {
		t.Error("expected poetry install command")
	}
}

func TestGenerateConfigUVBuildTool(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "backend",
		RootDir:           "/tmp/test",
		DeployableModules: []string{"backend"},
		Types: []ProjectType{
			{Name: "python", BuildTool: "uv", Confidence: 85, Details: "Python with uv"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "uv sync") {
		t.Error("expected uv sync command")
	}
}

func TestGenerateConfigJavaMavenMultiModule(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "platform",
		RootDir:           "/tmp/test",
		MultiModule:       true,
		ModuleNames:       []string{"api", "worker"},
		DeployableModules: []string{"api", "worker"},
		Types: []ProjectType{
			{Name: "java", BuildTool: "maven", Confidence: 85, Details: "Java with Maven"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "mvnw -pl") {
		t.Error("expected maven multi-module command with -pl")
	}
}

func TestGenerateConfigJavaGradleMultiModule(t *testing.T) {
	det := &DetectedProject{
		ProjectName:       "platform",
		RootDir:           "/tmp/test",
		MultiModule:       true,
		ModuleNames:       []string{"api"},
		DeployableModules: []string{"api"},
		Types: []ProjectType{
			{Name: "java", BuildTool: "gradle", Confidence: 85, Details: "Java with Gradle"},
		},
	}

	config := GenerateConfig(det)
	if !searchString(config, "gradlew :api:") {
		t.Error("expected gradle multi-module command")
	}
}
