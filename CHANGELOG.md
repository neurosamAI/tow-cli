# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-03-30

### Added

**New Commands**
- `tow ssh` — execute ad-hoc commands on remote servers without interactive sessions (`tow ssh -e prod -m kafka --all -- "free -h"`)
- `tow diff` — compare deployed code on remote servers against local build artifacts before deploying
- `tow config server add/remove/list` — manage servers in `tow.yaml` from the command line
- `tow config module add/remove/list` — manage modules in `tow.yaml` from the command line
- `tow config assign/unassign` — assign and unassign modules to servers from the command line

**Enhanced `tow logs`**
- `--all` flag — stream logs from all servers simultaneously
- Comma-separated server targeting (`-s kafka-1,kafka-3`)
- `-F` flag — follow mode for live tailing of log output
- Log presets: `--save-preset`, `--preset`, `--list-presets`, `--delete-preset` — save, replay, and manage frequently used log queries

**Configuration**
- `defaults.deploy_path` — configurable deploy path template (`"{module}"` default or `"{module}-{server}"` for legacy layouts)
- `defaults.log_dir` — configurable log subdirectory name (default: `"log"`)
- `defaults.log_file` — configurable default log filename (default: `"std.log"`)
- `modules.{name}.version` — version pinning for plugin modules with warning when omitted

---

## [0.1.0] - 2026-03-26

### Added

- Initial release of Tow by [Murry Jeong (comchangs)](https://github.com/comchangs), supported by [neurosam.AI](https://neurosam.ai)

**Core Deployment**
- `tow init` — auto-detect project type, framework, build tool, and monorepo modules
- `tow deploy` / `tow auto` — full deployment pipelines with parallel execution
- `tow rollback` — instant rollback via symlink switching
- `tow start` / `tow stop` / `tow restart` — service lifecycle management
- `tow status` — check process PID, uptime, memory, current deployment
- `tow logs` — stream remote logs with grep filter
- `tow setup` — initialize remote server directory structure
- `tow upload` / `tow install` — manual upload and install steps
- `tow login` — interactive SSH session to servers
- `tow unlock` — force release deploy locks
- `tow list` — list modules, environments, or deployment history

**Module Handlers**
- 12 built-in types: springboot, java, node, python, go, rust, php, ruby, dotnet, kotlin, elixir, generic
- Sensible defaults for build commands, start/stop scripts, health checks, and artifacts
- Project auto-detection: 10 languages, 40+ frameworks, build tools, monorepo sub-modules
- 35 YAML infrastructure plugins: databases (MySQL, PostgreSQL, MariaDB, MongoDB, ClickHouse, InfluxDB), message brokers (Kafka, RabbitMQ, ZooKeeper), Kafka ecosystem (Kafka Connect, Kafka Lag Exporter, KMinion, CMAK), caching (Redis, Memcached, MinIO, etcd), monitoring (Prometheus, Grafana, Node Exporter, Kibana), logging (Elasticsearch, Logstash, Fluentd, Loki, Promtail), web/proxy (Nginx, HAProxy), security (Vault, Consul, Keycloak), CI/CD (Jenkins, SonarQube), data (Airflow, Superset)

**Safety & Reliability**
- `--rolling` flag — deploy one server at a time with health check gates
- `--auto-rollback` flag — automatically revert if health check fails after start
- Production confirmation prompt — `Type 'yes' to confirm` for prod/production/live environments (`-y` to skip)
- Branch policies with glob pattern support (`main`, `release/*`, `hotfix/*`)
- Deploy locking to prevent concurrent deployments
- SSH connection retry (3 attempts with 2s backoff)
- Command execution timeout (10min default, SIGKILL on timeout)
- Graceful shutdown with SIGKILL fallback if process doesn't stop

**Configuration**
- Single `tow.yaml` with environment variable interpolation (`${VAR}`)
- Local overrides via `tow.local.yaml`
- Hierarchical config resolution: `config/{env}-{server}/ > config/{env}/ > config/`
- Module-level variables substituted in build commands (`${JAVA_OPTS}`, etc.)
- Per-module SSH configuration overrides

**Operations**
- `tow cleanup` — remove old deployment directories with configurable retention
- `tow download` — SCP files from remote servers to local
- `tow provision` — set up new servers (timezone, locale, JRE, essential tools)
- `--output json` flag for `status` and `list deployments` — machine-readable output
- Parallel execution across multiple servers via goroutines

**Notifications**
- Built-in Slack, Discord, and webhook notifications on deploy events
- Configurable via `notifications:` in tow.yaml

**Retention**
- `retention.keep` — number of deployments to keep per module (default: 5)
- `retention.auto_cleanup` — automatically clean up after deploy/auto

**Health Checks**
- 4 types: HTTP, TCP, log pattern, custom command
- Configurable timeout, interval, and retries

**Lifecycle Hooks**
- pre/post for: build, deploy, start, stop

**AI Agent Integration**
- MCP Server (`tow mcp-server`) — expose deployment tools to Claude, Cursor, Windsurf
- Claude Code Agent Skill — natural language deployment management
- 7 MCP tools: status, deploy, rollback, logs, list deployments/modules/environments

**IDE Integration**
- VS Code Extension scaffold — sidebar, command palette, status bar
- JetBrains Plugin plan — tool window, run configurations

**Distribution**
- Homebrew (`brew install neurosamAI/tap/tow`)
- npm (`npm install -g @neurosamai/tow`)
- Debian package (`.deb`)
- Go install / binary download
- Cross-compilation: Linux + macOS, amd64 + arm64

**CI/CD**
- GitHub Actions: CI (test + lint + build), Release (auto binary + Homebrew + npm), Pages (docs)
- GitHub issue/PR templates, FUNDING.yml

---

<p align="center">
  <sub>Created by <a href="https://github.com/comchangs">comchangs</a> &middot; <a href="https://neurosam.ai">neurosam.AI</a></sub>
</p>
