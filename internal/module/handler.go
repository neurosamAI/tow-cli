package module

import (
	"fmt"
	"strings"
)

// Handler defines the interface for module-type-specific behavior
type Handler interface {
	// Name returns the handler type name
	Name() string

	// DefaultBuildCmd returns the default build command for this module type
	DefaultBuildCmd(moduleName, env string) string

	// DefaultStartCmd returns the default start command
	DefaultStartCmd(baseDir string, port int) string

	// DefaultStopCmd returns the default stop command
	DefaultStopCmd(baseDir string, port int) string

	// DefaultStatusCmd returns the default status command
	DefaultStatusCmd(baseDir string, port int) string

	// DefaultArtifactPath returns the expected artifact path after build
	DefaultArtifactPath(moduleName string) string

	// PackageContents returns additional files/dirs to include in the package
	PackageContents(moduleName, baseDir string) []string
}

// Registry holds all registered module handlers
var registry = map[string]Handler{}

// Register adds a handler to the registry
func Register(h Handler) {
	registry[h.Name()] = h
}

// Get returns a handler by type name
func Get(typeName string) (Handler, error) {
	h, ok := registry[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown module type: %s (available: %s)", typeName, strings.Join(Available(), ", "))
	}
	return h, nil
}

// Available returns all registered handler names
func Available() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

func init() {
	// Register built-in handlers (language/framework types only)
	// Infrastructure services (kafka, redis, etc.) are loaded as YAML plugins
	Register(&JavaHandler{})
	Register(&SpringBootHandler{})
	Register(&NodeHandler{})
	Register(&PythonHandler{})
	Register(&GoHandler{})
	Register(&RustHandler{})
	Register(&PHPHandler{})
	Register(&RubyHandler{})
	Register(&DotNetHandler{})
	Register(&KotlinHandler{})
	Register(&ElixirHandler{})
	Register(&GenericHandler{})
}

// GoHandler handles Go applications
type GoHandler struct{}

func (h *GoHandler) Name() string { return "go" }
func (h *GoHandler) DefaultBuildCmd(moduleName, env string) string {
	return fmt.Sprintf("CGO_ENABLED=0 go build -o bin/%s ./cmd/%s", moduleName, moduleName)
}
func (h *GoHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/start.sh", baseDir)
}
func (h *GoHandler) DefaultStopCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("kill $(lsof -i :%d -t) 2>/dev/null || true", port)
	}
	return fmt.Sprintf("%s/current/bin/stop.sh", baseDir)
}
func (h *GoHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *GoHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("bin/%s", moduleName)
}
func (h *GoHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{fmt.Sprintf("bin/%s", moduleName), "config/"}
}

// RustHandler handles Rust applications
type RustHandler struct{}

func (h *RustHandler) Name() string { return "rust" }
func (h *RustHandler) DefaultBuildCmd(moduleName, env string) string {
	return "cargo build --release"
}
func (h *RustHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/start.sh", baseDir)
}
func (h *RustHandler) DefaultStopCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("kill $(lsof -i :%d -t) 2>/dev/null || true", port)
	}
	return fmt.Sprintf("%s/current/bin/stop.sh", baseDir)
}
func (h *RustHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *RustHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("target/release/%s", moduleName)
}
func (h *RustHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{fmt.Sprintf("target/release/%s", moduleName), "config/"}
}

// --- Built-in Handlers ---

// JavaHandler handles generic Java applications
type JavaHandler struct{}

func (h *JavaHandler) Name() string { return "java" }
func (h *JavaHandler) DefaultBuildCmd(moduleName, env string) string {
	return fmt.Sprintf("./gradlew :%s:clean :%s:build -Pprofile=%s", moduleName, moduleName, env)
}
func (h *JavaHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/start.sh", baseDir)
}
func (h *JavaHandler) DefaultStopCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/stop.sh", baseDir)
}
func (h *JavaHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *JavaHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("%s/build/libs/%s.jar", moduleName, moduleName)
}
func (h *JavaHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"bin/", "config/", "lib/"}
}

// SpringBootHandler handles Spring Boot applications
type SpringBootHandler struct{ JavaHandler }

func (h *SpringBootHandler) Name() string { return "springboot" }
func (h *SpringBootHandler) DefaultBuildCmd(moduleName, env string) string {
	return fmt.Sprintf("./gradlew :%s:clean :%s:bootJar -Pprofile=%s", moduleName, moduleName, env)
}

// NodeHandler handles Node.js applications
type NodeHandler struct{}

func (h *NodeHandler) Name() string { return "node" }
func (h *NodeHandler) DefaultBuildCmd(moduleName, env string) string {
	return fmt.Sprintf("cd %s && npm ci && npm run build", moduleName)
}
func (h *NodeHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("cd %s/current && NODE_ENV=production node dist/main.js &", baseDir)
}
func (h *NodeHandler) DefaultStopCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("kill $(lsof -i :%d -t) 2>/dev/null || true", port)
	}
	return "pkill -f 'node dist/main.js' || true"
}
func (h *NodeHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *NodeHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("build/%s.tar.gz", moduleName)
}
func (h *NodeHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"dist/", "node_modules/", "package.json"}
}

// PythonHandler handles Python applications
type PythonHandler struct{}

func (h *PythonHandler) Name() string { return "python" }
func (h *PythonHandler) DefaultBuildCmd(moduleName, env string) string {
	return fmt.Sprintf("cd %s && pip install -r requirements.txt", moduleName)
}
func (h *PythonHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("cd %s/current && python -m app &", baseDir)
}
func (h *PythonHandler) DefaultStopCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("kill $(lsof -i :%d -t) 2>/dev/null || true", port)
	}
	return "pkill -f 'python -m app' || true"
}
func (h *PythonHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *PythonHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("build/%s.tar.gz", moduleName)
}
func (h *PythonHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"*.py", "requirements.txt"}
}

// GenericHandler handles generic services (external packages, etc.)
type GenericHandler struct{}

func (h *GenericHandler) Name() string { return "generic" }
func (h *GenericHandler) DefaultBuildCmd(moduleName, env string) string {
	return "" // No build needed
}
func (h *GenericHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/start.sh", baseDir)
}
func (h *GenericHandler) DefaultStopCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/stop.sh", baseDir)
}
func (h *GenericHandler) DefaultStatusCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
	}
	return "echo 'no status check configured'"
}
func (h *GenericHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("build/%s.tar.gz", moduleName)
}
func (h *GenericHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"bin/", "config/"}
}

// PHPHandler handles PHP applications
type PHPHandler struct{}

func (h *PHPHandler) Name() string { return "php" }
func (h *PHPHandler) DefaultBuildCmd(moduleName, env string) string {
	return "composer install --no-dev --optimize-autoloader"
}
func (h *PHPHandler) DefaultStartCmd(baseDir string, port int) string {
	if port == 0 {
		port = 8000
	}
	return fmt.Sprintf("php artisan serve --host=0.0.0.0 --port=%d &", port)
}
func (h *PHPHandler) DefaultStopCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("kill $(lsof -i :%d -t) 2>/dev/null || true", port)
	}
	return "pkill -f 'php artisan serve' || true"
}
func (h *PHPHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *PHPHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("build/%s.tar.gz", moduleName)
}
func (h *PHPHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"app/", "config/", "public/", "routes/", "vendor/", "artisan", "composer.json"}
}

// RubyHandler handles Ruby applications
type RubyHandler struct{}

func (h *RubyHandler) Name() string { return "ruby" }
func (h *RubyHandler) DefaultBuildCmd(moduleName, env string) string {
	return "bundle install --deployment --without development test"
}
func (h *RubyHandler) DefaultStartCmd(baseDir string, port int) string {
	if port == 0 {
		port = 3000
	}
	return fmt.Sprintf("cd %s/current && bundle exec puma -C config/puma.rb -p %d -d", baseDir, port)
}
func (h *RubyHandler) DefaultStopCmd(baseDir string, port int) string {
	return fmt.Sprintf("kill $(cat %s/current/tmp/pids/server.pid 2>/dev/null) 2>/dev/null || kill $(lsof -i :%d -t) 2>/dev/null || true", baseDir, port)
}
func (h *RubyHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *RubyHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("build/%s.tar.gz", moduleName)
}
func (h *RubyHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"app/", "config/", "lib/", "public/", "vendor/", "Gemfile", "Gemfile.lock", "Rakefile", "config.ru"}
}

// DotNetHandler handles C# / .NET applications
type DotNetHandler struct{}

func (h *DotNetHandler) Name() string { return "dotnet" }
func (h *DotNetHandler) DefaultBuildCmd(moduleName, env string) string {
	return "dotnet publish -c Release -o build/publish"
}
func (h *DotNetHandler) DefaultStartCmd(baseDir string, port int) string {
	if port == 0 {
		port = 5000
	}
	return fmt.Sprintf("cd %s/current && dotnet *.dll --urls http://0.0.0.0:%d &", baseDir, port)
}
func (h *DotNetHandler) DefaultStopCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("kill $(lsof -i :%d -t) 2>/dev/null || true", port)
	}
	return "pkill -f 'dotnet.*dll' || true"
}
func (h *DotNetHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *DotNetHandler) DefaultArtifactPath(moduleName string) string {
	return "build/publish"
}
func (h *DotNetHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"build/publish/"}
}

// KotlinHandler handles Kotlin applications (non-Spring Boot)
type KotlinHandler struct{}

func (h *KotlinHandler) Name() string { return "kotlin" }
func (h *KotlinHandler) DefaultBuildCmd(moduleName, env string) string {
	return fmt.Sprintf("./gradlew :%s:build", moduleName)
}
func (h *KotlinHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/start.sh", baseDir)
}
func (h *KotlinHandler) DefaultStopCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/stop.sh", baseDir)
}
func (h *KotlinHandler) DefaultStatusCmd(baseDir string, port int) string {
	return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
}
func (h *KotlinHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("%s/build/libs/%s-all.jar", moduleName, moduleName)
}
func (h *KotlinHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"bin/", "config/", "lib/"}
}

// ElixirHandler handles Elixir applications
type ElixirHandler struct{}

func (h *ElixirHandler) Name() string { return "elixir" }
func (h *ElixirHandler) DefaultBuildCmd(moduleName, env string) string {
	return "MIX_ENV=prod mix do deps.get, compile, release"
}
func (h *ElixirHandler) DefaultStartCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/%s daemon", baseDir, "server")
}
func (h *ElixirHandler) DefaultStopCmd(baseDir string, port int) string {
	return fmt.Sprintf("%s/current/bin/%s stop", baseDir, "server")
}
func (h *ElixirHandler) DefaultStatusCmd(baseDir string, port int) string {
	if port > 0 {
		return fmt.Sprintf("lsof -i :%d -t 2>/dev/null", port)
	}
	return fmt.Sprintf("%s/current/bin/server pid", ".")
}
func (h *ElixirHandler) DefaultArtifactPath(moduleName string) string {
	return fmt.Sprintf("_build/prod/rel/%s", moduleName)
}
func (h *ElixirHandler) PackageContents(moduleName, baseDir string) []string {
	return []string{"bin/", "lib/", "releases/", "erts-*/"}
}

// Kafka, Redis, Elasticsearch, MongoDB, etc. are loaded as YAML plugins
// from the plugins/ directory. See plugins/README.md for details.
