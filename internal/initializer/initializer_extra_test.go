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

func TestGenerateScripts(t *testing.T) {
	dir := t.TempDir()

	det := &DetectedProject{
		ProjectName:       "my-api",
		RootDir:           dir,
		DeployableModules: []string{"my-api"},
		Types: []ProjectType{
			{Name: "springboot", BuildTool: "gradle", Confidence: 95, Details: "Spring Boot with Gradle"},
		},
	}

	err := GenerateScripts(det)
	if err != nil {
		t.Fatalf("GenerateScripts failed: %v", err)
	}

	// Verify script/env.sh was created
	envPath := filepath.Join(dir, "script", "env.sh")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Error("expected script/env.sh to be created")
	} else {
		data, _ := os.ReadFile(envPath)
		content := string(data)
		if !strings.Contains(content, "MODULE_NAME") {
			t.Error("expected MODULE_NAME in env.sh")
		}
		if !strings.Contains(content, "my-api") {
			t.Error("expected module name 'my-api' in env.sh")
		}
		if !strings.Contains(content, "JAVA_OPTS") {
			t.Error("expected JAVA_OPTS in springboot env.sh")
		}
	}

	// Verify script/server was created
	serverPath := filepath.Join(dir, "script", "server")
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		t.Error("expected script/server to be created")
	} else {
		data, _ := os.ReadFile(serverPath)
		content := string(data)
		if !strings.Contains(content, "start()") {
			t.Error("expected start() function in server script")
		}
		if !strings.Contains(content, "stop()") {
			t.Error("expected stop() function in server script")
		}
		if !strings.Contains(content, "threaddump") {
			t.Error("expected threaddump function for springboot type")
		}
	}
}

func TestGenerateScriptsMultiModule(t *testing.T) {
	dir := t.TempDir()

	det := &DetectedProject{
		ProjectName:       "platform",
		RootDir:           dir,
		MultiModule:       true,
		DeployableModules: []string{"api", "worker"},
		Types: []ProjectType{
			{Name: "node", BuildTool: "npm", Confidence: 85, Details: "Node.js"},
		},
	}

	err := GenerateScripts(det)
	if err != nil {
		t.Fatalf("GenerateScripts multi-module failed: %v", err)
	}

	// Verify scripts created for each module in {module}/script/
	for _, mod := range []string{"api", "worker"} {
		envPath := filepath.Join(dir, mod, "script", "env.sh")
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			t.Errorf("expected %s/script/env.sh to be created", mod)
		}
		serverPath := filepath.Join(dir, mod, "script", "server")
		if _, err := os.Stat(serverPath); os.IsNotExist(err) {
			t.Errorf("expected %s/script/server to be created", mod)
		}
	}
}

func TestGenerateScriptsSkipsExisting(t *testing.T) {
	dir := t.TempDir()

	// Pre-create script/env.sh
	scriptDir := filepath.Join(dir, "script")
	os.MkdirAll(scriptDir, 0755)
	os.WriteFile(filepath.Join(scriptDir, "env.sh"), []byte("existing-content"), 0755)

	det := &DetectedProject{
		ProjectName:       "my-api",
		RootDir:           dir,
		DeployableModules: []string{"my-api"},
		Types: []ProjectType{
			{Name: "go", Confidence: 90, Details: "Go"},
		},
	}

	err := GenerateScripts(det)
	if err != nil {
		t.Fatalf("GenerateScripts failed: %v", err)
	}

	// Verify existing env.sh was NOT overwritten
	data, _ := os.ReadFile(filepath.Join(scriptDir, "env.sh"))
	if string(data) != "existing-content" {
		t.Error("expected existing env.sh to be preserved")
	}
}

func TestSetupAIIntegrations(t *testing.T) {
	dir := t.TempDir()

	setupAIIntegrations(dir)

	// Verify .claude/skills/tow-deploy.md was created
	skillPath := filepath.Join(dir, ".claude", "skills", "tow-deploy.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("expected .claude/skills/tow-deploy.md to be created")
	} else {
		data, _ := os.ReadFile(skillPath)
		content := string(data)
		if !strings.Contains(content, "tow") {
			t.Error("expected 'tow' in skill content")
		}
		if !strings.Contains(content, "deploy") {
			t.Error("expected 'deploy' in skill content")
		}
	}

	// Verify .claude/settings.json was created
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("expected .claude/settings.json to be created")
	} else {
		data, _ := os.ReadFile(settingsPath)
		content := string(data)
		if !strings.Contains(content, "mcpServers") {
			t.Error("expected 'mcpServers' in settings.json")
		}
		if !strings.Contains(content, "mcp-server") {
			t.Error("expected 'mcp-server' in settings.json")
		}
	}
}

func TestSetupAIIntegrationsSkipsExisting(t *testing.T) {
	dir := t.TempDir()

	// Pre-create skill file
	skillDir := filepath.Join(dir, ".claude", "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "tow-deploy.md"), []byte("custom skill"), 0644)

	setupAIIntegrations(dir)

	// Verify existing file was NOT overwritten
	data, _ := os.ReadFile(filepath.Join(skillDir, "tow-deploy.md"))
	if string(data) != "custom skill" {
		t.Error("expected existing skill file to be preserved")
	}
}

func TestGenerateScriptsAllTypes(t *testing.T) {
	types := []struct {
		name      string
		buildTool string
		expect    string
	}{
		{"springboot", "gradle", "JAVA_OPTS"},
		{"java", "maven", "JAVA_OPTS"},
		{"node", "npm", "NODE_ENV"},
		{"python", "pip", "APP_PORT"},
		{"go", "", "GOMAXPROCS"},
		{"rust", "", "RUST_LOG"},
		{"generic", "", "START_CMD"},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			det := &DetectedProject{
				ProjectName:       "test-mod",
				RootDir:           dir,
				DeployableModules: []string{"test-mod"},
				Types: []ProjectType{
					{Name: tt.name, BuildTool: tt.buildTool, Confidence: 80, Details: tt.name},
				},
			}

			err := GenerateScripts(det)
			if err != nil {
				t.Fatalf("GenerateScripts for %s failed: %v", tt.name, err)
			}

			envPath := filepath.Join(dir, "script", "env.sh")
			data, err := os.ReadFile(envPath)
			if err != nil {
				t.Fatalf("reading env.sh: %v", err)
			}
			if !strings.Contains(string(data), tt.expect) {
				t.Errorf("expected %q in env.sh for type %s", tt.expect, tt.name)
			}
		})
	}
}
