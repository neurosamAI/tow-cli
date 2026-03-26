package initializer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/logger"
)

// ProjectType represents a detected project type
type ProjectType struct {
	Name       string // springboot, java, node, python, go, generic
	BuildTool  string // gradle, maven, npm, yarn, pnpm, pip, poetry, go
	Confidence int    // 0-100
	Details    string // e.g., "Spring Boot with Gradle"
}

// DetectedProject holds the full detection result
type DetectedProject struct {
	ProjectName       string
	Types             []ProjectType
	HasDocker         bool
	HasCI             bool
	MultiModule       bool
	ModuleNames       []string // all detected modules
	DeployableModules []string // only deployable modules (excluding common/lib)
	RootDir           string
	HasScriptDir      bool // has a script/ directory with deploy scripts
	HasConfigDir      bool // has a config/ directory with env configs
	HasExternalPkg    bool // has external-package/ directory
}

// Detect scans the current directory and detects the project type
func Detect(dir string) (*DetectedProject, error) {
	result := &DetectedProject{
		RootDir:     dir,
		ProjectName: filepath.Base(dir),
	}

	// Detect project types (ordered by specificity)
	detectors := []func(string, *DetectedProject){
		detectSpringBoot,
		detectKotlin,
		detectJavaGradle,
		detectJavaMaven,
		detectNode,
		detectPython,
		detectGo,
		detectRust,
		detectPHP,
		detectRuby,
		detectDotNet,
		detectElixir,
	}

	for _, detect := range detectors {
		detect(dir, result)
	}

	// Detect additional features
	result.HasDocker = fileExists(filepath.Join(dir, "Dockerfile")) ||
		fileExists(filepath.Join(dir, "docker-compose.yml")) ||
		fileExists(filepath.Join(dir, "docker-compose.yaml"))

	result.HasCI = fileExists(filepath.Join(dir, ".github", "workflows")) ||
		fileExists(filepath.Join(dir, ".gitlab-ci.yml")) ||
		fileExists(filepath.Join(dir, "Jenkinsfile"))

	// Detect project structure patterns (like noriter/monee server-manager style)
	result.HasScriptDir = fileExists(filepath.Join(dir, "script"))
	result.HasConfigDir = fileExists(filepath.Join(dir, "config"))
	result.HasExternalPkg = fileExists(filepath.Join(dir, "external-package"))

	// Filter deployable modules (exclude common/library modules)
	if result.MultiModule && len(result.ModuleNames) > 0 {
		result.DeployableModules = filterDeployableModules(dir, result.ModuleNames)
	} else if !result.MultiModule {
		result.DeployableModules = []string{result.ProjectName}
	}

	// If nothing detected, use generic
	if len(result.Types) == 0 {
		result.Types = append(result.Types, ProjectType{
			Name:       "generic",
			Confidence: 30,
			Details:    "No specific project type detected",
		})
	}

	return result, nil
}

// filterDeployableModules separates deployable modules from library/common modules
func filterDeployableModules(dir string, modules []string) []string {
	// Patterns that indicate a library/common module (not independently deployable)
	libraryPatterns := []string{
		"common", "lib", "shared", "core", "support",
		"api-common", "proto", "protobuf", "model",
		"client-common", "kafka-common", "mongodb-common",
		"sdk", "util", "utils",
	}

	var deployable []string
	for _, mod := range modules {
		isLibrary := false
		modLower := strings.ToLower(mod)

		for _, pattern := range libraryPatterns {
			// Check if module name ends with the pattern or is exactly the pattern
			if strings.HasSuffix(modLower, "-"+pattern) ||
				strings.HasSuffix(modLower, "_"+pattern) ||
				modLower == pattern {
				isLibrary = true
				break
			}
		}

		// Check build.gradle for explicitly disabled bootJar (library module)
		if !isLibrary {
			modDir := filepath.Join(dir, mod)
			for _, gradleFile := range []string{"build.gradle", "build.gradle.kts"} {
				content := readFileContent(filepath.Join(modDir, gradleFile))
				if content != "" && strings.Contains(content, "bootJar") && strings.Contains(content, "enabled = false") {
					isLibrary = true
				}
			}
		}

		if !isLibrary {
			deployable = append(deployable, mod)
		}
	}

	// If filtering removed everything, return all modules
	if len(deployable) == 0 {
		return modules
	}

	return deployable
}

// Primary returns the highest-confidence detected type
func (d *DetectedProject) Primary() ProjectType {
	if len(d.Types) == 0 {
		return ProjectType{Name: "generic"}
	}
	best := d.Types[0]
	for _, t := range d.Types[1:] {
		if t.Confidence > best.Confidence {
			best = t
		}
	}
	return best
}

// GenerateConfig creates a tow.yaml based on detected project
func GenerateConfig(det *DetectedProject) string {
	primary := det.Primary()
	var b strings.Builder

	b.WriteString("# Tow Configuration\n")
	b.WriteString(fmt.Sprintf("# Detected: %s\n", primary.Details))
	b.WriteString("# Edit this file to match your deployment topology.\n\n")

	b.WriteString("project:\n")
	b.WriteString(fmt.Sprintf("  name: %s\n", det.ProjectName))
	b.WriteString("  base_dir: /app\n\n")

	// Defaults
	b.WriteString("defaults:\n")
	b.WriteString("  ssh_user: ec2-user\n")
	b.WriteString("  ssh_port: 22\n")
	b.WriteString("  ssh_key_path: ~/.ssh/my-key.pem\n\n")

	// Environments
	b.WriteString("environments:\n")
	b.WriteString("  dev:\n")
	b.WriteString("    ssh_key_path: ~/.ssh/dev-key.pem\n")
	b.WriteString("    servers:\n")
	b.WriteString("      - number: 1\n")
	b.WriteString("        host: 0.0.0.0  # TODO: set your server IP\n\n")

	b.WriteString("  prod:\n")
	b.WriteString("    ssh_key_path: ~/.ssh/prod-key.pem\n")
	b.WriteString("    branch: main\n")
	b.WriteString("    servers:\n")
	b.WriteString("      - number: 1\n")
	b.WriteString("        host: 0.0.0.0  # TODO: set your server IP\n")

	// Modules
	b.WriteString("\nmodules:\n")

	if det.MultiModule && len(det.DeployableModules) > 0 {
		for _, modName := range det.DeployableModules {
			writeModuleConfig(&b, modName, primary, det)
		}

		// Add comment about excluded modules
		excluded := difference(det.ModuleNames, det.DeployableModules)
		if len(excluded) > 0 {
			b.WriteString(fmt.Sprintf("  # Library modules excluded: %s\n", strings.Join(excluded, ", ")))
		}
	} else {
		writeModuleConfig(&b, det.ProjectName, primary, det)
	}

	return b.String()
}

func writeModuleConfig(b *strings.Builder, modName string, pt ProjectType, det *DetectedProject) {
	b.WriteString(fmt.Sprintf("  %s:\n", modName))
	b.WriteString(fmt.Sprintf("    type: %s\n", pt.Name))

	switch pt.Name {
	case "springboot":
		writeSpringBootModule(b, modName, pt, det)
	case "java":
		writeJavaModule(b, modName, pt, det)
	case "node":
		writeNodeModule(b, modName, pt)
	case "python":
		writePythonModule(b, modName, pt)
	case "go":
		writeGoModule(b, modName)
	case "rust":
		writeRustModule(b, modName)
	default:
		writeGenericModule(b, modName)
	}

	// Add config_dir if the project has a config directory structure
	if det.HasConfigDir {
		configDir := filepath.Join("config")
		if det.MultiModule {
			// Check for module-specific config: config/{module}/ or {module}/config/
			modConfigDir := filepath.Join(det.RootDir, "config", modName)
			modLocalConfigDir := filepath.Join(det.RootDir, modName, "config")
			if fileExists(modConfigDir) {
				configDir = filepath.Join("config", modName)
			} else if fileExists(modLocalConfigDir) {
				configDir = filepath.Join(modName, "config")
			}
		}
		b.WriteString(fmt.Sprintf("    config_dir: %s  # supports hierarchical: %s/prod-1/ > %s/prod/\n", configDir, configDir, configDir))
	}

	b.WriteString("\n")
}

// difference returns elements in a that are not in b
func difference(a, b []string) []string {
	bSet := make(map[string]bool)
	for _, s := range b {
		bSet[s] = true
	}
	var result []string
	for _, s := range a {
		if !bSet[s] {
			result = append(result, s)
		}
	}
	return result
}

func writeSpringBootModule(b *strings.Builder, modName string, pt ProjectType, det *DetectedProject) {
	b.WriteString("    port: 8080\n")
	if pt.BuildTool == "gradle" {
		if det.MultiModule {
			b.WriteString(fmt.Sprintf("    build_cmd: \"./gradlew :%s:clean :%s:bootJar -Pprofile=${ENV}\"\n", modName, modName))
			b.WriteString(fmt.Sprintf("    artifact_path: %s/build/libs/%s.jar\n", modName, modName))
		} else {
			b.WriteString("    build_cmd: \"./gradlew clean bootJar -Pprofile=${ENV}\"\n")
			b.WriteString(fmt.Sprintf("    artifact_path: build/libs/%s.jar\n", modName))
		}
	} else { // maven
		if det.MultiModule {
			b.WriteString(fmt.Sprintf("    build_cmd: \"./mvnw -pl %s clean package -P ${ENV} -DskipTests\"\n", modName))
			b.WriteString(fmt.Sprintf("    artifact_path: %s/target/%s.jar\n", modName, modName))
		} else {
			b.WriteString("    build_cmd: \"./mvnw clean package -P ${ENV} -DskipTests\"\n")
			b.WriteString(fmt.Sprintf("    artifact_path: target/%s.jar\n", modName))
		}
	}
	b.WriteString("    health_check:\n")
	b.WriteString("      type: http\n")
	b.WriteString("      target: http://localhost:8080/actuator/health\n")
	b.WriteString("      timeout: 120\n")
	b.WriteString("      interval: 3\n")
}

func writeJavaModule(b *strings.Builder, modName string, pt ProjectType, det *DetectedProject) {
	b.WriteString("    port: 8080\n")
	if pt.BuildTool == "gradle" {
		if det.MultiModule {
			b.WriteString(fmt.Sprintf("    build_cmd: \"./gradlew :%s:clean :%s:build\"\n", modName, modName))
			b.WriteString(fmt.Sprintf("    artifact_path: %s/build/libs/%s.jar\n", modName, modName))
		} else {
			b.WriteString("    build_cmd: \"./gradlew clean build\"\n")
			b.WriteString(fmt.Sprintf("    artifact_path: build/libs/%s.jar\n", modName))
		}
	} else {
		if det.MultiModule {
			b.WriteString(fmt.Sprintf("    build_cmd: \"./mvnw -pl %s clean package -DskipTests\"\n", modName))
			b.WriteString(fmt.Sprintf("    artifact_path: %s/target/%s.jar\n", modName, modName))
		} else {
			b.WriteString("    build_cmd: \"./mvnw clean package -DskipTests\"\n")
			b.WriteString(fmt.Sprintf("    artifact_path: target/%s.jar\n", modName))
		}
	}
	b.WriteString("    health_check:\n")
	b.WriteString("      type: tcp\n")
	b.WriteString("      timeout: 60\n")
}

func writeNodeModule(b *strings.Builder, modName string, pt ProjectType) {
	b.WriteString("    port: 3000\n")
	switch pt.BuildTool {
	case "yarn":
		b.WriteString("    build_cmd: \"yarn install --frozen-lockfile && yarn build\"\n")
	case "pnpm":
		b.WriteString("    build_cmd: \"pnpm install --frozen-lockfile && pnpm build\"\n")
	default:
		b.WriteString("    build_cmd: \"npm ci && npm run build\"\n")
	}
	b.WriteString(fmt.Sprintf("    artifact_path: build/%s.tar.gz\n", modName))
	b.WriteString("    health_check:\n")
	b.WriteString("      type: tcp\n")
	b.WriteString("      timeout: 30\n")
}

func writePythonModule(b *strings.Builder, modName string, pt ProjectType) {
	b.WriteString("    port: 8000\n")
	switch pt.BuildTool {
	case "poetry":
		b.WriteString("    build_cmd: \"poetry install --no-dev\"\n")
	case "uv":
		b.WriteString("    build_cmd: \"uv sync --no-dev\"\n")
	default:
		b.WriteString("    build_cmd: \"pip install -r requirements.txt\"\n")
	}
	b.WriteString(fmt.Sprintf("    artifact_path: build/%s.tar.gz\n", modName))
	b.WriteString("    health_check:\n")
	b.WriteString("      type: tcp\n")
	b.WriteString("      timeout: 30\n")
}

func writeGoModule(b *strings.Builder, modName string) {
	b.WriteString("    port: 8080\n")
	b.WriteString(fmt.Sprintf("    build_cmd: \"go build -o bin/%s ./cmd/%s\"\n", modName, modName))
	b.WriteString(fmt.Sprintf("    artifact_path: bin/%s\n", modName))
	b.WriteString("    health_check:\n")
	b.WriteString("      type: tcp\n")
	b.WriteString("      timeout: 30\n")
}

func writeRustModule(b *strings.Builder, modName string) {
	b.WriteString("    port: 8080\n")
	b.WriteString("    build_cmd: \"cargo build --release\"\n")
	b.WriteString(fmt.Sprintf("    artifact_path: target/release/%s\n", modName))
	b.WriteString("    health_check:\n")
	b.WriteString("      type: tcp\n")
	b.WriteString("      timeout: 30\n")
}

func writeGenericModule(b *strings.Builder, modName string) {
	b.WriteString("    port: 8080\n")
	b.WriteString("    build_cmd: \"\"  # TODO: set your build command\n")
	b.WriteString(fmt.Sprintf("    artifact_path: build/%s.tar.gz\n", modName))
	b.WriteString("    health_check:\n")
	b.WriteString("      type: tcp\n")
	b.WriteString("      timeout: 30\n")
}

// Init runs the full initialization flow
func Init(dir string, force bool, withAI ...bool) error {
	configPath := filepath.Join(dir, "tow.yaml")

	if fileExists(configPath) && !force {
		return fmt.Errorf("tow.yaml already exists. Use --force to overwrite")
	}

	logger.Header("Detecting project type")

	det, err := Detect(dir)
	if err != nil {
		return fmt.Errorf("project detection failed: %w", err)
	}

	primary := det.Primary()
	logger.Success("Detected: %s (confidence: %d%%)", primary.Details, primary.Confidence)

	if det.MultiModule {
		logger.Info("Multi-module project with %d modules: %s", len(det.ModuleNames), strings.Join(det.ModuleNames, ", "))
	}

	if det.HasDocker {
		logger.Info("Docker configuration found")
	}

	// Generate and write config
	config := GenerateConfig(det)

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("writing tow.yaml: %w", err)
	}

	logger.Success("Created tow.yaml")

	// Generate running scripts
	logger.Header("Generating running scripts")
	if err := GenerateScripts(det); err != nil {
		return fmt.Errorf("generating scripts: %w", err)
	}

	// Auto-setup AI integrations (opt-in)
	if len(withAI) > 0 && withAI[0] {
		setupAIIntegrations(dir)
	}

	logger.Info("")
	logger.Info("Next steps:")
	logger.Info("  1. Edit tow.yaml — set server IPs and SSH key paths")
	logger.Info("  2. Review generated scripts in script/ (or {module}/script/)")
	logger.Info("  3. Run: tow setup -e dev -m %s    (initialize remote server)", det.DeployableModules[0])
	logger.Info("  4. Run: tow deploy -e dev -m %s   (deploy your app)", det.DeployableModules[0])
	logger.Info("")
	logger.Info("Tip: Create tow.local.yaml for machine-specific overrides (add it to .gitignore)")

	return nil
}

// setupAIIntegrations auto-generates Claude Code skill and MCP config hints
func setupAIIntegrations(dir string) {
	// Claude Code skill
	claudeSkillDir := filepath.Join(dir, ".claude", "skills")
	claudeSkillPath := filepath.Join(claudeSkillDir, "tow-deploy.md")
	if !fileExists(claudeSkillPath) {
		if err := os.MkdirAll(claudeSkillDir, 0755); err == nil {
			skill := generateClaudeSkill()
			if err := os.WriteFile(claudeSkillPath, []byte(skill), 0644); err == nil {
				logger.Success("Created .claude/skills/tow-deploy.md (Claude Code AI integration)")
			}
		}
	}

	// .claude/settings.json — add MCP server hint
	claudeSettingsPath := filepath.Join(dir, ".claude", "settings.json")
	if !fileExists(claudeSettingsPath) {
		mcpHint := `{
  "mcpServers": {
    "tow": {
      "command": "tow",
      "args": ["mcp-server"]
    }
  }
}
`
		if err := os.WriteFile(claudeSettingsPath, []byte(mcpHint), 0644); err == nil {
			logger.Success("Created .claude/settings.json (MCP server config)")
		}
	}
}

func generateClaudeSkill() string {
	return `You have access to the ` + "`tow`" + ` CLI for deploying applications to remote servers.

## Available Commands

` + "```" + `bash
tow status -e <env> -m <module>           # Check status
tow auto -e <env> -m <module> -y          # Full deploy pipeline
tow auto -e <env> -m <module> --rolling -y        # Rolling deploy
tow auto -e <env> -m <module> --auto-rollback -y  # Deploy with auto-rollback
tow rollback -e <env> -m <module> -y      # Rollback
tow logs -e <env> -m <module> -n 50       # Recent logs
tow logs -e <env> -m <module> -f "ERROR"  # Filter errors
tow list deployments -e <env> -m <module> # Deployment history
tow start -e <env> -m <module>            # Start
tow stop -e <env> -m <module>             # Stop
tow cleanup -e <env> -m <module> --keep 3 # Clean old deploys
` + "```" + `

## Safety Rules

1. ALWAYS check status before deploying
2. ALWAYS use -y flag when running non-interactively
3. For production: confirm with the user before deploy/rollback
4. After deploy: verify with status and check logs for errors
5. If deploy fails: check logs first, then consider rollback
`
}

// --- Detectors ---

func detectSpringBoot(dir string, result *DetectedProject) {
	// Check for Spring Boot markers
	isSpringBoot := false

	// Check build.gradle for spring boot plugin
	if content := readFileContent(filepath.Join(dir, "build.gradle")); content != "" {
		if strings.Contains(content, "org.springframework.boot") || strings.Contains(content, "spring-boot") {
			isSpringBoot = true
		}
	}
	if content := readFileContent(filepath.Join(dir, "build.gradle.kts")); content != "" {
		if strings.Contains(content, "org.springframework.boot") || strings.Contains(content, "spring-boot") {
			isSpringBoot = true
		}
	}
	if content := readFileContent(filepath.Join(dir, "pom.xml")); content != "" {
		if strings.Contains(content, "spring-boot") || strings.Contains(content, "springframework") {
			isSpringBoot = true
		}
	}

	if !isSpringBoot {
		return
	}

	buildTool := "gradle"
	if fileExists(filepath.Join(dir, "pom.xml")) && !fileExists(filepath.Join(dir, "build.gradle")) && !fileExists(filepath.Join(dir, "build.gradle.kts")) {
		buildTool = "maven"
	}

	pt := ProjectType{
		Name:       "springboot",
		BuildTool:  buildTool,
		Confidence: 95,
		Details:    fmt.Sprintf("Spring Boot with %s", capitalize(buildTool)),
	}

	result.Types = append(result.Types, pt)

	// Detect multi-module
	detectJavaMultiModule(dir, buildTool, result)
}

func detectJavaGradle(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "build.gradle")) && !fileExists(filepath.Join(dir, "build.gradle.kts")) {
		return
	}

	// Skip if already detected as Spring Boot or Kotlin
	for _, t := range result.Types {
		if t.Name == "springboot" || t.Name == "kotlin" {
			return
		}
	}

	confidence := 85
	details := "Java with Gradle"

	// Check for Java frameworks
	content := readFileContent(filepath.Join(dir, "build.gradle"))
	if content == "" {
		content = readFileContent(filepath.Join(dir, "build.gradle.kts"))
	}

	switch {
	case strings.Contains(content, "io.quarkus"):
		details = "Quarkus with Gradle"
		confidence = 93
	case strings.Contains(content, "io.micronaut"):
		details = "Micronaut with Gradle"
		confidence = 92
	case strings.Contains(content, "io.dropwizard"):
		details = "Dropwizard with Gradle"
		confidence = 90
	}

	pt := ProjectType{
		Name:       "java",
		BuildTool:  "gradle",
		Confidence: confidence,
		Details:    details,
	}

	result.Types = append(result.Types, pt)
	detectJavaMultiModule(dir, "gradle", result)
}

func detectJavaMaven(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "pom.xml")) {
		return
	}

	// Skip if already detected
	for _, t := range result.Types {
		if t.Name == "springboot" || t.Name == "java" || t.Name == "kotlin" {
			return
		}
	}

	confidence := 85
	details := "Java with Maven"

	content := readFileContent(filepath.Join(dir, "pom.xml"))
	switch {
	case strings.Contains(content, "io.quarkus"):
		details = "Quarkus with Maven"
		confidence = 93
	case strings.Contains(content, "io.micronaut"):
		details = "Micronaut with Maven"
		confidence = 92
	case strings.Contains(content, "io.dropwizard"):
		details = "Dropwizard with Maven"
		confidence = 90
	}

	pt := ProjectType{
		Name:       "java",
		BuildTool:  "maven",
		Confidence: confidence,
		Details:    details,
	}

	result.Types = append(result.Types, pt)
	detectJavaMultiModule(dir, "maven", result)
}

func detectJavaMultiModule(dir, buildTool string, result *DetectedProject) {
	if result.MultiModule {
		return // already detected
	}

	if buildTool == "gradle" {
		// Check settings.gradle for include statements
		for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
			content := readFileContent(filepath.Join(dir, name))
			if content == "" {
				continue
			}
			modules := parseGradleIncludes(content)
			if len(modules) > 0 {
				result.MultiModule = true
				result.ModuleNames = modules
				return
			}
		}
	} else if buildTool == "maven" {
		content := readFileContent(filepath.Join(dir, "pom.xml"))
		if content != "" {
			modules := parseMavenModules(content)
			if len(modules) > 0 {
				result.MultiModule = true
				result.ModuleNames = modules
			}
		}
	}
}

func detectNode(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "package.json")) {
		return
	}

	buildTool := "npm"
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		buildTool = "yarn"
	} else if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		buildTool = "pnpm"
	}

	confidence := 85
	details := fmt.Sprintf("Node.js with %s", buildTool)

	// Check for monorepo
	if fileExists(filepath.Join(dir, "lerna.json")) ||
		fileExists(filepath.Join(dir, "nx.json")) ||
		fileExists(filepath.Join(dir, "turbo.json")) {
		details += " (monorepo)"
	}

	// Check for framework
	content := readFileContent(filepath.Join(dir, "package.json"))
	if strings.Contains(content, "\"next\"") {
		details = fmt.Sprintf("Next.js with %s", buildTool)
		confidence = 90
	} else if strings.Contains(content, "\"nuxt\"") {
		details = fmt.Sprintf("Nuxt.js with %s", buildTool)
		confidence = 90
	} else if strings.Contains(content, "\"nest\"") || strings.Contains(content, "\"@nestjs/") {
		details = fmt.Sprintf("NestJS with %s", buildTool)
		confidence = 90
	} else if strings.Contains(content, "\"express\"") {
		details = fmt.Sprintf("Express.js with %s", buildTool)
		confidence = 88
	} else if strings.Contains(content, "\"fastify\"") {
		details = fmt.Sprintf("Fastify with %s", buildTool)
		confidence = 90
	} else if strings.Contains(content, "\"hono\"") {
		details = fmt.Sprintf("Hono with %s", buildTool)
		confidence = 88
	} else if strings.Contains(content, "\"koa\"") {
		details = fmt.Sprintf("Koa with %s", buildTool)
		confidence = 88
	}

	result.Types = append(result.Types, ProjectType{
		Name:       "node",
		BuildTool:  buildTool,
		Confidence: confidence,
		Details:    details,
	})
}

func detectPython(dir string, result *DetectedProject) {
	buildTool := ""
	details := ""
	confidence := 0

	if fileExists(filepath.Join(dir, "pyproject.toml")) {
		content := readFileContent(filepath.Join(dir, "pyproject.toml"))
		if strings.Contains(content, "[tool.poetry]") {
			buildTool = "poetry"
			details = "Python with Poetry"
			confidence = 90
		} else if strings.Contains(content, "[tool.uv]") || fileExists(filepath.Join(dir, "uv.lock")) {
			buildTool = "uv"
			details = "Python with uv"
			confidence = 90
		} else {
			buildTool = "pip"
			details = "Python with pyproject.toml"
			confidence = 80
		}

		// Check for framework
		if strings.Contains(content, "fastapi") {
			details = fmt.Sprintf("FastAPI with %s", buildTool)
			confidence = 92
		} else if strings.Contains(content, "django") {
			details = fmt.Sprintf("Django with %s", buildTool)
			confidence = 92
		} else if strings.Contains(content, "flask") {
			details = fmt.Sprintf("Flask with %s", buildTool)
			confidence = 92
		} else if strings.Contains(content, "litestar") {
			details = fmt.Sprintf("Litestar with %s", buildTool)
			confidence = 90
		} else if strings.Contains(content, "tornado") {
			details = fmt.Sprintf("Tornado with %s", buildTool)
			confidence = 88
		}
	} else if fileExists(filepath.Join(dir, "requirements.txt")) {
		buildTool = "pip"
		confidence = 75
		content := readFileContent(filepath.Join(dir, "requirements.txt"))
		if strings.Contains(content, "fastapi") || strings.Contains(content, "FastAPI") {
			details = "FastAPI with pip"
			confidence = 88
		} else if strings.Contains(content, "django") || strings.Contains(content, "Django") {
			details = "Django with pip"
			confidence = 88
		} else if strings.Contains(content, "flask") || strings.Contains(content, "Flask") {
			details = "Flask with pip"
			confidence = 88
		} else {
			details = "Python with pip"
		}
	} else if fileExists(filepath.Join(dir, "setup.py")) {
		buildTool = "pip"
		details = "Python with setup.py"
		confidence = 70
	}

	if confidence > 0 {
		result.Types = append(result.Types, ProjectType{
			Name:       "python",
			BuildTool:  buildTool,
			Confidence: confidence,
			Details:    details,
		})
	}
}

func detectGo(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "go.mod")) {
		return
	}

	confidence := 90
	details := "Go"

	// Detect Go framework
	content := readFileContent(filepath.Join(dir, "go.mod"))
	switch {
	case strings.Contains(content, "github.com/gin-gonic/gin"):
		details = "Go with Gin"
		confidence = 93
	case strings.Contains(content, "github.com/labstack/echo"):
		details = "Go with Echo"
		confidence = 93
	case strings.Contains(content, "github.com/gofiber/fiber"):
		details = "Go with Fiber"
		confidence = 93
	case strings.Contains(content, "github.com/go-chi/chi"):
		details = "Go with Chi"
		confidence = 92
	}

	result.Types = append(result.Types, ProjectType{
		Name:       "go",
		BuildTool:  "go",
		Confidence: confidence,
		Details:    details,
	})
}

func detectRust(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "Cargo.toml")) {
		return
	}

	confidence := 90
	details := "Rust with Cargo"

	content := readFileContent(filepath.Join(dir, "Cargo.toml"))
	switch {
	case strings.Contains(content, "axum"):
		details = "Rust with Axum"
		confidence = 93
	case strings.Contains(content, "actix-web"):
		details = "Rust with Actix Web"
		confidence = 93
	case strings.Contains(content, "rocket"):
		details = "Rust with Rocket"
		confidence = 93
	case strings.Contains(content, "warp"):
		details = "Rust with Warp"
		confidence = 92
	case strings.Contains(content, "poem"):
		details = "Rust with Poem"
		confidence = 92
	}

	result.Types = append(result.Types, ProjectType{
		Name:       "rust",
		BuildTool:  "cargo",
		Confidence: confidence,
		Details:    details,
	})
}

func detectPHP(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "composer.json")) {
		return
	}

	content := readFileContent(filepath.Join(dir, "composer.json"))
	confidence := 80
	details := "PHP with Composer"

	switch {
	case strings.Contains(content, "laravel/framework"):
		details = "Laravel"
		confidence = 95
	case strings.Contains(content, "symfony/"):
		details = "Symfony"
		confidence = 93
	case strings.Contains(content, "codeigniter4/framework"):
		details = "CodeIgniter"
		confidence = 92
	case strings.Contains(content, "slim/slim"):
		details = "Slim"
		confidence = 90
	case strings.Contains(content, "cakephp/cakephp"):
		details = "CakePHP"
		confidence = 90
	}

	result.Types = append(result.Types, ProjectType{
		Name:       "php",
		BuildTool:  "composer",
		Confidence: confidence,
		Details:    details,
	})
}

func detectRuby(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "Gemfile")) {
		return
	}

	content := readFileContent(filepath.Join(dir, "Gemfile"))
	confidence := 80
	details := "Ruby"

	switch {
	case strings.Contains(content, "'rails'") || strings.Contains(content, "\"rails\""):
		details = "Ruby on Rails"
		confidence = 95
	case strings.Contains(content, "'sinatra'") || strings.Contains(content, "\"sinatra\""):
		details = "Sinatra"
		confidence = 90
	case strings.Contains(content, "'hanami'") || strings.Contains(content, "\"hanami\""):
		details = "Hanami"
		confidence = 90
	case strings.Contains(content, "'grape'") || strings.Contains(content, "\"grape\""):
		details = "Grape"
		confidence = 88
	case strings.Contains(content, "'roda'") || strings.Contains(content, "\"roda\""):
		details = "Roda"
		confidence = 88
	}

	buildTool := "bundler"
	result.Types = append(result.Types, ProjectType{
		Name:       "ruby",
		BuildTool:  buildTool,
		Confidence: confidence,
		Details:    details,
	})
}

func detectDotNet(dir string, result *DetectedProject) {
	// Look for .csproj files
	csprojFiles, _ := filepath.Glob(filepath.Join(dir, "*.csproj"))
	if len(csprojFiles) == 0 {
		csprojFiles, _ = filepath.Glob(filepath.Join(dir, "*", "*.csproj"))
	}
	if len(csprojFiles) == 0 {
		return
	}

	confidence := 85
	details := "ASP.NET Core"

	for _, f := range csprojFiles {
		content := readFileContent(f)
		switch {
		case strings.Contains(content, "Microsoft.AspNetCore.Components.Server"):
			details = "Blazor Server"
			confidence = 92
		case strings.Contains(content, "Carter"):
			details = "ASP.NET Core with Carter"
			confidence = 90
		case strings.Contains(content, "Microsoft.AspNetCore"):
			details = "ASP.NET Core"
			confidence = 90
		}
	}

	// Check for solution file
	slnFiles, _ := filepath.Glob(filepath.Join(dir, "*.sln"))
	if len(slnFiles) > 0 {
		details += " (Solution)"
	}

	result.Types = append(result.Types, ProjectType{
		Name:       "dotnet",
		BuildTool:  "dotnet",
		Confidence: confidence,
		Details:    details,
	})
}

func detectKotlin(dir string, result *DetectedProject) {
	// Kotlin with Gradle
	gradleKts := readFileContent(filepath.Join(dir, "build.gradle.kts"))
	if gradleKts == "" {
		return
	}

	if !strings.Contains(gradleKts, "kotlin") {
		return
	}

	// Skip if Spring Boot (handled by detectSpringBoot)
	if strings.Contains(gradleKts, "org.springframework.boot") {
		return
	}

	confidence := 85
	details := "Kotlin"

	switch {
	case strings.Contains(gradleKts, "io.ktor"):
		details = "Kotlin with Ktor"
		confidence = 93
	case strings.Contains(gradleKts, "io.micronaut"):
		details = "Kotlin with Micronaut"
		confidence = 92
	case strings.Contains(gradleKts, "io.javalin"):
		details = "Kotlin with Javalin"
		confidence = 90
	case strings.Contains(gradleKts, "org.http4k"):
		details = "Kotlin with http4k"
		confidence = 90
	}

	result.Types = append(result.Types, ProjectType{
		Name:       "kotlin",
		BuildTool:  "gradle",
		Confidence: confidence,
		Details:    details,
	})
}

func detectElixir(dir string, result *DetectedProject) {
	if !fileExists(filepath.Join(dir, "mix.exs")) {
		return
	}

	content := readFileContent(filepath.Join(dir, "mix.exs"))
	confidence := 85
	details := "Elixir"

	switch {
	case strings.Contains(content, ":phoenix"):
		details = "Elixir with Phoenix"
		confidence = 95
	case strings.Contains(content, ":ash") && strings.Contains(content, ":ash_phoenix"):
		details = "Elixir with Ash Framework"
		confidence = 92
	case strings.Contains(content, ":absinthe"):
		details = "Elixir with Absinthe (GraphQL)"
		confidence = 90
	case strings.Contains(content, ":bandit"):
		details = "Elixir with Bandit"
		confidence = 88
	case strings.Contains(content, ":plug_cowboy") || strings.Contains(content, ":plug"):
		details = "Elixir with Plug"
		confidence = 88
	}

	result.Types = append(result.Types, ProjectType{
		Name:       "elixir",
		BuildTool:  "mix",
		Confidence: confidence,
		Details:    details,
	})
}

// --- Helpers ---

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFileContent(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseGradleIncludes(content string) []string {
	var modules []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "include") || strings.HasPrefix(line, "include(") {
			// Extract module names from include 'module1', 'module2' or include("module1")
			for _, sep := range []string{"'", "\"", "`"} {
				parts := strings.Split(line, sep)
				for i := 1; i < len(parts); i += 2 {
					name := strings.TrimPrefix(parts[i], ":")
					if name != "" && !strings.HasPrefix(name, "//") {
						modules = append(modules, name)
					}
				}
			}
		}
	}
	return modules
}

func parseMavenModules(content string) []string {
	var modules []string
	inModules := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "<modules>" {
			inModules = true
			continue
		}
		if line == "</modules>" {
			break
		}
		if inModules && strings.HasPrefix(line, "<module>") {
			name := strings.TrimPrefix(line, "<module>")
			name = strings.TrimSuffix(name, "</module>")
			name = strings.TrimSpace(name)
			if name != "" {
				modules = append(modules, name)
			}
		}
	}
	return modules
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
