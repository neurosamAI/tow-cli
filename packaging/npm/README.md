<p align="center">
  <strong>&#9875; Tow</strong>
</p>

<p align="center">
  <em>Lightweight, agentless deployment orchestrator for bare-metal servers and cloud VMs.</em>
</p>

<p align="center">
  Created by <a href="https://github.com/comchangs"><strong>Murry Jeong</strong></a> &middot; Supported by <a href="https://neurosam.ai"><strong>neurosam.AI</strong></a>
</p>

<p align="center">
  <a href="https://github.com/neurosamAI/tow-cli/actions/workflows/ci.yml"><img src="https://github.com/neurosamAI/tow-cli/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/neurosamAI/tow-cli/releases"><img src="https://img.shields.io/github/v/release/neurosamAI/tow-cli" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://goreportcard.com/report/github.com/neurosamAI/tow-cli"><img src="https://goreportcard.com/badge/github.com/neurosamAI/tow-cli" alt="Go Report Card"></a>
</p>

<p align="center">
  <a href="https://tow-cli.neurosam.ai">Documentation</a> &middot;
  <a href="https://tow-cli.neurosam.ai/getting-started">Getting Started</a> &middot;
  <a href="https://tow-cli.neurosam.ai/examples">Examples</a> &middot;
  <a href="https://tow-cli.neurosam.ai/comparison">Why Tow?</a>
</p>

---

Many teams run services on EC2 instances, bare-metal servers, or simple VMs. They don't need Kubernetes, but they still want reliable, repeatable deployments with zero-downtime rollbacks. **Tow fills this gap.**

```bash
tow init                              # Auto-detect project → generate config
tow auto -e prod -m api-server        # Build → package → upload → deploy
tow rollback -e prod -m api-server    # Instant rollback
```

<details>
<summary><b>See it in action — real production output</b></summary>

```
$ tow status -e prod -m api-server -o json
[
  {
    "host": "3.34.xx.xxx",
    "server": "api-1",
    "status": "running",
    "pid": "23117",
    "uptime": "56-01:58:33",
    "memory": "962468KB",
    "deployment": "20240424-110212"
  }
]

$ tow logs -e prod -m kafka --all -n 3
[kafka-1] 2026-03-30 14:35:49 GC(7722152) Pause Young 765M→702M(1024M) 17ms
[kafka-2] 2026-03-30 14:35:55 GC(8629812) Pause Young 340M→292M(1024M) 18ms
[kafka-3] 2026-03-30 14:36:01 GC(7493779) Pause Young 797M→723M(1024M) 13ms

$ tow ssh -e prod -m kafka --all -- "free -h | head -2"
[kafka-1]              total        used        free
[kafka-1] Mem:         1.9Gi       1.7Gi        66Mi
[kafka-2]              total        used        free
[kafka-2] Mem:         1.9Gi       1.7Gi        77Mi
[kafka-3]              total        used        free
[kafka-3] Mem:         1.9Gi       1.7Gi        70Mi

$ tow doctor -e prod -m api-server
  ✓ tow.yaml is valid
  ✓ Environment 'prod' exists
  ✓ SSH key exists
  ✓ Servers configured (22)
  ✓ SSH connection successful
  ✓ Remote dir exists
  ✓ Disk space — Available: 4.9G
  ✓ Branch policy
  ✓ No active deploy lock
  9 passed, 0 failed
```

</details>

## Why Tow?

| | Tow | Ansible | Capistrano | Kamal |
|---|:---:|:---:|:---:|:---:|
| Single binary, zero deps | **Yes** | Python + pip | Ruby + Bundler | Ruby + Docker |
| Auto-detection | **Yes** | No | No | No |
| No Docker required | **Yes** | Yes | Yes | **No** |
| Multi-language native | **12 types** | Manual | Ruby-first | Docker-only |
| Built-in health checks | **4 types** | Manual | Plugin | HTTP only |
| Instant rollback | **Symlink** | Re-run playbook | Symlink | Container swap |

> See the [full comparison](https://tow-cli.neurosam.ai/comparison) for detailed analysis.

### What makes Tow different

**Most deployment tools force you into their world.** Ansible requires you to become a YAML/Jinja expert. Capistrano assumes you're a Ruby shop. Kamal demands Docker on every server. Tow works with what you already have.

<details>
<summary><b>Auto-detection that no other tool has</b></summary>
<br>

Every other deployment tool starts with a blank config file you have to write from scratch. Tow scans your project and generates everything:

```bash
$ cd my-springboot-monorepo
$ tow init

✔ Detected: springboot (gradle, multi-module)
  Modules: api-server, batch-server, admin-server
  Excluded: common-lib, data-core (library modules)
  Framework: Spring Boot 3.x
✔ Generated tow.yaml
✔ Generated script/api-server/env.sh + server
✔ Generated script/batch-server/env.sh + server
✔ Generated script/admin-server/env.sh + server
```

It detects languages, frameworks (NestJS, FastAPI, Django, Flask, Next.js...), build tools, monorepo sub-modules, and even filters out library modules like `-common` or `-core`. **Zero to deployable config in seconds.**

</details>

<details>
<summary><b>No Docker? No problem. (Unlike Kamal)</b></summary>
<br>

[Kamal](https://kamal-deploy.org/) is excellent — but it requires Docker on every target server. That's a non-starter for:

- **JVM apps** that already manage their own lifecycle and don't benefit from containerization
- **Legacy infrastructure** where installing Docker isn't an option
- **Resource-constrained VMs** where the Docker daemon overhead matters
- **GPU workloads** where Docker GPU passthrough adds complexity
- **Regulated environments** with container usage restrictions

Tow deploys directly to the OS. If you can SSH in, you can deploy.

</details>

<details>
<summary><b>One tool for apps AND infrastructure</b></summary>
<br>

Most deployment tools only handle application code. If you also run Kafka, Redis, or MongoDB on VMs, you need separate automation.

Tow manages everything through one interface:

```bash
tow auto -e prod -m api-server     # Spring Boot app
tow auto -e prod -m kafka          # Kafka broker
tow auto -e prod -m redis          # Redis server
tow status -e prod                  # Check everything
```

Same config file. Same commands. Same rollback mechanism.

</details>

<details>
<summary><b>Battle-tested patterns, not theory</b></summary>
<br>

Tow isn't a weekend project. It's the Go rewrite of deployment systems that ran in production across multiple companies — managing Spring Boot microservices, Kafka clusters, Redis, MongoDB, Prometheus, Grafana, and more.

Every feature exists because a real production incident demanded it:

- **Deploy locking** → Two developers deployed at the same time, caused inconsistent state
- **Branch policies** → Someone deployed a feature branch to production
- **Health checks** → Deploys "succeeded" but the app was crash-looping
- **Hierarchical config** → Server 1 needed different JVM settings than server 2

These aren't theoretical features. They're battle scars turned into safeguards.

</details>

## Core Features

- **Agentless** — SSH is the only requirement on target servers
- **Symlink-based atomic deployments** — instant rollback via symlink switch
- **Auto-detection** — `tow init` detects your project type, framework, build tool, and monorepo modules
- **12 built-in module handlers** — Spring Boot, Java, Node.js, Python, Go, Rust, Kafka, Redis, generic
- **4 health check types** — HTTP, TCP, log pattern, custom command
- **Hierarchical config** — environment-level defaults with per-server overrides
- **Branch policies** — prevent deploying wrong branches to production
- **Deploy locking** — prevent concurrent deployments
- **Parallel execution** — deploy to multiple servers simultaneously
- **Lifecycle hooks** — pre/post build, deploy, start, stop

## Installation

```bash
# Homebrew (macOS / Linux)
brew install neurosamAI/tap/tow

# npm
npm install -g @neurosamai/tow

# Go
go install github.com/neurosamAI/tow-cli/cmd/tow@latest

# Binary (macOS Apple Silicon)
curl -L https://github.com/neurosamAI/tow-cli/releases/latest/download/tow-darwin-arm64 -o tow
chmod +x tow && sudo mv tow /usr/local/bin/
```

## Quick Start

```bash

# Initialize — auto-detects project type and generates tow.yaml
cd my-project
tow init

# Edit tow.yaml with your server IPs and SSH key paths
# Then setup remote servers and deploy
tow setup -e prod -m api-server
tow auto -e prod -m api-server
```

> See the [Getting Started guide](https://tow-cli.neurosam.ai/getting-started) for detailed instructions.

## How Deployment Works

```
Local Machine                    Remote Server
─────────────                    ─────────────
1. build      →  compile/jar
2. package    →  tar.gz artifact
3. upload     →  ─── SCP ───→   ~/upload/module.tar.gz
4. install    →  ─── SSH ───→   deploy/20240626-101530/  (extract)
                                current → deploy/20240626-101530  (symlink)
5. stop/start →  ─── SSH ───→   bin/stop.sh → bin/start.sh
```

**Rollback** is instant — just switch the `current` symlink to a previous deployment directory.

## Commands

| Command | Description |
|---------|-------------|
| `tow init` | Auto-detect project type and generate `tow.yaml` |
| `tow validate` | Validate configuration file |
| `tow setup` | Initialize remote server directories |
| `tow auto` | Full pipeline: build → package → upload → install → restart |
| `tow deploy` | Deploy pipeline: package → upload → install → restart |
| `tow start` | Start a module |
| `tow stop` | Stop a module |
| `tow restart` | Restart a module |
| `tow status` | Check module status (PID, uptime, memory) |
| `tow rollback` | Switch to previous deployment |
| `tow logs` | Stream remote logs with optional grep filter |
| `tow ssh` | Execute commands on remote servers |
| `tow diff` | Compare deployed vs local code |
| `tow config` | Manage servers, modules, and assignments |
| `tow upload` | Upload a file to target servers |
| `tow install` | Install uploaded package (extract + symlink) |
| `tow login` | SSH into a server |
| `tow unlock` | Force release deploy lock |
| `tow list` | List modules, environments, or deployment history |
| `tow cleanup` | Remove old deployment directories |
| `tow download` | Download files from remote servers |
| `tow provision` | Provision a new server (timezone, JRE, tools) |
| `tow metrics` | Show deployment statistics from audit log |
| `tow doctor` | Pre-flight diagnostics |
| `tow mcp-server` | Start MCP server for AI agent integration |

**Advanced flags:**

```bash
tow deploy --rolling          # Deploy one server at a time
tow auto --auto-rollback      # Auto-rollback if health check fails
tow auto -y                   # Skip production confirmation prompt
tow status -o json            # Machine-readable JSON output
```

> See the [full command reference](https://tow-cli.neurosam.ai/commands) for all flags and usage details.

## Module Types

| Type | Default Build | Default Health Check |
|------|---------------|---------------------|
| `springboot` | `gradlew bootJar` | HTTP /actuator/health |
| `java` | `gradlew build` | TCP port check |
| `node` | `npm ci && npm run build` | TCP port check |
| `python` | `pip install -r requirements.txt` | TCP port check |
| `go` | `go build` | TCP port check |
| `rust` | `cargo build --release` | TCP port check |
| `php` | `composer install` | TCP port check |
| `ruby` | `bundle install` | TCP port check |
| `dotnet` | `dotnet publish` | TCP port check |
| `kotlin` | `./gradlew build` | TCP port check |
| `elixir` | `mix deps.get, compile, release` | TCP port check |
| `generic` | — | TCP port check |

Infrastructure services are supported via [35 bundled YAML plugins](plugins/) — Kafka, Redis, MySQL, PostgreSQL, MongoDB, Elasticsearch, ZooKeeper, Nginx, Prometheus, Grafana, Loki, Vault, Jenkins, and more.

Community plugins can be installed from GitHub:

```bash
tow plugin add someuser/tow-plugin-mssql          # GitHub repo
tow plugin add myorg/infra-plugins/oracle.yaml     # specific file
tow plugin add https://example.com/custom.yaml     # any URL
tow plugin list                                     # list all (bundled + external)
```

## Documentation

Full documentation is available at **[tow-cli.neurosam.ai](https://tow-cli.neurosam.ai)**:

- [Getting Started](https://tow-cli.neurosam.ai/getting-started) — Install and deploy in 5 minutes
- [Architecture](https://tow-cli.neurosam.ai/architecture) — How Tow works under the hood
- [Configuration](https://tow-cli.neurosam.ai/configuration) — Full `tow.yaml` reference
- [Commands](https://tow-cli.neurosam.ai/commands) — CLI reference
- [Security](https://tow-cli.neurosam.ai/security) — Secrets management and SSH hardening
- [Examples](https://tow-cli.neurosam.ai/examples) — Real-world deployment scenarios
- [Comparison](https://tow-cli.neurosam.ai/comparison) — Tow vs. Ansible, Capistrano, Kamal, and more

## AI Agent Integration

Tow is the first deployment tool with native AI agent support.

### MCP Server (Claude, Cursor, Windsurf)

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

Then ask your AI assistant: *"Check the status of api-server in prod"* or *"Roll back the API"*.

### Claude Code Skill

Copy [`integrations/claude-skill/tow-deploy.md`](integrations/claude-skill/tow-deploy.md) to `.claude/skills/` in your project.

### VS Code Extension

See [`integrations/vscode/`](integrations/vscode/) — sidebar UI with environments, modules, and deployment controls.

## Production-Tested

Tow has been validated on **real production infrastructure** at [Balkari Inc.](https://balkari.io), managing:

- **22 servers** across Spring Boot microservices, Kafka clusters, Redis, MongoDB, Prometheus, Grafana, and more
- **Multi-server operations**: status checks across 3-node Kafka cluster, multiplexed log viewing, parallel SSH execution
- **Legacy compatibility**: seamlessly manages existing deployments created by bash scripts (deploy_path `{module}-{server}`)

```
$ tow doctor -e prod -m api-server
  ✓ tow.yaml is valid
  ✓ Environment 'prod' exists
  ✓ SSH key exists
  ✓ Servers configured (22)
  ✓ SSH connection successful
  ✓ Remote dir exists
  ✓ Disk space — Available: 4.9G
  ✓ Branch policy
  ✓ No active deploy lock
  9 passed, 0 failed
```

## Current Status

Tow is at **v0.3** — production-tested and actively developed.

**Known limitations:**
- No resume for interrupted large file uploads
- No blue-green deployment strategy yet (rolling deploy is supported)
- IDE plugins (VS Code, JetBrains) are functional but early-stage

**Roadmap to v1.0:**
- Blue-green deployment strategy
- Web dashboard for deployment status
- Config encryption for sensitive values
- SSH interface abstraction for 80%+ test coverage

If you hit an issue, please [open a GitHub issue](https://github.com/neurosamAI/tow-cli/issues). We take bug reports seriously.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE) — Copyright (c) 2026-present [Murry Jeong (comchangs)](https://github.com/comchangs) and [neurosam.AI](https://neurosam.ai) (Neurosam AI Inc.)

---

<p align="center">
  <sub>Created by <a href="https://github.com/comchangs"><strong>Murry Jeong (comchangs)</strong></a> &middot; Supported by <a href="https://neurosam.ai"><strong>neurosam.AI</strong></a></sub>
</p>
