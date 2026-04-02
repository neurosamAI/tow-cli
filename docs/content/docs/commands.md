---
title: Commands
---

# Command Reference

## Global Flags

These flags are available on all commands:

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | `-c` | Path to config file | `tow.yaml` |
| `--environment` | `-e` | Target environment | — |
| `--module` | `-m` | Target module | — |
| `--server` | `-s` | Target server number (0 = all) | `0` |
| `--verbose` | `-v` | Enable debug output | `false` |
| `--dry-run` | | Show actions without executing | `false` |
| `--insecure` | | Skip SSH host key verification | `false` |

## Commands

### `tow init`

Auto-detect project type and generate configuration.

```bash
tow init
```

Scans your project directory and detects:
- **Language**: Java, Node.js, Python, Go, Rust
- **Framework**: Spring Boot, NestJS, Express, FastAPI, Django, Flask, Next.js
- **Build tool**: Gradle, Maven, npm, yarn, pnpm, poetry, uv, cargo
- **Monorepo modules**: `settings.gradle`, `pom.xml <modules>`
- **CI/CD**: Dockerfile, docker-compose, GitHub Actions, GitLab CI, Jenkins

Generates:
- `tow.yaml` with detected module types and sensible defaults
- `script/{module}/env.sh` — environment variables for each module
- `script/{module}/server` — start/stop/status/restart control script

> **Note**: Library modules (ending in `-common`, `-core`, `-support`, `-lib`, `-shared`, `-util`) are automatically excluded from deployment modules.

### `tow validate`

Validate configuration file without connecting to servers.

```bash
tow validate
tow validate -c custom-config.yaml
```

Checks:
- Required fields (`project.name`, servers, modules)
- SSH key file existence
- Module type validity
- Health check configuration

### `tow deploy`

Run the deploy pipeline: **package → upload → install → stop → start**.

```bash
tow deploy -e prod -m api-server
tow deploy -e prod -m api-server -s 1    # Deploy to server 1 only
tow deploy -e prod                        # Deploy all modules
tow deploy -e prod --dry-run              # Preview without executing
```

### `tow auto`

Run the full pipeline: **build → package → upload → install → stop → start**.

```bash
tow auto -e dev -m api-server
tow auto -e prod -m api-server --dry-run
```

This is the most common command for end-to-end deployment from source code to running service.

### `tow start`

Start a module on remote servers.

```bash
tow start -e prod -m api-server
tow start -e prod -m api-server -s 1
```

After starting, Tow runs health checks based on the module's `health_check` configuration.

### `tow stop`

Stop a module on remote servers.

```bash
tow stop -e prod -m api-server
tow stop -e prod -m api-server -s 2
```

### `tow restart`

Stop then start a module.

```bash
tow restart -e prod -m api-server
```

### `tow status`

Check the current status of a module.

```bash
tow status -e prod -m api-server
```

Shows:
- Process ID (PID)
- Uptime
- Memory usage
- Current deployment version (symlink target)

### `tow rollback`

Switch to a previous deployment version.

```bash
# Roll back to the previous deployment
tow rollback -e prod -m api-server

# Roll back to a specific version
tow rollback -e prod -m api-server -t 20240626-101530
```

Rollback is instant — it switches the `current` symlink and restarts the service. No re-upload or re-build required.

### `tow logs`

Stream remote log files from one or more servers.

```bash
tow logs -e prod -m api-server
tow logs -e prod -m api-server -f "ERROR"                 # Filter with grep
tow logs -e prod -m api-server -n 100                     # Last 100 lines
tow logs -e prod -m api-server -s kafka-1                 # Specific server
tow logs -e prod -m api-server -s kafka-1,kafka-3         # Multiple servers (comma-separated)
tow logs -e prod -m api-server --all                      # All servers simultaneously
tow logs -e prod -m api-server -F                         # Follow (live tail)
```

#### Log Presets

Save and reuse frequently used log queries:

```bash
tow logs -e prod -m api-server -f "ERROR" --save-preset error-logs
tow logs --preset error-logs                              # Replay saved query
tow logs --list-presets                                   # List all saved presets
tow logs --delete-preset error-logs                       # Remove a preset
```

| Flag | Short | Description |
|------|-------|-------------|
| `--all` | | Stream logs from all servers simultaneously |
| `-s` | | Target servers (comma-separated, e.g., `kafka-1,kafka-3`) |
| `-F` | | Follow mode — live tail of log output |
| `--preset` | | Replay a saved log query preset |
| `--save-preset` | | Save the current query as a named preset |
| `--list-presets` | | List all saved log presets |
| `--delete-preset` | | Delete a saved log preset |

#### Multi-Module Logs

Stream logs from multiple modules simultaneously:

```bash
tow logs -e prod -m kafka,zookeeper --all -n 5    # multiple modules
tow logs -e prod -m kafka,zookeeper --all -F      # stream multiple modules
```

Output is prefixed with `[module/server]` for each line.

### `tow ssh`

Execute ad-hoc commands on remote servers over SSH without opening an interactive session.

```bash
tow ssh -e prod -m kafka --all -- "free -h"
tow ssh -e prod -m api-server -s 1 -- "df -h"
tow ssh -e prod -m kafka -s kafka-1,kafka-3 -- "cat /etc/os-release"
```

The command after `--` is executed on every matched server. Output is prefixed with the server name for clarity.

### `tow diff`

Compare deployed code on a remote server against the local build artifact or source.

```bash
tow diff -e prod -m api-server
tow diff -e prod -m api-server -s 1
```

Shows a summary of files that differ between the currently deployed version and the local artifact. Useful for verifying what will change before running `tow deploy`.

### `tow config`

Manage servers, modules, and assignments in `tow.yaml` from the command line.

#### Server management

```bash
tow config server add -e prod --name api-3 --host 10.0.1.30
tow config server remove -e prod --name api-3
tow config server list -e prod
```

#### Module management

```bash
tow config module add --name payment-api --type springboot --port 8083
tow config module remove --name payment-api
tow config module list
```

#### Assign / unassign modules to servers

```bash
tow config assign -e prod --server api-3 --module payment-api
tow config unassign -e prod --server api-3 --module payment-api
```

### `tow setup`

Initialize remote server directory structure.

```bash
tow setup -e dev
tow setup -e prod -m api-server
```

Creates on each server:

```
{base_dir}/{module}-{server_number}/
├── deploy/      # Deployment versions
├── upload/      # Upload staging
├── log/         # Persistent logs
├── conf/        # Persistent configuration
└── {data_dirs}  # Custom data directories
```

### `tow upload`

Transfer a file to remote servers.

```bash
tow upload -e prod -m api-server
tow upload -e prod -m api-server path/to/file.tar.gz
```

### `tow install`

Extract the uploaded package and update the `current` symlink.

```bash
tow install -e prod -m api-server
```

Creates a new timestamped directory in `deploy/` and updates the symlink.

### `tow list`

List configuration information.

```bash
tow list modules       # List all modules
tow list envs          # List all environments
tow list deployments -e prod -m api-server  # List deployment history
```

### `tow login`

Open an interactive SSH session to a server.

```bash
tow login -e prod -m api-server -s 1
```

> **Tip**: When multiple servers or modules match, Tow shows an interactive picker. Use `-s` to specify a server by name, or `-m` for a module. If only one is given, Tow lets you pick the other interactively.

### `tow unlock`

Force release a deploy lock.

```bash
tow unlock -e prod -m api-server
```

Use this when a deploy was interrupted and the lock wasn't cleaned up.

### `tow cleanup`

Remove old deployment directories from remote servers.

```bash
tow cleanup -e prod -m api-server              # Keep default (from config or 5)
tow cleanup -e prod -m api-server --keep 3     # Keep only 3 most recent
```

The current active deployment is never removed. Configure automatic cleanup in `tow.yaml`:

```yaml
retention:
  keep: 5
  auto_cleanup: true    # Clean up after every deploy/auto
```

### `tow download`

Download files from a remote server to local.

```bash
tow download -e prod -m api-server logs/std.log
tow download -e prod -m api-server /var/log/syslog -d ./local-logs/
```

If the path is relative, it's resolved against the module base directory.

### `tow provision`

Provision a new server with basic requirements.

```bash
tow provision -e prod -m api-server --timezone Asia/Seoul --locale en_US.UTF-8 --jre --tools
```

| Flag | Description |
|------|-------------|
| `--timezone` | Set server timezone (e.g., `Asia/Seoul`) |
| `--locale` | Set server locale (e.g., `en_US.UTF-8`) |
| `--jre` | Install Java Runtime Environment |
| `--tools` | Install essential tools (lsof, nc, curl, tar) |

This also runs `tow setup` to create the deployment directory structure.

### `tow threaddump`

Trigger a thread dump on Java/Spring Boot modules for debugging.

```bash
tow threaddump -e prod -m api-server
tow threaddump -e prod -m api-server -s 1
```

Sends `kill -3` to the JVM process. Thread dump output is written to the application's log file.

> Only available for modules with type `springboot` or `java`.

### `tow plugin`

Manage infrastructure plugins.

```bash
# List all installed plugins (bundled + external)
tow plugin list

# Install bundled plugins to ~/.tow/plugins/ (for customization)
tow plugin install kafka redis
tow plugin install --all

# Add community plugins from GitHub or URL
tow plugin add someuser/tow-plugin-mssql
tow plugin add myorg/infra-plugins/oracle.yaml
tow plugin add https://example.com/my-plugin.yaml

# Remove
tow plugin remove mssql
```

35 plugins are bundled in the binary. Community plugins are installed to `~/.tow/plugins/`.

See [plugins/README.md](https://github.com/neurosamAI/tow-cli/tree/main/plugins) for the full list and plugin authoring guide.

### `tow metrics`

Show deployment statistics from audit log.

```bash
tow metrics                           # last 30 days, all modules
tow metrics -e prod                   # filter by environment
tow metrics -m api-server             # filter by module
tow metrics --days 7                  # last 7 days
```

Shows total deployments, breakdown by action (deploy/auto/rollback), by module (with bar chart), and by day of week.

---

### `tow doctor`

Pre-flight diagnostics — checks config, SSH, remote directories, disk space, and deploy locks.

```bash
tow doctor -e prod -m api-server --insecure

  ✓ tow.yaml is valid
  ✓ Environment 'prod' exists
  ✓ SSH key exists
  ✓ Servers configured (22)
  ✓ SSH connection to 10.0.1.10
  ✓ Remote dir exists: /home/ec2-user/api-server-1
  ✓ Disk space — Available: 15GB
  ✓ Branch policy
  ✓ No active deploy lock
```

Run without `-m` to check environment-level connectivity.

---

### `tow mcp-server`

Start the MCP (Model Context Protocol) server for AI agent integration.

```bash
tow mcp-server
tow mcp-server -c /path/to/tow.yaml
```

This exposes Tow operations as tools for AI assistants (Claude, Cursor, Windsurf). See [AI Agent Integration](#ai-agent-integration) below.

## Advanced Flags

### Rolling Deployment

Deploy to one server at a time, verifying health before proceeding to the next:

```bash
tow deploy -e prod -m api-server --rolling
tow auto -e prod -m api-server --rolling
tow start -e prod -m api-server --rolling
```

### Auto-Rollback

Automatically revert to the previous deployment if health check fails after start:

```bash
tow auto -e prod -m api-server --auto-rollback
```

### Production Confirmation

Production environments (`prod`, `production`, `live`) require interactive confirmation:

```
⚠  WARNING: You are about to deploy api-server in PROD
  Type 'yes' to confirm:
```

Skip with `-y` / `--yes` flag (for CI/CD):

```bash
tow deploy -e prod -m api-server -y
```

### JSON Output

Machine-readable output for CI/CD pipelines:

```bash
tow status -e prod -m api-server -o json
tow list deployments -e prod -m api-server -o json
```

## AI Agent Integration

### MCP Server

Configure in Claude Desktop, Cursor, or any MCP-compatible client:

```json
{
  "mcpServers": {
    "tow": {
      "command": "tow",
      "args": ["mcp-server"],
      "env": { "TOW_CONFIG": "./tow.yaml" }
    }
  }
}
```

**Available MCP tools:**

| Tool | Description |
|------|-------------|
| `tow_status` | Check module status (PID, uptime, memory) |
| `tow_deploy` | Deploy a module (with production safety gate) |
| `tow_rollback` | Rollback to previous deployment |
| `tow_logs` | Fetch recent log lines |
| `tow_list_deployments` | List deployment history |
| `tow_list_modules` | List configured modules |
| `tow_list_environments` | List configured environments |

## Command Pipelines

| Command | build | package | upload | install | stop | start | health check |
|---------|:-----:|:-------:|:------:|:-------:|:----:|:-----:|:------------:|
| `auto` | ● | ● | ● | ● | ● | ● | ● |
| `deploy` | | ● | ● | ● | ● | ● | ● |
| `start` | | | | | | ● | ● |
| `stop` | | | | | ● | | |
| `restart` | | | | | ● | ● | ● |
| `rollback` | | | | ● | ● | ● | ● |
| `upload` | | | ● | | | | |
| `install` | | | | ● | | | |

## Dry-Run Mode

Add `--dry-run` to any command to preview actions without executing:

```bash
tow auto -e prod -m api-server --dry-run
```

```
[1/6] Building api-server...
  → Would run: ./gradlew :api-server:bootJar
[2/6] Packaging api-server...
  → Would create: build/api-server.tar.gz
[3/6] Uploading to prod-1 (52.78.100.1)...
  → Would SCP: build/api-server.tar.gz → ~/upload/
[4/6] Installing on prod-1...
  → Would extract to: deploy/20240626-143022/
  → Would update symlink: current → deploy/20240626-143022
[5/6] Stopping api-server on prod-1...
  → Would run: bin/server stop
[6/6] Starting api-server on prod-1...
  → Would run: bin/server start
```
