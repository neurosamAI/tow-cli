package deploy

import (
	"fmt"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/module"
)

// ProvisionOptions configures what to provision on a server
type ProvisionOptions struct {
	Timezone     string // e.g., "Asia/Seoul"
	Locale       string // e.g., "en_US.UTF-8"
	InstallJRE   bool
	InstallTools bool // lsof, nc, curl, etc.
}

// Provision sets up a new server with basic requirements
func (d *Deployer) Provision(envName, moduleName string, serverNum int, opts ProvisionOptions) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	for _, srv := range servers {
		logger.Header("Provisioning %s (server-%d)", srv.Host, srv.Number)

		// Step 1: Set timezone
		if opts.Timezone != "" {
			logger.ServerAction(srv.Host, "Setting timezone to %s", opts.Timezone)
			cmd := fmt.Sprintf("sudo timedatectl set-timezone %s 2>/dev/null || sudo ln -sf /usr/share/zoneinfo/%s /etc/localtime", opts.Timezone, opts.Timezone)
			if _, err := d.ssh.Exec(env, srv.Host, cmd); err != nil {
				logger.Warn("[%s] Failed to set timezone: %v", srv.Host, err)
			}
		}

		// Step 2: Set locale
		if opts.Locale != "" {
			logger.ServerAction(srv.Host, "Setting locale to %s", opts.Locale)
			cmd := fmt.Sprintf(`sudo localectl set-locale LANG=%s 2>/dev/null || echo "LANG=%s" | sudo tee /etc/locale.conf >/dev/null`, opts.Locale, opts.Locale)
			if _, err := d.ssh.Exec(env, srv.Host, cmd); err != nil {
				logger.Warn("[%s] Failed to set locale: %v", srv.Host, err)
			}
		}

		// Step 3: Install JRE if needed
		if opts.InstallJRE {
			logger.ServerAction(srv.Host, "Installing JRE")
			cmd := `
if command -v java &>/dev/null; then
    echo "JRE_ALREADY_INSTALLED: $(java -version 2>&1 | head -1)"
else
    if command -v apt-get &>/dev/null; then
        sudo apt-get update -qq && sudo apt-get install -y -qq default-jre
    elif command -v yum &>/dev/null; then
        sudo yum install -y java-17-amazon-corretto-headless 2>/dev/null || sudo yum install -y java-17-openjdk-headless
    elif command -v dnf &>/dev/null; then
        sudo dnf install -y java-17-openjdk-headless
    else
        echo "ERROR: No package manager found"
        exit 1
    fi
    echo "JRE_INSTALLED: $(java -version 2>&1 | head -1)"
fi
`
			result, err := d.ssh.Exec(env, srv.Host, cmd)
			if err != nil {
				logger.Warn("[%s] Failed to install JRE: %v", srv.Host, err)
			} else {
				for _, line := range strings.Split(result.Stdout, "\n") {
					if strings.HasPrefix(line, "JRE_") {
						logger.Success("[%s] %s", srv.Host, line)
					}
				}
			}
		}

		// Step 4: Install essential tools
		if opts.InstallTools {
			logger.ServerAction(srv.Host, "Installing essential tools")
			cmd := `
if command -v apt-get &>/dev/null; then
    sudo apt-get install -y -qq lsof nc curl tar gzip
elif command -v yum &>/dev/null; then
    sudo yum install -y lsof nc curl tar gzip
fi
echo "TOOLS_OK"
`
			d.ssh.Exec(env, srv.Host, cmd)
		}

		// Step 5: Module-specific initialization
		mod := d.cfg.Modules[moduleName]
		if mod != nil {
			d.provisionModule(env, srv, moduleName, mod)
		}

		// Step 6: Create base directory structure
		logger.ServerAction(srv.Host, "Creating base directories")
		if err := d.Init(envName, moduleName, srv.Number); err != nil {
			logger.Warn("[%s] Init failed: %v", srv.Host, err)
		}

		logger.Success("[%s] Provisioning complete", srv.Host)
	}

	return nil
}

// provisionModule handles module-type-specific server initialization
func (d *Deployer) provisionModule(env *config.Environment, srv config.Server, moduleName string, mod *config.Module) {
	baseDir := d.RemoteBaseDirForServer(moduleName, srv)

	// Check if there's a plugin definition with provision steps
	pluginDef := module.GetPluginDef(mod.Type)
	if pluginDef != nil && (len(pluginDef.Provision.Packages) > 0 || len(pluginDef.Provision.Directories) > 0 || len(pluginDef.Provision.Commands) > 0) {
		logger.ServerAction(srv.Host, "Running plugin provisioning for %s (%s)", moduleName, mod.Type)

		// Install packages
		if len(pluginDef.Provision.Packages) > 0 {
			pkgList := strings.Join(pluginDef.Provision.Packages, " ")
			cmd := fmt.Sprintf(`
if command -v apt-get &>/dev/null; then
    sudo apt-get install -y -qq %s
elif command -v yum &>/dev/null; then
    sudo yum install -y %s
elif command -v dnf &>/dev/null; then
    sudo dnf install -y %s
fi
`, pkgList, pkgList, pkgList)
			d.ssh.Exec(env, srv.Host, cmd)
		}

		// Create directories
		if len(pluginDef.Provision.Directories) > 0 {
			var dirs []string
			for _, dir := range pluginDef.Provision.Directories {
				expanded := strings.ReplaceAll(dir, "{{BASE_DIR}}", baseDir)
				expanded = strings.ReplaceAll(expanded, "{{MODULE}}", moduleName)
				dirs = append(dirs, expanded)
			}
			d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " ")))
		}

		// Run custom commands
		for _, cmd := range pluginDef.Provision.Commands {
			expanded := strings.ReplaceAll(cmd, "{{BASE_DIR}}", baseDir)
			expanded = strings.ReplaceAll(expanded, "{{MODULE}}", moduleName)
			expanded = strings.ReplaceAll(expanded, "{{PORT}}", fmt.Sprintf("%d", mod.Port))
			d.ssh.Exec(env, srv.Host, expanded)
		}

		logger.Success("[%s] Plugin provisioning complete for %s", srv.Host, mod.Type)
	} else {
		// Built-in handler provisioning
		switch mod.Type {
		case "kafka":
			logger.ServerAction(srv.Host, "Setting up Kafka directories")
			d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s/data/kafka-logs", baseDir))

		case "redis":
			logger.ServerAction(srv.Host, "Setting up Redis directories")
			d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s/data", baseDir))

		case "springboot", "java":
			logger.ServerAction(srv.Host, "Verifying Java for %s", moduleName)
			result, _ := d.ssh.Exec(env, srv.Host, "java -version 2>&1 | head -1")
			if result != nil && result.Stdout != "" {
				logger.Success("[%s] Java: %s", srv.Host, strings.TrimSpace(result.Stdout))
			} else {
				logger.Warn("[%s] Java not found — run with --jre flag to install", srv.Host)
			}
		}
	}

	// Create persistent data directories for all module types
	if len(mod.DataDirs) > 0 {
		var dirs []string
		for _, dir := range mod.DataDirs {
			dirs = append(dirs, fmt.Sprintf("%s/%s", baseDir, dir))
		}
		d.ssh.Exec(env, srv.Host, fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " ")))
	}
}
