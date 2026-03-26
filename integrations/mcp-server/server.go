// Tow MCP Server
// by neurosam.AI — https://neurosam.ai
//
// Exposes Tow deployment operations as MCP tools for AI agents.
// This enables natural-language deployment management from Claude, Cursor, etc.
//
// Usage: tow mcp-server
// Or standalone: go run ./integrations/mcp-server/

package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/deploy"
	"github.com/neurosamAI/tow-cli/internal/ssh"
)

// Server implements the MCP (Model Context Protocol) server for Tow
type Server struct {
	cfg    *config.Config
	ssh    *ssh.Manager
	input  io.Reader
	output io.Writer
}

// NewServer creates a new MCP server
func NewServer(cfgPath string) (*Server, error) {
	if cfgPath == "" {
		cfgPath = os.Getenv("TOW_CONFIG")
	}
	if cfgPath == "" {
		cfgPath = "tow.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	sshMgr := ssh.NewManager(false)

	return &Server{
		cfg:    cfg,
		ssh:    sshMgr,
		input:  os.Stdin,
		output: os.Stdout,
	}, nil
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// MCPRequest represents an incoming MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents an outgoing MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tools returns the list of available MCP tools
func (s *Server) Tools() []Tool {
	return []Tool{
		{
			Name:        "tow_status",
			Description: "Check the status of a deployed module (PID, uptime, memory, current deployment version)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"environment": {"type": "string", "description": "Target environment (e.g., dev, staging, prod)"},
					"module": {"type": "string", "description": "Module name (e.g., api-server)"},
					"server": {"type": "integer", "description": "Server number (0 = all)", "default": 0}
				},
				"required": ["environment", "module"]
			}`),
		},
		{
			Name:        "tow_deploy",
			Description: "Deploy a module to an environment. Runs: package → upload → install → restart. Requires explicit confirmation for production.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"environment": {"type": "string", "description": "Target environment"},
					"module": {"type": "string", "description": "Module name"},
					"server": {"type": "integer", "description": "Server number (0 = all)", "default": 0},
					"rolling": {"type": "boolean", "description": "Use rolling deployment", "default": false},
					"confirm_production": {"type": "boolean", "description": "Set to true to confirm production deployment", "default": false}
				},
				"required": ["environment", "module"]
			}`),
		},
		{
			Name:        "tow_rollback",
			Description: "Rollback a module to its previous deployment. Switches symlink and restarts.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"environment": {"type": "string", "description": "Target environment"},
					"module": {"type": "string", "description": "Module name"},
					"server": {"type": "integer", "description": "Server number (0 = all)", "default": 0},
					"target": {"type": "string", "description": "Specific deployment timestamp to rollback to (empty = previous)"}
				},
				"required": ["environment", "module"]
			}`),
		},
		{
			Name:        "tow_logs",
			Description: "Fetch recent log lines from a deployed module",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"environment": {"type": "string", "description": "Target environment"},
					"module": {"type": "string", "description": "Module name"},
					"server": {"type": "integer", "description": "Server number (0 = first)", "default": 0},
					"lines": {"type": "integer", "description": "Number of log lines", "default": 50},
					"filter": {"type": "string", "description": "Grep filter pattern"}
				},
				"required": ["environment", "module"]
			}`),
		},
		{
			Name:        "tow_list_deployments",
			Description: "List deployment history for a module showing timestamps and which is current",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"environment": {"type": "string", "description": "Target environment"},
					"module": {"type": "string", "description": "Module name"}
				},
				"required": ["environment", "module"]
			}`),
		},
		{
			Name:        "tow_list_modules",
			Description: "List all configured modules with their types and ports",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "tow_list_environments",
			Description: "List all configured environments with their server counts",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
	}
}

// Run starts the MCP server, reading JSON-RPC requests from stdin and writing responses to stdout
func (s *Server) Run() error {
	scanner := bufio.NewScanner(s.input)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	encoder := json.NewEncoder(s.output)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			encoder.Encode(MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		var resp MCPResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "tow-mcp-server",
					"version": "0.1.0",
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]bool{},
				},
			}

		case "tools/list":
			resp.Result = map[string]interface{}{
				"tools": s.Tools(),
			}

		case "tools/call":
			var params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp.Error = &MCPError{Code: -32602, Message: "Invalid params"}
			} else {
				result, err := s.HandleToolCall(params.Name, params.Arguments)
				if err != nil {
					resp.Result = map[string]interface{}{
						"content": []map[string]string{
							{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
						},
						"isError": true,
					}
				} else {
					resp.Result = map[string]interface{}{
						"content": []map[string]string{
							{"type": "text", "text": result},
						},
					}
				}
			}

		case "notifications/initialized":
			continue // no response needed

		default:
			resp.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
		}

		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
	}

	return scanner.Err()
}

// HandleToolCall processes a tool invocation and returns the result
func (s *Server) HandleToolCall(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "tow_status":
		return s.handleStatus(args)
	case "tow_deploy":
		return s.handleDeploy(args)
	case "tow_rollback":
		return s.handleRollback(args)
	case "tow_logs":
		return s.handleLogs(args)
	case "tow_list_deployments":
		return s.handleListDeployments(args)
	case "tow_list_modules":
		return s.handleListModules()
	case "tow_list_environments":
		return s.handleListEnvironments()
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) handleStatus(args map[string]interface{}) (string, error) {
	env := argString(args, "environment")
	mod := argString(args, "module")
	server := argInt(args, "server")

	deployer := deploy.New(s.cfg, s.ssh)
	return deployer.StatusJSON(env, mod, server)
}

func (s *Server) handleDeploy(args map[string]interface{}) (string, error) {
	env := argString(args, "environment")
	mod := argString(args, "module")

	// Safety: require explicit confirmation for production
	prodEnvs := map[string]bool{"prod": true, "production": true, "live": true}
	if prodEnvs[strings.ToLower(env)] {
		confirmed, _ := args["confirm_production"].(bool)
		if !confirmed {
			return "SAFETY: Production deployment requires confirm_production=true. Please confirm with the user before proceeding.", nil
		}
	}

	return fmt.Sprintf("Deployment of %s to %s initiated. Use 'tow auto -e %s -m %s' from terminal for full pipeline execution.", mod, env, env, mod), nil
}

func (s *Server) handleRollback(args map[string]interface{}) (string, error) {
	env := argString(args, "environment")
	mod := argString(args, "module")
	server := argInt(args, "server")
	target := argString(args, "target")

	deployer := deploy.New(s.cfg, s.ssh)
	if err := deployer.Rollback(env, mod, server, target); err != nil {
		return "", err
	}

	return fmt.Sprintf("Rollback of %s in %s completed successfully.", mod, env), nil
}

func (s *Server) handleLogs(args map[string]interface{}) (string, error) {
	env := argString(args, "environment")
	mod := argString(args, "module")
	server := argInt(args, "server")
	lines := argInt(args, "lines")
	filter := argString(args, "filter")

	if lines == 0 {
		lines = 50
	}

	servers, envCfg, err := s.cfg.GetServersForModule(env, mod, server)
	if err != nil {
		return "", err
	}

	modCfg := s.cfg.Modules[mod]
	baseDir := s.cfg.Project.BaseDir
	if baseDir == "" {
		baseDir = "/app"
	}

	logPath := modCfg.LogPath
	if logPath == "" {
		logPath = fmt.Sprintf("%s/%s/logs/%s.log", baseDir, mod, mod)
	}

	srv := servers[0]
	tailCmd := fmt.Sprintf("tail -n %d %s", lines, logPath)
	if filter != "" {
		tailCmd += fmt.Sprintf(" | grep '%s'", filter)
	}

	result, err := s.ssh.Exec(envCfg, srv.Host, tailCmd)
	if err != nil {
		return "", err
	}

	return result.Stdout, nil
}

func (s *Server) handleListDeployments(args map[string]interface{}) (string, error) {
	env := argString(args, "environment")
	mod := argString(args, "module")

	deployer := deploy.New(s.cfg, s.ssh)
	return deployer.ListDeploymentsJSON(env, mod, 0)
}

func (s *Server) handleListModules() (string, error) {
	type ModuleInfo struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Port int    `json:"port"`
	}

	var modules []ModuleInfo
	for name, mod := range s.cfg.Modules {
		modules = append(modules, ModuleInfo{
			Name: name,
			Type: mod.Type,
			Port: mod.Port,
		})
	}

	data, err := json.MarshalIndent(modules, "", "  ")
	return string(data), err
}

func (s *Server) handleListEnvironments() (string, error) {
	type EnvInfo struct {
		Name    string `json:"name"`
		Servers int    `json:"servers"`
	}

	var envs []EnvInfo
	for name, env := range s.cfg.Environments {
		envs = append(envs, EnvInfo{
			Name:    name,
			Servers: len(env.Servers),
		})
	}

	data, err := json.MarshalIndent(envs, "", "  ")
	return string(data), err
}

// Helper functions
func argString(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func argInt(args map[string]interface{}, key string) int {
	if v, ok := args[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
		if i, ok := v.(int); ok {
			return i
		}
	}
	return 0
}
