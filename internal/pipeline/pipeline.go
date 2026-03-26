package pipeline

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/logger"
	"github.com/neurosamAI/tow-cli/internal/module"
	"github.com/neurosamAI/tow-cli/internal/ssh"
)

// dryRun controls whether local commands are actually executed
var dryRun bool

// SetDryRun enables or disables dry-run mode for local commands
func SetDryRun(enabled bool) {
	dryRun = enabled
}

// Pipeline orchestrates multi-step deployment workflows
type Pipeline struct {
	cfg      *config.Config
	ssh      *ssh.Manager
	deployer *deploy.Deployer
	Rolling  bool // use rolling deployment strategy
}

// New creates a new Pipeline
func New(cfg *config.Config, sshMgr *ssh.Manager) *Pipeline {
	return &Pipeline{
		cfg:      cfg,
		ssh:      sshMgr,
		deployer: deploy.New(cfg, sshMgr),
	}
}

// Deploy executes: package → upload → install → stop → start
func (p *Pipeline) Deploy(envName, moduleName string, serverNum int) error {
	logger.Header("Deploy: %s → %s", moduleName, envName)

	// Notify start
	p.deployer.Notify(envName, moduleName, "deploy_start", "deployment started")

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Package", func() error { return p.Package(moduleName, envName) }},
		{"Upload", func() error { return p.deployer.Upload(envName, moduleName, serverNum, "") }},
		{"Install", func() error { return p.deployer.Install(envName, moduleName, serverNum) }},
		{"Stop", func() error { return p.deployer.Stop(envName, moduleName, serverNum) }},
		{"Start", func() error {
			if p.Rolling {
				return p.deployer.StartRolling(envName, moduleName, serverNum)
			}
			return p.deployer.Start(envName, moduleName, serverNum)
		}},
	}

	if err := p.runSteps(steps); err != nil {
		p.deployer.Notify(envName, moduleName, "deploy_failed", err.Error())
		return err
	}

	// Auto cleanup old deployments
	if p.cfg.Retention.AutoCleanup {
		logger.Info("Auto-cleaning old deployments (keeping %d)...", p.cfg.Retention.Keep)
		if err := p.deployer.Cleanup(envName, moduleName, serverNum, p.cfg.Retention.Keep); err != nil {
			logger.Warn("Auto-cleanup failed: %v", err)
		}
	}

	p.deployer.Notify(envName, moduleName, "deploy_success", "deployment completed")
	return nil
}

// Auto executes: build → package → upload → install → stop → start
func (p *Pipeline) Auto(envName, moduleName string, serverNum int) error {
	logger.Header("Auto: %s → %s (full pipeline)", moduleName, envName)

	p.deployer.Notify(envName, moduleName, "auto_start", "full pipeline started")

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Build", func() error { return p.Build(moduleName, envName) }},
		{"Package", func() error { return p.Package(moduleName, envName) }},
		{"Upload", func() error { return p.deployer.Upload(envName, moduleName, serverNum, "") }},
		{"Install", func() error { return p.deployer.Install(envName, moduleName, serverNum) }},
		{"Stop", func() error { return p.deployer.Stop(envName, moduleName, serverNum) }},
		{"Start", func() error {
			if p.Rolling {
				return p.deployer.StartRolling(envName, moduleName, serverNum)
			}
			return p.deployer.Start(envName, moduleName, serverNum)
		}},
	}

	if err := p.runSteps(steps); err != nil {
		p.deployer.Notify(envName, moduleName, "auto_failed", err.Error())
		return err
	}

	// Auto cleanup old deployments
	if p.cfg.Retention.AutoCleanup {
		logger.Info("Auto-cleaning old deployments (keeping %d)...", p.cfg.Retention.Keep)
		if err := p.deployer.Cleanup(envName, moduleName, serverNum, p.cfg.Retention.Keep); err != nil {
			logger.Warn("Auto-cleanup failed: %v", err)
		}
	}

	p.deployer.Notify(envName, moduleName, "auto_success", "full pipeline completed")
	return nil
}

// AutoWithRollback executes Auto pipeline and auto-rolls back on Start failure
func (p *Pipeline) AutoWithRollback(envName, moduleName string, serverNum int) error {
	logger.Header("Auto (with auto-rollback): %s → %s", moduleName, envName)

	p.deployer.Notify(envName, moduleName, "auto_start", "full pipeline started (auto-rollback enabled)")

	// Build and package locally first
	if err := p.Build(moduleName, envName); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	if err := p.Package(moduleName, envName); err != nil {
		return fmt.Errorf("package failed: %w", err)
	}

	// Remote steps — these can be rolled back
	if err := p.deployer.Upload(envName, moduleName, serverNum, ""); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	if err := p.deployer.Install(envName, moduleName, serverNum); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}
	if err := p.deployer.Stop(envName, moduleName, serverNum); err != nil {
		logger.Warn("Stop returned error (continuing): %v", err)
	}

	// Start — if this fails, auto-rollback
	var startErr error
	if p.Rolling {
		startErr = p.deployer.StartRolling(envName, moduleName, serverNum)
	} else {
		startErr = p.deployer.Start(envName, moduleName, serverNum)
	}

	if startErr != nil {
		logger.Error("Start failed: %v", startErr)
		logger.Header("Auto-rollback: reverting to previous deployment")
		p.deployer.Notify(envName, moduleName, "auto_rollback", fmt.Sprintf("start failed, rolling back: %v", startErr))

		if rbErr := p.deployer.Rollback(envName, moduleName, serverNum, ""); rbErr != nil {
			logger.Error("Auto-rollback also failed: %v", rbErr)
			p.deployer.Notify(envName, moduleName, "rollback_failed", rbErr.Error())
			return fmt.Errorf("start failed (%w) and auto-rollback also failed: %v", startErr, rbErr)
		}

		logger.Success("Auto-rollback completed successfully")
		p.deployer.Notify(envName, moduleName, "rollback_success", "reverted to previous deployment")
		return fmt.Errorf("deployment failed and was rolled back: %w", startErr)
	}

	// Auto cleanup
	if p.cfg.Retention.AutoCleanup {
		logger.Info("Auto-cleaning old deployments (keeping %d)...", p.cfg.Retention.Keep)
		if err := p.deployer.Cleanup(envName, moduleName, serverNum, p.cfg.Retention.Keep); err != nil {
			logger.Warn("Auto-cleanup failed: %v", err)
		}
	}

	p.deployer.Notify(envName, moduleName, "auto_success", "full pipeline completed")
	logger.Success("Pipeline completed successfully")
	return nil
}

// Build runs the build command for a module
func (p *Pipeline) Build(moduleName, envName string) error {
	mod, ok := p.cfg.Modules[moduleName]
	if !ok {
		return fmt.Errorf("module %q not found", moduleName)
	}

	if mod.BuildCmd == "" {
		logger.Info("No build command for %s, skipping", moduleName)
		return nil
	}

	// Execute pre-build hook
	if mod.Hooks.PreBuild != "" {
		logger.Debug("Running pre-build hook...")
		if err := runLocalCmd(mod.Hooks.PreBuild); err != nil {
			return fmt.Errorf("pre-build hook failed: %w", err)
		}
	}

	// Substitute variables in build command
	buildCmd := substituteVars(mod.BuildCmd, envName, moduleName, mod.Variables)

	logger.Info("Building %s: %s", moduleName, buildCmd)

	if err := runLocalCmd(buildCmd); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Execute post-build hook
	if mod.Hooks.PostBuild != "" {
		logger.Debug("Running post-build hook...")
		if err := runLocalCmd(mod.Hooks.PostBuild); err != nil {
			return fmt.Errorf("post-build hook failed: %w", err)
		}
	}

	logger.Success("Build complete for %s", moduleName)
	return nil
}

// Package creates the deployment artifact
func (p *Pipeline) Package(moduleName, envName string) error {
	mod, ok := p.cfg.Modules[moduleName]
	if !ok {
		return fmt.Errorf("module %q not found", moduleName)
	}

	logger.Info("Packaging %s for %s", moduleName, envName)

	artifactPath := mod.ArtifactPath
	if artifactPath == "" {
		artifactPath = fmt.Sprintf("build/%s.tar.gz", moduleName)
	}

	var includes []string

	// Get handler-specific package contents
	handler, err := module.Get(mod.Type)
	if err == nil {
		includes = append(includes, handler.PackageContents(moduleName, "")...)
	} else if mod.Type == "java" || mod.Type == "springboot" {
		includes = append(includes, fmt.Sprintf("build/libs/%s.jar", moduleName))
	}

	includes = append(includes, mod.PackageIncludes...)

	configPath := p.cfg.GetConfigPath(moduleName, envName, 0)
	if configPath != "" {
		includes = append(includes, configPath)
	}

	if len(includes) == 0 {
		logger.Warn("No files to package for %s", moduleName)
		return nil
	}

	tarArgs := fmt.Sprintf("tar czf %s %s", artifactPath, strings.Join(includes, " "))
	logger.Debug("Package command: %s", tarArgs)

	if err := runLocalCmd(tarArgs); err != nil {
		return fmt.Errorf("packaging failed: %w", err)
	}

	logger.Success("Package created: %s", artifactPath)
	return nil
}

// runSteps executes a sequence of named steps with progress logging
func (p *Pipeline) runSteps(steps []struct {
	name string
	fn   func() error
}) error {
	total := len(steps)
	for i, step := range steps {
		logger.Step(i+1, total, "%s", step.name)
		if err := step.fn(); err != nil {
			logger.Error("Step %q failed: %v", step.name, err)
			return fmt.Errorf("pipeline failed at step %q: %w", step.name, err)
		}
	}
	logger.Success("Pipeline completed successfully (%d steps)", total)
	return nil
}

// substituteVars replaces ${ENV}, ${MODULE}, and module variables in a command string
func substituteVars(cmd, envName, moduleName string, vars map[string]string) string {
	cmd = strings.ReplaceAll(cmd, "${ENV}", envName)
	cmd = strings.ReplaceAll(cmd, "${MODULE}", moduleName)
	for k, v := range vars {
		cmd = strings.ReplaceAll(cmd, fmt.Sprintf("${%s}", k), v)
	}
	return cmd
}

// runLocalCmd executes a command on the local machine
func runLocalCmd(command string) error {
	if dryRun {
		logger.Info("[DRY-RUN] Would execute locally: %s", command)
		return nil
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = logger.NewWriter(logger.InfoLevel)
	cmd.Stderr = logger.NewWriter(logger.WarnLevel)
	return cmd.Run()
}
