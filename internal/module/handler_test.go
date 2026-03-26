package module

import (
	"fmt"
	"strings"
	"testing"
)

// TestAllHandlersRegistered verifies that all built-in handlers are registered on init
func TestAllHandlersRegistered(t *testing.T) {
	expected := []string{
		"java", "springboot", "node", "python", "go", "rust",
		"php", "ruby", "dotnet", "kotlin", "elixir", "generic",
	}

	for _, name := range expected {
		h, err := Get(name)
		if err != nil {
			t.Errorf("handler %q not registered: %v", name, err)
			continue
		}
		if h.Name() != name {
			t.Errorf("handler %q returned Name() = %q", name, h.Name())
		}
	}
}

func TestGetUnknownHandler(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown handler")
	}
	if !strings.Contains(err.Error(), "unknown module type") {
		t.Errorf("expected 'unknown module type' error, got: %v", err)
	}
}

func TestAvailable(t *testing.T) {
	names := Available()
	if len(names) < 12 {
		t.Errorf("expected at least 12 handlers, got %d", len(names))
	}

	// All built-in should be present
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"java", "springboot", "node", "python", "go", "rust", "php", "ruby", "dotnet", "kotlin", "elixir", "generic"} {
		if !nameSet[expected] {
			t.Errorf("expected %q in Available(), not found", expected)
		}
	}
}

func TestRegisterDuplicate(t *testing.T) {
	// Register should overwrite existing handler without panic
	Register(&GenericHandler{})
	h, err := Get("generic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Name() != "generic" {
		t.Error("handler name mismatch after re-register")
	}
}

// --- Go Handler ---

func TestGoHandler(t *testing.T) {
	h := &GoHandler{}

	if h.Name() != "go" {
		t.Errorf("expected 'go', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("myapp", "prod")
	if !strings.Contains(build, "go build") {
		t.Errorf("expected 'go build' in build cmd, got %q", build)
	}
	if !strings.Contains(build, "myapp") {
		t.Errorf("expected module name in build cmd, got %q", build)
	}

	start := h.DefaultStartCmd("/app/myapp", 8080)
	if !strings.Contains(start, "/app/myapp") {
		t.Errorf("expected base dir in start cmd, got %q", start)
	}

	// StopCmd with port
	stop := h.DefaultStopCmd("/app/myapp", 8080)
	if !strings.Contains(stop, "8080") {
		t.Errorf("expected port in stop cmd, got %q", stop)
	}

	// StopCmd without port
	stopNoPort := h.DefaultStopCmd("/app/myapp", 0)
	if !strings.Contains(stopNoPort, "stop.sh") {
		t.Errorf("expected stop.sh fallback, got %q", stopNoPort)
	}

	status := h.DefaultStatusCmd("/app/myapp", 8080)
	if !strings.Contains(status, "8080") {
		t.Errorf("expected port in status cmd, got %q", status)
	}

	artifact := h.DefaultArtifactPath("myapp")
	if artifact != "bin/myapp" {
		t.Errorf("expected 'bin/myapp', got %q", artifact)
	}

	contents := h.PackageContents("myapp", "/base")
	if len(contents) != 2 {
		t.Errorf("expected 2 package contents, got %d", len(contents))
	}
}

// --- Rust Handler ---

func TestRustHandler(t *testing.T) {
	h := &RustHandler{}

	if h.Name() != "rust" {
		t.Errorf("expected 'rust', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("myapp", "prod")
	if !strings.Contains(build, "cargo build --release") {
		t.Errorf("expected 'cargo build --release', got %q", build)
	}

	start := h.DefaultStartCmd("/app/svc", 8080)
	if !strings.Contains(start, "/app/svc") {
		t.Errorf("expected base dir in start cmd, got %q", start)
	}

	// StopCmd with port
	stop := h.DefaultStopCmd("/app/svc", 3000)
	if !strings.Contains(stop, "3000") {
		t.Errorf("expected port in stop cmd, got %q", stop)
	}

	// StopCmd without port
	stopNoPort := h.DefaultStopCmd("/app/svc", 0)
	if !strings.Contains(stopNoPort, "stop.sh") {
		t.Errorf("expected stop.sh fallback, got %q", stopNoPort)
	}

	artifact := h.DefaultArtifactPath("myapp")
	if artifact != "target/release/myapp" {
		t.Errorf("expected 'target/release/myapp', got %q", artifact)
	}

	contents := h.PackageContents("myapp", "/base")
	if len(contents) != 2 {
		t.Errorf("expected 2 package contents, got %d", len(contents))
	}
}

// --- Java Handler ---

func TestJavaHandler(t *testing.T) {
	h := &JavaHandler{}

	if h.Name() != "java" {
		t.Errorf("expected 'java', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("api", "prod")
	if !strings.Contains(build, "gradlew") {
		t.Errorf("expected gradlew in build cmd, got %q", build)
	}
	if !strings.Contains(build, ":api:") {
		t.Errorf("expected module name in build cmd, got %q", build)
	}
	if !strings.Contains(build, "prod") {
		t.Errorf("expected env in build cmd, got %q", build)
	}

	start := h.DefaultStartCmd("/app/api", 8080)
	if !strings.Contains(start, "start.sh") {
		t.Errorf("expected start.sh, got %q", start)
	}

	stop := h.DefaultStopCmd("/app/api", 8080)
	if !strings.Contains(stop, "stop.sh") {
		t.Errorf("expected stop.sh, got %q", stop)
	}

	status := h.DefaultStatusCmd("/app/api", 8080)
	if !strings.Contains(status, "8080") {
		t.Errorf("expected port in status cmd, got %q", status)
	}

	artifact := h.DefaultArtifactPath("api")
	if !strings.Contains(artifact, "api/build/libs/api.jar") {
		t.Errorf("expected jar artifact, got %q", artifact)
	}

	contents := h.PackageContents("api", "/base")
	if len(contents) != 3 {
		t.Errorf("expected 3 package contents, got %d: %v", len(contents), contents)
	}
}

// --- SpringBoot Handler ---

func TestSpringBootHandler(t *testing.T) {
	h := &SpringBootHandler{}

	if h.Name() != "springboot" {
		t.Errorf("expected 'springboot', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("api", "prod")
	if !strings.Contains(build, "bootJar") {
		t.Errorf("expected bootJar in build cmd, got %q", build)
	}

	// SpringBoot inherits from Java for start/stop/status/artifact/package
	start := h.DefaultStartCmd("/app/api", 8080)
	if !strings.Contains(start, "start.sh") {
		t.Errorf("expected start.sh (inherited from Java), got %q", start)
	}

	artifact := h.DefaultArtifactPath("api")
	if !strings.Contains(artifact, "api.jar") {
		t.Errorf("expected jar artifact, got %q", artifact)
	}
}

// --- Node Handler ---

func TestNodeHandler(t *testing.T) {
	h := &NodeHandler{}

	if h.Name() != "node" {
		t.Errorf("expected 'node', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("frontend", "prod")
	if !strings.Contains(build, "npm") {
		t.Errorf("expected npm in build cmd, got %q", build)
	}

	start := h.DefaultStartCmd("/app/frontend", 3000)
	if !strings.Contains(start, "node") {
		t.Errorf("expected node in start cmd, got %q", start)
	}

	// StopCmd with port
	stop := h.DefaultStopCmd("/app/frontend", 3000)
	if !strings.Contains(stop, "3000") {
		t.Errorf("expected port in stop cmd, got %q", stop)
	}

	// StopCmd without port
	stopNoPort := h.DefaultStopCmd("/app/frontend", 0)
	if !strings.Contains(stopNoPort, "pkill") {
		t.Errorf("expected pkill fallback, got %q", stopNoPort)
	}

	status := h.DefaultStatusCmd("/app/frontend", 3000)
	if !strings.Contains(status, "3000") {
		t.Errorf("expected port in status cmd, got %q", status)
	}

	artifact := h.DefaultArtifactPath("frontend")
	if !strings.Contains(artifact, "frontend.tar.gz") {
		t.Errorf("expected tar.gz artifact, got %q", artifact)
	}

	contents := h.PackageContents("frontend", "/base")
	if len(contents) != 3 {
		t.Errorf("expected 3 package contents, got %d", len(contents))
	}
}

// --- Python Handler ---

func TestPythonHandler(t *testing.T) {
	h := &PythonHandler{}

	if h.Name() != "python" {
		t.Errorf("expected 'python', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("api", "prod")
	if !strings.Contains(build, "pip") {
		t.Errorf("expected pip in build cmd, got %q", build)
	}

	start := h.DefaultStartCmd("/app/api", 8000)
	if !strings.Contains(start, "python") {
		t.Errorf("expected python in start cmd, got %q", start)
	}

	// StopCmd with port
	stop := h.DefaultStopCmd("/app/api", 8000)
	if !strings.Contains(stop, "8000") {
		t.Errorf("expected port in stop cmd, got %q", stop)
	}

	// StopCmd without port
	stopNoPort := h.DefaultStopCmd("/app/api", 0)
	if !strings.Contains(stopNoPort, "pkill") {
		t.Errorf("expected pkill fallback, got %q", stopNoPort)
	}

	artifact := h.DefaultArtifactPath("api")
	if !strings.Contains(artifact, "api.tar.gz") {
		t.Errorf("expected tar.gz artifact, got %q", artifact)
	}

	contents := h.PackageContents("api", "/base")
	if len(contents) != 2 {
		t.Errorf("expected 2 package contents, got %d", len(contents))
	}
}

// --- PHP Handler ---

func TestPHPHandler(t *testing.T) {
	h := &PHPHandler{}

	if h.Name() != "php" {
		t.Errorf("expected 'php', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("app", "prod")
	if !strings.Contains(build, "composer") {
		t.Errorf("expected composer in build cmd, got %q", build)
	}

	// StartCmd with port
	start := h.DefaultStartCmd("/app/web", 9000)
	if !strings.Contains(start, "9000") {
		t.Errorf("expected port 9000, got %q", start)
	}
	if !strings.Contains(start, "artisan") {
		t.Errorf("expected artisan in start cmd, got %q", start)
	}

	// StartCmd with zero port (should default to 8000)
	startDefault := h.DefaultStartCmd("/app/web", 0)
	if !strings.Contains(startDefault, "8000") {
		t.Errorf("expected default port 8000, got %q", startDefault)
	}

	// StopCmd with port
	stop := h.DefaultStopCmd("/app/web", 9000)
	if !strings.Contains(stop, "9000") {
		t.Errorf("expected port in stop cmd, got %q", stop)
	}

	// StopCmd without port
	stopNoPort := h.DefaultStopCmd("/app/web", 0)
	if !strings.Contains(stopNoPort, "pkill") {
		t.Errorf("expected pkill fallback, got %q", stopNoPort)
	}

	artifact := h.DefaultArtifactPath("app")
	if !strings.Contains(artifact, "app.tar.gz") {
		t.Errorf("expected tar.gz artifact, got %q", artifact)
	}

	contents := h.PackageContents("app", "/base")
	if len(contents) < 5 {
		t.Errorf("expected at least 5 package contents for PHP, got %d: %v", len(contents), contents)
	}
}

// --- Ruby Handler ---

func TestRubyHandler(t *testing.T) {
	h := &RubyHandler{}

	if h.Name() != "ruby" {
		t.Errorf("expected 'ruby', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("app", "prod")
	if !strings.Contains(build, "bundle") {
		t.Errorf("expected bundle in build cmd, got %q", build)
	}

	// StartCmd with port
	start := h.DefaultStartCmd("/app/web", 4000)
	if !strings.Contains(start, "puma") {
		t.Errorf("expected puma in start cmd, got %q", start)
	}
	if !strings.Contains(start, "4000") {
		t.Errorf("expected port 4000, got %q", start)
	}

	// StartCmd with zero port (should default to 3000)
	startDefault := h.DefaultStartCmd("/app/web", 0)
	if !strings.Contains(startDefault, "3000") {
		t.Errorf("expected default port 3000, got %q", startDefault)
	}

	stop := h.DefaultStopCmd("/app/web", 3000)
	if !strings.Contains(stop, "3000") {
		t.Errorf("expected port in stop cmd, got %q", stop)
	}

	artifact := h.DefaultArtifactPath("app")
	if !strings.Contains(artifact, "app.tar.gz") {
		t.Errorf("expected tar.gz artifact, got %q", artifact)
	}

	contents := h.PackageContents("app", "/base")
	if len(contents) < 5 {
		t.Errorf("expected at least 5 package contents for Ruby, got %d", len(contents))
	}
}

// --- DotNet Handler ---

func TestDotNetHandler(t *testing.T) {
	h := &DotNetHandler{}

	if h.Name() != "dotnet" {
		t.Errorf("expected 'dotnet', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("api", "Release")
	if !strings.Contains(build, "dotnet publish") {
		t.Errorf("expected 'dotnet publish' in build cmd, got %q", build)
	}

	// StartCmd with port
	start := h.DefaultStartCmd("/app/api", 5000)
	if !strings.Contains(start, "dotnet") {
		t.Errorf("expected dotnet in start cmd, got %q", start)
	}
	if !strings.Contains(start, "5000") {
		t.Errorf("expected port 5000, got %q", start)
	}

	// StartCmd with zero port (should default to 5000)
	startDefault := h.DefaultStartCmd("/app/api", 0)
	if !strings.Contains(startDefault, "5000") {
		t.Errorf("expected default port 5000, got %q", startDefault)
	}

	// StopCmd with port
	stop := h.DefaultStopCmd("/app/api", 5000)
	if !strings.Contains(stop, "5000") {
		t.Errorf("expected port in stop cmd, got %q", stop)
	}

	// StopCmd without port
	stopNoPort := h.DefaultStopCmd("/app/api", 0)
	if !strings.Contains(stopNoPort, "pkill") {
		t.Errorf("expected pkill fallback, got %q", stopNoPort)
	}

	artifact := h.DefaultArtifactPath("api")
	if artifact != "build/publish" {
		t.Errorf("expected 'build/publish', got %q", artifact)
	}

	contents := h.PackageContents("api", "/base")
	if len(contents) != 1 || contents[0] != "build/publish/" {
		t.Errorf("expected ['build/publish/'], got %v", contents)
	}
}

// --- Kotlin Handler ---

func TestKotlinHandler(t *testing.T) {
	h := &KotlinHandler{}

	if h.Name() != "kotlin" {
		t.Errorf("expected 'kotlin', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("api", "prod")
	if !strings.Contains(build, "gradlew") {
		t.Errorf("expected gradlew in build cmd, got %q", build)
	}
	if !strings.Contains(build, ":api:build") {
		t.Errorf("expected module name in build cmd, got %q", build)
	}

	start := h.DefaultStartCmd("/app/api", 8080)
	if !strings.Contains(start, "start.sh") {
		t.Errorf("expected start.sh, got %q", start)
	}

	stop := h.DefaultStopCmd("/app/api", 8080)
	if !strings.Contains(stop, "stop.sh") {
		t.Errorf("expected stop.sh, got %q", stop)
	}

	status := h.DefaultStatusCmd("/app/api", 8080)
	if !strings.Contains(status, "8080") {
		t.Errorf("expected port in status cmd, got %q", status)
	}

	artifact := h.DefaultArtifactPath("api")
	if !strings.Contains(artifact, "api-all.jar") {
		t.Errorf("expected all.jar artifact, got %q", artifact)
	}

	contents := h.PackageContents("api", "/base")
	if len(contents) != 3 {
		t.Errorf("expected 3 package contents, got %d", len(contents))
	}
}

// --- Elixir Handler ---

func TestElixirHandler(t *testing.T) {
	h := &ElixirHandler{}

	if h.Name() != "elixir" {
		t.Errorf("expected 'elixir', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("app", "prod")
	if !strings.Contains(build, "mix") {
		t.Errorf("expected mix in build cmd, got %q", build)
	}
	if !strings.Contains(build, "MIX_ENV=prod") {
		t.Errorf("expected MIX_ENV=prod, got %q", build)
	}

	start := h.DefaultStartCmd("/app/svc", 4000)
	if !strings.Contains(start, "daemon") {
		t.Errorf("expected daemon in start cmd, got %q", start)
	}

	stop := h.DefaultStopCmd("/app/svc", 4000)
	if !strings.Contains(stop, "stop") {
		t.Errorf("expected stop in stop cmd, got %q", stop)
	}

	// StatusCmd with port
	status := h.DefaultStatusCmd("/app/svc", 4000)
	if !strings.Contains(status, "4000") {
		t.Errorf("expected port in status cmd with port, got %q", status)
	}

	// StatusCmd without port
	statusNoPort := h.DefaultStatusCmd("/app/svc", 0)
	if !strings.Contains(statusNoPort, "pid") {
		t.Errorf("expected pid check fallback, got %q", statusNoPort)
	}

	artifact := h.DefaultArtifactPath("app")
	if !strings.Contains(artifact, "_build/prod/rel/app") {
		t.Errorf("expected elixir release path, got %q", artifact)
	}

	contents := h.PackageContents("app", "/base")
	if len(contents) < 3 {
		t.Errorf("expected at least 3 package contents, got %d", len(contents))
	}
}

// --- Generic Handler ---

func TestGenericHandler(t *testing.T) {
	h := &GenericHandler{}

	if h.Name() != "generic" {
		t.Errorf("expected 'generic', got %q", h.Name())
	}

	build := h.DefaultBuildCmd("svc", "prod")
	if build != "" {
		t.Errorf("expected empty build cmd, got %q", build)
	}

	start := h.DefaultStartCmd("/app/svc", 8080)
	if !strings.Contains(start, "start.sh") {
		t.Errorf("expected start.sh, got %q", start)
	}

	stop := h.DefaultStopCmd("/app/svc", 8080)
	if !strings.Contains(stop, "stop.sh") {
		t.Errorf("expected stop.sh, got %q", stop)
	}

	// StatusCmd with port
	status := h.DefaultStatusCmd("/app/svc", 8080)
	if !strings.Contains(status, "8080") {
		t.Errorf("expected port in status cmd, got %q", status)
	}

	// StatusCmd without port
	statusNoPort := h.DefaultStatusCmd("/app/svc", 0)
	if !strings.Contains(statusNoPort, "no status check") {
		t.Errorf("expected no status check message, got %q", statusNoPort)
	}

	artifact := h.DefaultArtifactPath("svc")
	if !strings.Contains(artifact, "svc.tar.gz") {
		t.Errorf("expected tar.gz artifact, got %q", artifact)
	}

	contents := h.PackageContents("svc", "/base")
	if len(contents) != 2 {
		t.Errorf("expected 2 package contents, got %d", len(contents))
	}
}

// --- Table-driven test for all handlers ---

func TestAllHandlersDefaultBuildCmd(t *testing.T) {
	tests := []struct {
		handler    Handler
		moduleName string
		env        string
		expectSub  string
	}{
		{&GoHandler{}, "myapp", "prod", "go build"},
		{&RustHandler{}, "myapp", "prod", "cargo build"},
		{&JavaHandler{}, "api", "staging", "gradlew"},
		{&SpringBootHandler{}, "api", "prod", "bootJar"},
		{&NodeHandler{}, "web", "dev", "npm"},
		{&PythonHandler{}, "svc", "prod", "pip"},
		{&PHPHandler{}, "app", "prod", "composer"},
		{&RubyHandler{}, "app", "prod", "bundle"},
		{&DotNetHandler{}, "api", "prod", "dotnet publish"},
		{&KotlinHandler{}, "svc", "prod", "gradlew"},
		{&ElixirHandler{}, "app", "prod", "mix"},
		{&GenericHandler{}, "svc", "prod", ""},
	}

	for _, tt := range tests {
		t.Run(tt.handler.Name(), func(t *testing.T) {
			result := tt.handler.DefaultBuildCmd(tt.moduleName, tt.env)
			if tt.expectSub == "" {
				if result != "" {
					t.Errorf("expected empty build cmd, got %q", result)
				}
			} else if !strings.Contains(result, tt.expectSub) {
				t.Errorf("expected %q to contain %q", result, tt.expectSub)
			}
		})
	}
}

func TestAllHandlersDefaultArtifactPath(t *testing.T) {
	tests := []struct {
		handler    Handler
		moduleName string
		expected   string
	}{
		{&GoHandler{}, "app", "bin/app"},
		{&RustHandler{}, "app", "target/release/app"},
		{&JavaHandler{}, "api", "api/build/libs/api.jar"},
		{&SpringBootHandler{}, "api", "api/build/libs/api.jar"},
		{&NodeHandler{}, "web", "build/web.tar.gz"},
		{&PythonHandler{}, "svc", "build/svc.tar.gz"},
		{&PHPHandler{}, "web", "build/web.tar.gz"},
		{&RubyHandler{}, "web", "build/web.tar.gz"},
		{&DotNetHandler{}, "api", "build/publish"},
		{&KotlinHandler{}, "api", "api/build/libs/api-all.jar"},
		{&ElixirHandler{}, "app", "_build/prod/rel/app"},
		{&GenericHandler{}, "svc", "build/svc.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.handler.Name(), func(t *testing.T) {
			result := tt.handler.DefaultArtifactPath(tt.moduleName)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAllHandlersStartStopReturnNonEmpty(t *testing.T) {
	handlers := []Handler{
		&GoHandler{}, &RustHandler{}, &JavaHandler{}, &SpringBootHandler{},
		&NodeHandler{}, &PythonHandler{}, &PHPHandler{}, &RubyHandler{},
		&DotNetHandler{}, &KotlinHandler{}, &ElixirHandler{}, &GenericHandler{},
	}

	for _, h := range handlers {
		t.Run(fmt.Sprintf("%s/start", h.Name()), func(t *testing.T) {
			cmd := h.DefaultStartCmd("/app/svc", 8080)
			if cmd == "" {
				t.Error("expected non-empty start cmd")
			}
		})
		t.Run(fmt.Sprintf("%s/stop", h.Name()), func(t *testing.T) {
			cmd := h.DefaultStopCmd("/app/svc", 8080)
			if cmd == "" {
				t.Error("expected non-empty stop cmd")
			}
		})
		t.Run(fmt.Sprintf("%s/status", h.Name()), func(t *testing.T) {
			cmd := h.DefaultStatusCmd("/app/svc", 8080)
			if cmd == "" {
				t.Error("expected non-empty status cmd")
			}
		})
	}
}

func TestAllHandlersPackageContentsNonEmpty(t *testing.T) {
	handlers := []Handler{
		&GoHandler{}, &RustHandler{}, &JavaHandler{}, &SpringBootHandler{},
		&NodeHandler{}, &PythonHandler{}, &PHPHandler{}, &RubyHandler{},
		&DotNetHandler{}, &KotlinHandler{}, &ElixirHandler{}, &GenericHandler{},
	}

	for _, h := range handlers {
		t.Run(h.Name(), func(t *testing.T) {
			contents := h.PackageContents("mod", "/base")
			if len(contents) == 0 {
				t.Error("expected non-empty package contents")
			}
		})
	}
}

// --- Plugin Handler additional tests ---

func TestPluginHandlerStatusCmdEmpty(t *testing.T) {
	h := &PluginHandler{
		Def: PluginDef{
			Name: "test",
		},
	}

	// No port, no status cmd
	status := h.DefaultStatusCmd("/app", 0)
	if status != "" {
		t.Errorf("expected empty, got %q", status)
	}

	// With port, no status cmd defined
	status = h.DefaultStatusCmd("/app", 9092)
	if !strings.Contains(status, "9092") {
		t.Errorf("expected lsof fallback with port, got %q", status)
	}
}

func TestPluginHandlerSubstitution(t *testing.T) {
	h := &PluginHandler{
		Def: PluginDef{
			Name:     "kafka",
			StartCmd: "{{BASE_DIR}}/bin/kafka-server-start.sh --port={{PORT}} --env={{ENV}}",
			StopCmd:  "{{BASE_DIR}}/bin/kafka-server-stop.sh",
			Package: PackageInfo{
				DefaultVersion: "3.7.0",
			},
		},
	}

	start := h.DefaultStartCmd("/opt/kafka", 9092)
	if !strings.Contains(start, "/opt/kafka") {
		t.Errorf("expected base dir substituted, got %q", start)
	}
	if !strings.Contains(start, "9092") {
		t.Errorf("expected port substituted, got %q", start)
	}

	stop := h.DefaultStopCmd("/opt/kafka", 9092)
	if !strings.Contains(stop, "/opt/kafka") {
		t.Errorf("expected base dir substituted, got %q", stop)
	}
}

func TestPluginHandlerPackageContentsDefault(t *testing.T) {
	h := &PluginHandler{
		Def: PluginDef{
			Name: "minimal",
		},
	}

	contents := h.PackageContents("mod", "/base")
	if len(contents) != 2 || contents[0] != "bin/" || contents[1] != "config/" {
		t.Errorf("expected default [bin/, config/], got %v", contents)
	}
}

func TestPluginHandlerPackageContentsCustom(t *testing.T) {
	h := &PluginHandler{
		Def: PluginDef{
			Name:            "custom",
			PackageIncludes: []string{"data/", "scripts/", "lib/"},
		},
	}

	contents := h.PackageContents("mod", "/base")
	if len(contents) != 3 {
		t.Errorf("expected 3 custom contents, got %d", len(contents))
	}
}
