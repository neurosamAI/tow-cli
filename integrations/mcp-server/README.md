# Tow MCP Server

> by [neurosam.AI](https://neurosam.ai)

Model Context Protocol (MCP) server that exposes Tow deployment operations as tools for AI agents.

## What is this?

This MCP server lets AI coding assistants (Claude, Cursor, Windsurf, etc.) directly manage deployments through natural language:

- "Deploy api-server to production"
- "What's the status of all services in prod?"
- "Roll back the API server"
- "Show me the last 50 error logs from prod"

## Setup

### Claude Desktop / Claude Code

Add to your MCP settings:

```json
{
  "mcpServers": {
    "tow": {
      "command": "tow",
      "args": ["mcp-server"],
      "env": {
        "TOW_CONFIG": "/path/to/your/tow.yaml"
      }
    }
  }
}
```

### VS Code (with Copilot / Continue)

```json
{
  "mcp.servers": {
    "tow": {
      "command": "tow",
      "args": ["mcp-server"]
    }
  }
}
```

## Available Tools

| Tool | Description |
|------|-------------|
| `tow_status` | Check module status on servers |
| `tow_deploy` | Deploy a module to an environment |
| `tow_rollback` | Rollback to a previous deployment |
| `tow_logs` | Fetch recent logs from a module |
| `tow_list_deployments` | List deployment history |
| `tow_list_modules` | List configured modules |
| `tow_list_environments` | List configured environments |

## Security

- The MCP server runs locally with your SSH credentials
- It respects `tow.yaml` branch policies
- Production deployments still require `--yes` flag (no silent prod deploys from AI)
