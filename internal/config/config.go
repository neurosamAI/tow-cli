package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/module"
	"gopkg.in/yaml.v3"
)

// Config is the root configuration for Tow
type Config struct {
	Project       ProjectConfig           `yaml:"project"`
	Environments  map[string]*Environment `yaml:"environments"`
	Modules       map[string]*Module      `yaml:"modules"`
	Defaults      Defaults                `yaml:"defaults"`
	Notifications []NotificationConfig    `yaml:"notifications"` // deployment notifications
	Retention     RetentionConfig         `yaml:"retention"`     // deployment history retention
}

// NotificationConfig defines a notification target
type NotificationConfig struct {
	Type   string `yaml:"type"`    // "webhook", "slack", "discord", "telegram"
	URL    string `yaml:"url"`     // webhook URL (for telegram: bot token)
	ChatID string `yaml:"chat_id"` // telegram chat ID
}

// RetentionConfig defines deployment history retention policy
type RetentionConfig struct {
	Keep        int  `yaml:"keep"`         // number of deployments to keep (default: 5)
	AutoCleanup bool `yaml:"auto_cleanup"` // automatically cleanup after deploy (default: false)
}

// ProjectConfig holds project-level metadata
type ProjectConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	BaseDir string `yaml:"base_dir"` // base directory on remote servers (e.g., /app)
}

// Environment defines a deployment target environment
type Environment struct {
	Servers      []Server          `yaml:"servers"`
	SSHUser      string            `yaml:"ssh_user"`
	SSHKeyPath   string            `yaml:"ssh_key_path"`
	SSHPort      int               `yaml:"ssh_port"`
	Variables    map[string]string `yaml:"variables"`     // env-level variables
	Branch       string            `yaml:"branch"`        // required git branch (simple mode, empty = any)
	BranchPolicy *BranchPolicy     `yaml:"branch_policy"` // advanced branch verification
}

// BranchPolicy defines advanced branch verification rules
type BranchPolicy struct {
	// Allowed branches (exact match or glob pattern)
	// Examples: ["main"], ["main", "release/*"], ["develop", "feature/*"]
	Allowed []string `yaml:"allowed"`
	// Commands that require branch verification (empty = all mutating commands)
	Commands []string `yaml:"commands"`
	// Skip branch check entirely
	Skip bool `yaml:"skip"`
}

// Server represents a single target server
type Server struct {
	Name    string            `yaml:"name"`    // unique server name (e.g., "kafka-1", "api-primary")
	Number  int               `yaml:"number"`  // auto-assigned if not set (for backward compatibility)
	Host    string            `yaml:"host"`    // single host
	Hosts   []string          `yaml:"hosts"`   // multiple hosts (shorthand — auto-expands to individual servers)
	Labels  map[string]string `yaml:"labels"`  // for filtering
	Modules []string          `yaml:"modules"` // which modules run on this server (empty = all)
}

// ID returns the display identifier for the server (name if set, otherwise number)
func (s Server) ID() string {
	if s.Name != "" {
		return s.Name
	}
	return fmt.Sprintf("%d", s.Number)
}

// SSHConfig defines per-module SSH connection settings
type SSHConfig struct {
	User     string `yaml:"user"`
	Port     int    `yaml:"port"`
	Auth     string `yaml:"auth"` // "key" (default), "password", "agent"
	KeyPath  string `yaml:"key_path"`
	Password string `yaml:"password"` // supports ${ENV_VAR}
}

// Module defines a deployable service/application
type Module struct {
	Type            string            `yaml:"type"`             // java, kafka, redis, node, python, generic
	Version         string            `yaml:"version"`          // package version (e.g., "3.7.0") — required for plugin types
	Port            int               `yaml:"port"`             // service port
	SSH             *SSHConfig        `yaml:"ssh"`              // per-module SSH config (optional)
	BuildCmd        string            `yaml:"build_cmd"`        // build command (e.g., "./gradlew :module:bootJar")
	ArtifactPath    string            `yaml:"artifact_path"`    // path to build artifact
	PackageIncludes []string          `yaml:"package_includes"` // additional files/dirs to package
	HealthCheck     HealthCheckConfig `yaml:"health_check"`
	StartCmd        string            `yaml:"start_cmd"`  // remote start command
	StopCmd         string            `yaml:"stop_cmd"`   // remote stop command
	StatusCmd       string            `yaml:"status_cmd"` // remote status command
	LogPath         string            `yaml:"log_path"`   // remote log file path
	DeployDir       string            `yaml:"deploy_dir"` // deployment directory on server
	ConfigDir       string            `yaml:"config_dir"` // local config directory
	DataDirs        []string          `yaml:"data_dirs"`  // persistent data directories
	Variables       map[string]string `yaml:"variables"`  // module-level variables
	Hooks           HooksConfig       `yaml:"hooks"`      // lifecycle hooks
}

// HealthCheckConfig defines how to verify a service is healthy after deployment
type HealthCheckConfig struct {
	Type     string `yaml:"type"`     // http, tcp, log, command
	Target   string `yaml:"target"`   // URL, port, log pattern, or command
	Timeout  int    `yaml:"timeout"`  // seconds to wait
	Interval int    `yaml:"interval"` // seconds between checks
	Retries  int    `yaml:"retries"`  // max retry count
}

// HooksConfig defines lifecycle hooks
type HooksConfig struct {
	PreBuild   string `yaml:"pre_build"`
	PostBuild  string `yaml:"post_build"`
	PreDeploy  string `yaml:"pre_deploy"`
	PostDeploy string `yaml:"post_deploy"`
	PreStart   string `yaml:"pre_start"`
	PostStart  string `yaml:"post_start"`
	PreStop    string `yaml:"pre_stop"`
	PostStop   string `yaml:"post_stop"`
}

// Defaults provides default values for environments and modules
type Defaults struct {
	SSHUser     string            `yaml:"ssh_user"`
	SSHPort     int               `yaml:"ssh_port"`
	SSHKeyPath  string            `yaml:"ssh_key_path"`
	DeployDir   string            `yaml:"deploy_dir"`
	DeployPath  string            `yaml:"deploy_path"` // remote dir pattern: "{module}" (default) or "{module}-{server}" (legacy)
	LogDir      string            `yaml:"log_dir"`     // log subdirectory name: "log" (legacy) or "logs" (default)
	LogFile     string            `yaml:"log_file"`    // default log filename: "std.log" (legacy) or "{module}.log"
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// Load reads and parses the Tow configuration file
func Load(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving config path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", absPath, err)
	}

	// Expand only SET environment variables; leave unset ${VAR} as-is
	// This prevents build_cmd's ${ENV}, ${MODULE} from being wiped by os.ExpandEnv
	rawStr := string(data)
	expanded := os.Expand(rawStr, func(key string) string {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return "${" + key + "}" // leave unset vars as literal
	})

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Merge tow.local.yaml overrides if the file exists
	if err := mergeLocal(cfg, absPath); err != nil {
		return nil, fmt.Errorf("merging local config: %w", err)
	}

	// Load community plugins before applying defaults (so handlers are available)
	module.LoadPlugins(module.PluginDirs()...)

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// applyDefaults fills in missing values from the defaults section
func (c *Config) applyDefaults() {
	if c.Defaults.SSHPort == 0 {
		c.Defaults.SSHPort = 22
	}
	if c.Defaults.SSHUser == "" {
		c.Defaults.SSHUser = "ec2-user"
	}
	if c.Defaults.DeployDir == "" {
		c.Defaults.DeployDir = "deploy"
	}
	if c.Defaults.HealthCheck.Timeout == 0 {
		c.Defaults.HealthCheck.Timeout = 300
	}
	if c.Defaults.HealthCheck.Interval == 0 {
		c.Defaults.HealthCheck.Interval = 5
	}
	if c.Defaults.HealthCheck.Retries == 0 {
		c.Defaults.HealthCheck.Retries = 60
	}

	if c.Retention.Keep == 0 {
		c.Retention.Keep = 5
	}

	for _, env := range c.Environments {
		if env.SSHUser == "" {
			env.SSHUser = c.Defaults.SSHUser
		}
		if env.SSHPort == 0 {
			env.SSHPort = c.Defaults.SSHPort
		}
		// Expand hosts shorthand: a server with `hosts: [a, b, c]` becomes 3 servers
		var expanded []Server
		for _, srv := range env.Servers {
			if len(srv.Hosts) > 0 && srv.Host == "" {
				baseName := srv.Name
				for i, h := range srv.Hosts {
					s := Server{
						Host:    h,
						Labels:  srv.Labels,
						Modules: srv.Modules,
						Number:  i + 1,
					}
					if baseName != "" {
						s.Name = fmt.Sprintf("%s-%d", baseName, i+1)
					}
					expanded = append(expanded, s)
				}
			} else {
				expanded = append(expanded, srv)
			}
		}
		env.Servers = expanded

		// Auto-assign server numbers if not set
		for i := range env.Servers {
			if env.Servers[i].Number == 0 {
				env.Servers[i].Number = i + 1
			}
		}
		if env.SSHKeyPath == "" {
			env.SSHKeyPath = c.Defaults.SSHKeyPath
		}
	}

	baseDir := c.Project.BaseDir
	if baseDir == "" {
		baseDir = "/app"
	}

	for name, mod := range c.Modules {
		if mod.DeployDir == "" {
			mod.DeployDir = c.Defaults.DeployDir
		}
		if mod.HealthCheck.Timeout == 0 {
			mod.HealthCheck = c.Defaults.HealthCheck
		}

		// Set default version from plugin if not specified by user
		if mod.Version == "" {
			if pluginDef := module.GetPluginDef(mod.Type); pluginDef != nil {
				mod.Version = pluginDef.Package.DefaultVersion
				fmt.Fprintf(os.Stderr, "  [warn] module %q: no version specified, using plugin default %q. Pin with: version: \"%s\"\n",
					name, mod.Version, mod.Version)
			}
		}

		// If user specified a version, update the plugin's default for handler resolution
		if mod.Version != "" {
			if pluginDef := module.GetPluginDef(mod.Type); pluginDef != nil {
				pluginDef.Package.DefaultVersion = mod.Version
			}
		}

		// Apply handler defaults for empty commands/paths
		handler, err := module.Get(mod.Type)
		if err == nil {
			modBaseDir := filepath.Join(baseDir, name)
			if mod.BuildCmd == "" {
				mod.BuildCmd = handler.DefaultBuildCmd(name, "")
			}
			if mod.StartCmd == "" {
				mod.StartCmd = handler.DefaultStartCmd(modBaseDir, mod.Port)
			}
			if mod.StopCmd == "" {
				mod.StopCmd = handler.DefaultStopCmd(modBaseDir, mod.Port)
			}
			if mod.ArtifactPath == "" {
				mod.ArtifactPath = handler.DefaultArtifactPath(name)
			}
		}
	}
}

// Validate checks the configuration for required fields and consistency
func (c *Config) Validate() error {
	if c.Project.Name == "" {
		return fmt.Errorf("project.name is required")
	}
	if len(c.Environments) == 0 {
		return fmt.Errorf("at least one environment is required")
	}
	if len(c.Modules) == 0 {
		return fmt.Errorf("at least one module is required")
	}

	for name, env := range c.Environments {
		if len(env.Servers) == 0 {
			return fmt.Errorf("environment %q has no servers", name)
		}
		for i, srv := range env.Servers {
			if srv.Host == "" {
				return fmt.Errorf("environment %q server[%d] has no host", name, i)
			}
		}
	}

	for name, mod := range c.Modules {
		if mod.Type == "" {
			return fmt.Errorf("module %q has no type", name)
		}
		// SECURITY: Block plaintext passwords in config
		if mod.SSH != nil && mod.SSH.Auth == "password" && mod.SSH.Password != "" {
			if !strings.HasPrefix(mod.SSH.Password, "${") {
				return fmt.Errorf("module %q: plaintext passwords are not allowed in config. Use environment variable: password: ${MY_PASSWORD}", name)
			}
		}
	}

	// Validate no empty hosts from unresolved env vars
	for envName, env := range c.Environments {
		for i, srv := range env.Servers {
			if srv.Host == "" || srv.Host == "${}" {
				return fmt.Errorf("environment %q server[%d]: host is empty (check that environment variables are set)", envName, i)
			}
		}
	}

	return nil
}

// GetServersForModule returns the servers in an environment that host a given module.
// serverFilter can be: 0 (all), a number (by index), or parsed from string name.
func (c *Config) GetServersForModule(envName, moduleName string, serverNum int) ([]Server, *Environment, error) {
	return c.GetServersForModuleByName(envName, moduleName, "", serverNum)
}

// GetServersForModuleByName filters by server name or number.
// If serverName is set, it takes priority over serverNum.
func (c *Config) GetServersForModuleByName(envName, moduleName, serverName string, serverNum int) ([]Server, *Environment, error) {
	env, ok := c.Environments[envName]
	if !ok {
		return nil, nil, fmt.Errorf("environment %q not found", envName)
	}

	if _, ok := c.Modules[moduleName]; !ok {
		return nil, nil, fmt.Errorf("module %q not found", moduleName)
	}

	var servers []Server
	for _, srv := range env.Servers {
		// Filter by server name if specified
		if serverName != "" && srv.Name != serverName {
			continue
		}
		// Filter by server number if specified (and no name filter)
		if serverName == "" && serverNum > 0 && srv.Number != serverNum {
			continue
		}
		// Filter by module assignment (empty = all modules)
		if len(srv.Modules) > 0 {
			found := false
			for _, m := range srv.Modules {
				if m == moduleName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		servers = append(servers, srv)
	}

	filter := serverName
	if filter == "" && serverNum > 0 {
		filter = fmt.Sprintf("%d", serverNum)
	}
	if len(servers) == 0 {
		return nil, nil, fmt.Errorf("no servers found for module %q in environment %q (server=%s)", moduleName, envName, filter)
	}

	return servers, env, nil
}

// GetConfigPath returns the config directory path for a module in an environment,
// supporting hierarchical override:
//
//	config/{env}-{serverName}/ > config/{env}-{serverNum}/ > config/{env}/ > config/
func (c *Config) GetConfigPath(moduleName, envName string, serverNum int) string {
	return c.GetConfigPathByName(moduleName, envName, "", serverNum)
}

// GetConfigPathByName resolves config path with server name support
func (c *Config) GetConfigPathByName(moduleName, envName, serverName string, serverNum int) string {
	mod := c.Modules[moduleName]
	if mod == nil || mod.ConfigDir == "" {
		return ""
	}

	// Try most specific first: config/{env}-{serverName}/
	if serverName != "" {
		specific := filepath.Join(mod.ConfigDir, fmt.Sprintf("%s-%s", envName, serverName))
		if info, err := os.Stat(specific); err == nil && info.IsDir() {
			return specific
		}
	}

	// Then: config/{env}-{serverNum}/
	if serverNum > 0 {
		specific := filepath.Join(mod.ConfigDir, fmt.Sprintf("%s-%d", envName, serverNum))
		if info, err := os.Stat(specific); err == nil && info.IsDir() {
			return specific
		}
	}

	// Then env-level: config/{env}/
	envPath := filepath.Join(mod.ConfigDir, envName)
	if info, err := os.Stat(envPath); err == nil && info.IsDir() {
		return envPath
	}

	// Fallback to base config
	return mod.ConfigDir
}

// ValidateDetailed performs a thorough validation of the configuration and
// returns a list of all warnings and errors found (does not stop at the first error).
func (c *Config) ValidateDetailed() []string {
	var issues []string

	// Check required project fields
	if c.Project.Name == "" {
		issues = append(issues, "project.name is required")
	}
	if c.Project.BaseDir == "" {
		issues = append(issues, "project.base_dir is not set")
	}

	// Check environments
	if len(c.Environments) == 0 {
		issues = append(issues, "at least one environment is required")
	}

	// Built-in types + dynamically loaded plugins
	validModuleTypes := map[string]bool{
		"java": true, "springboot": true, "node": true,
		"python": true, "go": true, "rust": true,
		"php": true, "ruby": true, "dotnet": true,
		"kotlin": true, "elixir": true,
		"generic": true,
	}
	// Add all registered handlers (including plugins)
	for _, name := range module.Available() {
		validModuleTypes[name] = true
	}

	for envName, env := range c.Environments {
		if len(env.Servers) == 0 {
			issues = append(issues, fmt.Sprintf("environment %q has no servers", envName))
		}
		for i, srv := range env.Servers {
			if srv.Host == "" {
				issues = append(issues, fmt.Sprintf("environment %q server[%d] has empty host", envName, i))
			}
		}
		// Check SSH key file exists
		if env.SSHKeyPath != "" {
			keyPath := env.SSHKeyPath
			if len(keyPath) > 0 && keyPath[0] == '~' {
				if home, err := os.UserHomeDir(); err == nil {
					keyPath = filepath.Join(home, keyPath[1:])
				}
			}
			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				issues = append(issues, fmt.Sprintf("environment %q SSH key file does not exist: %s", envName, env.SSHKeyPath))
			}
		}
	}

	// Check modules
	if len(c.Modules) == 0 {
		issues = append(issues, "at least one module is required")
	}

	for modName, mod := range c.Modules {
		if mod.Type == "" {
			issues = append(issues, fmt.Sprintf("module %q has no type", modName))
		} else if !validModuleTypes[mod.Type] {
			issues = append(issues, fmt.Sprintf("module %q has invalid type %q (valid: java, springboot, node, python, generic, kafka, redis)", modName, mod.Type))
		}

		// Warn if plugin-type module has no version pinned
		if mod.Version == "" {
			pluginDef := module.GetPluginDef(mod.Type)
			if pluginDef != nil {
				issues = append(issues, fmt.Sprintf("module %q: no version specified — using plugin default %q. Pin a version with 'version: \"%s\"' to avoid unexpected changes on plugin update",
					modName, pluginDef.Package.DefaultVersion, pluginDef.Package.DefaultVersion))
			}
		}

		// Check config directory exists if specified
		if mod.ConfigDir != "" {
			if info, err := os.Stat(mod.ConfigDir); os.IsNotExist(err) {
				issues = append(issues, fmt.Sprintf("module %q config_dir does not exist: %s", modName, mod.ConfigDir))
			} else if err == nil && !info.IsDir() {
				issues = append(issues, fmt.Sprintf("module %q config_dir is not a directory: %s", modName, mod.ConfigDir))
			}
		}
	}

	return issues
}

// Summary returns a summary of the configuration counts.
func (c *Config) Summary() (envCount, moduleCount, serverCount int) {
	envCount = len(c.Environments)
	moduleCount = len(c.Modules)
	for _, env := range c.Environments {
		serverCount += len(env.Servers)
	}
	return
}

// findUnsetEnvVars scans text for ${VAR} patterns and returns names of unset variables
func findUnsetEnvVars(text string) []string {
	var unset []string
	seen := map[string]bool{}

	for i := 0; i < len(text)-1; i++ {
		if text[i] == '$' && text[i+1] == '{' {
			end := strings.Index(text[i:], "}")
			if end < 0 {
				continue
			}
			varName := text[i+2 : i+end]
			if varName == "" || seen[varName] {
				continue
			}
			seen[varName] = true
			if os.Getenv(varName) == "" {
				unset = append(unset, varName)
			}
		}
	}
	return unset
}
