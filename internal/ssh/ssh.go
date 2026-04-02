package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Manager handles SSH connections to remote servers
type Manager struct {
	connections     map[string]*ssh.Client
	mu              sync.Mutex
	connSem         chan struct{} // semaphore for max concurrent connections
	InsecureHostKey bool
	DryRun          bool
	MaxRetries      int           // Max retries for transient SSH failures (default: 3)
	RetryDelay      time.Duration // Delay between retries (default: 2s)
	CommandTimeout  time.Duration // Per-command execution timeout (default: 10m)
	MaxConcurrent   int           // Max concurrent SSH connections (default: 20)
}

// NewManager creates a new SSH connection manager
func NewManager(insecureHostKey bool) *Manager {
	return &Manager{
		connections:     make(map[string]*ssh.Client),
		connSem:         make(chan struct{}, 20),
		InsecureHostKey: insecureHostKey,
		MaxRetries:      3,
		RetryDelay:      2 * time.Second,
		CommandTimeout:  10 * time.Minute,
		MaxConcurrent:   20,
	}
}

// IsDryRun returns whether the manager is in dry-run mode
func (m *Manager) IsDryRun() bool { return m.DryRun }

// Close closes all open SSH connections
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, client := range m.connections {
		client.Close()
		delete(m.connections, key)
	}
}

// connectionKey generates a unique key for an SSH connection
func connectionKey(host string, port int, user string) string {
	return fmt.Sprintf("%s@%s:%d", user, host, port)
}

// getClient returns a cached or new SSH client with retry logic
func (m *Manager) getClient(host string, port int, user, keyPath string, authCfg *config.SSHConfig) (*ssh.Client, error) {
	key := connectionKey(host, port, user)

	m.mu.Lock()
	client, ok := m.connections[key]
	m.mu.Unlock()

	if ok {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return client, nil
		}
		m.mu.Lock()
		delete(m.connections, key)
		m.mu.Unlock()
	}

	// Acquire connection semaphore
	m.connSem <- struct{}{}
	defer func() { <-m.connSem }()

	// Retry connection with backoff
	var lastErr error
	retries := m.MaxRetries
	if retries < 1 {
		retries = 1
	}

	for attempt := 1; attempt <= retries; attempt++ {
		newClient, err := m.dial(host, port, user, keyPath, authCfg)
		if err == nil {
			m.mu.Lock()
			m.connections[key] = newClient
			m.mu.Unlock()
			return newClient, nil
		}
		lastErr = err
		if attempt < retries {
			logger.Debug("[%s] SSH connection attempt %d/%d failed: %v, retrying in %s...", host, attempt, retries, err, m.RetryDelay)
			time.Sleep(m.RetryDelay)
		}
	}

	return nil, fmt.Errorf("SSH connection failed after %d attempts: %w", retries, lastErr)
}

// hostKeyCallback returns the appropriate host key callback based on configuration
func (m *Manager) hostKeyCallback() ssh.HostKeyCallback {
	if m.InsecureHostKey {
		return ssh.InsecureIgnoreHostKey()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("Cannot determine home directory, falling back to insecure host key checking: %v", err)
		return ssh.InsecureIgnoreHostKey()
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		logger.Error("SSH known_hosts file not found: %s", knownHostsPath)
		logger.Error("Fix: ssh-keyscan -H <your-server> >> %s", knownHostsPath)
		logger.Error("Or use --insecure flag to skip host key verification (NOT recommended for production)")
		// Return a callback that always rejects — forces user to fix
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return fmt.Errorf("host key verification failed: %s not in known_hosts. Run: ssh-keyscan -H %s >> %s", hostname, hostname, knownHostsPath)
		}
	}

	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		logger.Warn("Failed to load known hosts from %s, falling back to insecure host key checking: %v", knownHostsPath, err)
		return ssh.InsecureIgnoreHostKey()
	}

	return hostKeyCallback
}

// dial creates a new SSH connection with support for multiple auth modes
func (m *Manager) dial(host string, port int, user, keyPath string, authCfg *config.SSHConfig) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	authMode := "key"
	if authCfg != nil && authCfg.Auth != "" {
		authMode = authCfg.Auth
	}

	switch authMode {
	case "password":
		password := ""
		if authCfg != nil {
			password = authCfg.Password
		}
		authMethods = []ssh.AuthMethod{ssh.Password(password)}
	case "agent":
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, fmt.Errorf("SSH_AUTH_SOCK not set, cannot use ssh-agent auth")
		}
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, fmt.Errorf("connecting to ssh-agent: %w", err)
		}
		agentClient := agent.NewClient(conn)
		authMethods = []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)}
	default: // "key"
		actualKeyPath := keyPath
		if authCfg != nil && authCfg.KeyPath != "" {
			actualKeyPath = authCfg.KeyPath
		}
		expandedPath := expandPath(actualKeyPath)
		keyData, err := os.ReadFile(expandedPath)
		if err != nil {
			return nil, fmt.Errorf("reading SSH key %s: %w", expandedPath, err)
		}

		// Try parsing without passphrase first, then with passphrase via agent fallback
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			if _, ok := err.(*ssh.PassphraseMissingError); ok {
				// Try SSH agent as fallback for passphrase-protected keys
				sock := os.Getenv("SSH_AUTH_SOCK")
				if sock != "" {
					conn, agentErr := net.Dial("unix", sock)
					if agentErr == nil {
						agentClient := agent.NewClient(conn)
						authMethods = []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)}
						goto connect
					}
				}
				return nil, fmt.Errorf("SSH key %s is passphrase-protected; use ssh-agent or auth: agent", expandedPath)
			}
			return nil, fmt.Errorf("parsing SSH key: %w", err)
		}
		authMethods = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

connect:
	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: m.hostKeyCallback(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Debug("SSH connecting to %s", addr)

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", addr, err)
	}

	logger.Debug("SSH connected to %s", addr)
	return client, nil
}

// ExecResult holds the result of a remote command execution
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Host     string
}

// Exec executes a command on a remote server with timeout
func (m *Manager) Exec(env *config.Environment, host, command string) (*ExecResult, error) {
	if m.DryRun {
		logger.Info("[DRY-RUN] Would execute on %s: %s", host, command)
		return &ExecResult{Host: host}, nil
	}

	client, err := m.getClient(host, env.SSHPort, env.SSHUser, env.SSHKeyPath, nil)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	logger.Debug("[%s] exec: %s", host, command)

	// Execute with timeout
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	timeout := m.CommandTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var runErr error
	select {
	case runErr = <-done:
	case <-ctx.Done():
		session.Signal(ssh.SIGKILL)
		return &ExecResult{Host: host, Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: -1},
			fmt.Errorf("command timed out after %s on %s", timeout, host)
	}

	result := &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Host:   host,
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			return result, fmt.Errorf("SSH exec on %s: %w", host, runErr)
		}
	}

	return result, nil
}

// ExecStream executes a command and streams output in real-time
func (m *Manager) ExecStream(env *config.Environment, host, command string, stdout, stderr io.Writer) error {
	if m.DryRun {
		logger.Info("[DRY-RUN] Would stream execute on %s: %s", host, command)
		return nil
	}

	client, err := m.getClient(host, env.SSHPort, env.SSHUser, env.SSHKeyPath, nil)
	if err != nil {
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	session.Stdout = stdout
	session.Stderr = stderr

	return session.Run(command)
}

// Upload transfers a local file to a remote server via SCP
func (m *Manager) Upload(env *config.Environment, host, localPath, remotePath string) error {
	if m.DryRun {
		logger.Info("[DRY-RUN] Would upload %s to %s:%s", localPath, host, remotePath)
		return nil
	}

	client, err := m.getClient(host, env.SSHPort, env.SSHUser, env.SSHKeyPath, nil)
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat local file %s: %w", localPath, err)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening local file %s: %w", localPath, err)
	}
	defer file.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	w, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating stdin pipe: %w", err)
	}

	go func() {
		defer w.Close()
		fileName := filepath.Base(remotePath)
		fmt.Fprintf(w, "C0644 %d %s\n", fileInfo.Size(), fileName)
		io.Copy(w, file)
		fmt.Fprint(w, "\x00")
	}()

	remoteDir := filepath.Dir(remotePath)
	err = session.Run(fmt.Sprintf("scp -t %s", remoteDir))
	if err != nil {
		return fmt.Errorf("SCP to %s:%s: %w", host, remotePath, err)
	}

	return nil
}

// Download copies a file from a remote server to local via SCP
func (m *Manager) Download(env *config.Environment, host, remotePath, localDir string) error {
	if m.DryRun {
		logger.Info("[DRY-RUN] Would download %s:%s to %s", host, remotePath, localDir)
		return nil
	}

	expandedKey := expandPath(env.SSHKeyPath)

	args := []string{
		"-i", expandedKey,
		"-P", fmt.Sprintf("%d", env.SSHPort),
	}
	if m.InsecureHostKey {
		args = append(args, "-o", "StrictHostKeyChecking=no")
	}
	args = append(args, fmt.Sprintf("%s@%s:%s", env.SSHUser, host, remotePath), localDir+"/")

	cmd := exec.Command("scp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Debug("[%s] scp download %s → %s", host, remotePath, localDir)
	return cmd.Run()
}

// UploadDir transfers a local directory to a remote server via tar + SCP
func (m *Manager) UploadDir(env *config.Environment, host, localDir, remoteDir string) error {
	if m.DryRun {
		logger.Info("[DRY-RUN] Would upload directory %s to %s:%s", localDir, host, remoteDir)
		return nil
	}

	expandedKey := expandPath(env.SSHKeyPath)

	// Create temp tar.gz
	tmpFile, err := os.CreateTemp("", "tow-upload-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// tar the directory
	tarCmd := exec.Command("tar", "czf", tmpPath, "-C", localDir, ".")
	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("tar %s: %w", localDir, err)
	}

	// SCP to remote
	remoteTmp := fmt.Sprintf("/tmp/tow-upload-%d.tar.gz", time.Now().UnixNano())
	scpArgs := []string{"-i", expandedKey, "-P", fmt.Sprintf("%d", env.SSHPort)}
	if m.InsecureHostKey {
		scpArgs = append(scpArgs, "-o", "StrictHostKeyChecking=no")
	}
	scpArgs = append(scpArgs, tmpPath, fmt.Sprintf("%s@%s:%s", env.SSHUser, host, remoteTmp))

	scpCmd := exec.Command("scp", scpArgs...)
	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("scp to %s: %w", host, err)
	}

	// Extract on remote
	extractCmd := fmt.Sprintf("mkdir -p %s && tar xzf %s -C %s && rm -f %s", remoteDir, remoteTmp, remoteDir, remoteTmp)
	sshArgs := []string{"-i", expandedKey, "-p", fmt.Sprintf("%d", env.SSHPort)}
	if m.InsecureHostKey {
		sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=no")
	}
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", env.SSHUser, host), extractCmd)

	sshCmd := exec.Command("ssh", sshArgs...)
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("extract on %s: %w", host, err)
	}

	logger.Debug("[%s] uploaded dir %s → %s", host, localDir, remoteDir)
	return nil
}

// InteractiveLogin opens an interactive SSH session
func (m *Manager) InteractiveLogin(cfg *config.Config, envName, moduleName string, serverNum int) error {
	servers, env, err := cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	srv := servers[0]
	expandedKey := expandPath(env.SSHKeyPath)

	logger.Info("Logging into %s@%s:%d", env.SSHUser, srv.Host, env.SSHPort)

	sshArgs := []string{
		"-i", expandedKey,
		"-p", fmt.Sprintf("%d", env.SSHPort),
	}
	if m.InsecureHostKey {
		sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=no")
	}
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", env.SSHUser, srv.Host))

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// resolveAuth resolves the SSH connection parameters for a specific module
func (m *Manager) resolveAuth(env *config.Environment, moduleName string, cfg *config.Config) (user string, port int, keyPath string, authConfig *config.SSHConfig) {
	user = env.SSHUser
	port = env.SSHPort
	keyPath = env.SSHKeyPath

	if cfg != nil && moduleName != "" {
		if mod, ok := cfg.Modules[moduleName]; ok && mod.SSH != nil {
			if mod.SSH.User != "" {
				user = mod.SSH.User
			}
			if mod.SSH.Port > 0 {
				port = mod.SSH.Port
			}
			if mod.SSH.KeyPath != "" {
				keyPath = mod.SSH.KeyPath
			}
			authConfig = mod.SSH
		}
	}
	return
}
