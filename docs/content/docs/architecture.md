---
title: Architecture
---

# Architecture

Tow is designed around a few core principles: **simplicity**, **atomicity**, and **zero dependencies on remote servers**.

## Design Philosophy

### Agentless

Tow operates entirely over SSH. Your servers need nothing installed — no agent, no runtime, no Docker daemon. If you can SSH in, you can deploy with Tow.

### Atomic Deployments via Symlinks

Every deployment creates a timestamped directory. The `current` symlink points to the active version. Switching versions (deploy or rollback) is a single atomic `ln -sfn` operation.

```
/app/api-server-1/
├── current → deploy/20240626-143022    ← atomic symlink
├── deploy/
│   ├── 20240626-143022/                ← latest (active)
│   │   ├── bin/
│   │   ├── conf/
│   │   ├── lib/
│   │   └── log → /app/api-server-1/log
│   ├── 20240625-091500/                ← previous
│   └── 20240624-170200/                ← older
├── upload/
├── log/                                ← persistent logs
├── conf/                               ← persistent configs
└── data/                               ← persistent data dirs
```

### Single Configuration File

Everything is defined in one `tow.yaml`. No inventory files, no playbooks, no Dockerfiles — just one file that describes your entire deployment topology.

## System Overview

```
┌────────────────────────────────────────────────────────────────┐
│                        CLI (cmd/tow)                           │
│   cobra commands: init, deploy, auto, start, stop, rollback..  │
└───────────────┬────────────────────────────────────────────────┘
                │
    ┌───────────┴───────────┐
    │                       │
    ▼                       ▼
┌─────────┐         ┌──────────────┐
│ Config  │         │ Initializer  │
│ Loader  │         │ (tow init)   │
└────┬────┘         └──────┬───────┘
     │                     │
     │  loads tow.yaml     │  detects project type
     │  + tow.local.yaml   │  generates config + scripts
     │  + env vars         │
     ▼                     ▼
┌──────────────────────────────────────────────────────┐
│                   Pipeline Engine                     │
│                                                      │
│  auto:   build → package → upload → install → start  │
│  deploy:          package → upload → install → start  │
└──────────────────────────┬───────────────────────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
              ▼                         ▼
    ┌──────────────┐          ┌──────────────┐
    │   Module      │          │    Deploy     │
    │   Handlers    │          │    Engine     │
    │               │          │               │
    │  springboot   │          │  upload (SCP) │
    │  java         │          │  install      │
    │  node         │          │  start/stop   │
    │  python       │          │  rollback     │
    │  go / rust    │          │  status       │
    │  kafka/redis  │          │  health check │
    │  generic      │          │  lock/unlock  │
    └──────────────┘          └───────┬───────┘
                                      │
                                      ▼
                              ┌──────────────┐
                              │  SSH Client   │
                              │               │
                              │  connection   │
                              │  pooling      │
                              │  key/agent/pw │
                              │  SCP / rsync  │
                              └──────────────┘
```

## Core Components

### Config Loader (`internal/config/`)

Responsible for loading and validating `tow.yaml`. Key capabilities:

- **Environment variable interpolation**: `${MY_VAR}` in any string value is resolved at load time
- **Local overrides**: `tow.local.yaml` is deep-merged over `tow.yaml` — ideal for developer-specific settings (SSH keys, local ports)
- **Defaults cascade**: Global defaults → environment-level → module-level → per-module SSH config
- **Validation**: Required fields, SSH key existence, port conflicts

```yaml
# tow.yaml (committed to git)
defaults:
  ssh_key_path: ~/.ssh/deploy.pem

# tow.local.yaml (in .gitignore)
defaults:
  ssh_key_path: ~/.ssh/my-personal-key.pem
```

### Pipeline Engine (`internal/pipeline/`)

Orchestrates multi-step deployment workflows as named pipelines:

| Pipeline | Steps |
|----------|-------|
| `auto` | `build` → `package` → `upload` → `install` → `stop` → `start` |
| `deploy` | `package` → `upload` → `install` → `stop` → `start` |

Each step:
- Logs progress with step counters (`[2/6] Packaging...`)
- Supports **dry-run mode** (shows commands without executing)
- Substitutes `${ENV}` and `${MODULE}` in build commands
- Triggers lifecycle hooks (`pre_build`, `post_build`, `pre_deploy`, `post_deploy`)

### Deploy Engine (`internal/deploy/`)

Handles all remote server operations:

| Operation | What It Does |
|-----------|-------------|
| `Setup` | Creates directory structure on remote |
| `Upload` | Transfers artifact via SCP |
| `Install` | Extracts archive, updates `current` symlink |
| `Start` / `Stop` | Runs control scripts on remote |
| `Status` | Reports PID, uptime, memory usage |
| `Rollback` | Switches symlink to previous/specified version |
| `Logs` | Streams remote log files with optional grep filter |
| `ListDeployments` | Shows deployment history with timestamps |

### Module Handlers (`internal/module/`)

Each module type provides sensible defaults:

| Handler | Build Command | Start | Health Check |
|---------|--------------|-------|-------------|
| **springboot** | `./gradlew :mod:bootJar` | `bin/server start` | HTTP `/actuator/health` |
| **java** | `./gradlew :mod:build` | `bin/server start` | TCP port |
| **node** | `npm ci && npm run build` | `node dist/main.js` | TCP port |
| **python** | `pip install -r requirements.txt` | `python -m app` | TCP port |
| **go** | `go build -o bin/mod` | `bin/mod` | TCP port |
| **rust** | `cargo build --release` | `bin/mod` | TCP port |
| **kafka** | — | `bin/kafka-server-start.sh` | TCP `:9092` |
| **redis** | — | `redis-server conf` | `redis-cli ping` |
| **generic** | (custom) | (custom) | TCP port |

You can override any default in `tow.yaml`.

### SSH Client (`internal/ssh/`)

Manages all remote connectivity:

- **Connection pooling**: Reuses SSH connections with keepalive verification
- **Authentication**: SSH key, SSH agent (`SSH_AUTH_SOCK`), or password
- **Per-module override**: Different SSH credentials per module (e.g., Kafka on a different user)
- **Host key verification**: Uses `~/.ssh/known_hosts` by default; `--insecure` flag to skip
- **Operations**: Command execution, streaming output, SCP upload, rsync for directories

### Initializer (`internal/initializer/`)

Powers the `tow init` command:

1. **Scans** the project directory for build files, frameworks, and configuration patterns
2. **Detects** project type with confidence scoring:
   - `pom.xml` + Spring Boot dependencies → `springboot` (high confidence)
   - `package.json` + NestJS → `node` with NestJS framework
   - `pyproject.toml` + FastAPI → `python` with FastAPI framework
3. **Discovers monorepo modules**: Reads `settings.gradle` or `pom.xml <modules>`
4. **Filters library modules**: Excludes modules ending in `-common`, `-core`, `-support`, etc.
5. **Generates** `tow.yaml` and per-module control scripts (`env.sh`, `server`)

### Deploy Locking (`internal/deploy/lock.go`)

Prevents concurrent deployments:

```
Server A: tow deploy → acquires lock (.tow-lock/info) → deploys → releases lock
Server B: tow deploy → lock exists → "Deploy in progress by user@host since..." → abort
```

- Lock created via atomic `mkdir` on the first target server
- Contains deployer info (user, timestamp, command)
- Auto-released on completion; `tow unlock` for manual release

### Branch Verification (`internal/deploy/branch.go`)

Ensures production safety:

```yaml
environments:
  prod:
    # Simple mode
    branch: main

    # Advanced mode
    branch_policy:
      allowed: [main, release/*, hotfix/*]
      commands: [deploy, auto, start, restart]
```

- Checks local git branch before executing commands
- Supports glob patterns for flexible branch policies
- Configurable per-command (e.g., allow `status` from any branch)

## Hierarchical Config Resolution

Configuration files are resolved in priority order:

```
config/{env}-{serverNum}/   ← Highest priority (server-specific)
config/{env}/               ← Environment-specific
config/                     ← Base configuration
```

This allows fine-grained customization:

```
config/
├── application.yml           ← shared defaults
├── dev/
│   └── application.yml       ← dev overrides
├── prod/
│   └── application.yml       ← prod overrides
└── prod-1/
    └── application.yml       ← prod server 1 specific
```

## Parallel Execution

When deploying to multiple servers, Tow executes operations concurrently:

```
tow deploy -e prod -m api-server    (2 servers configured)
    │
    ├── goroutine 1 → server prod-1 (52.78.100.1)
    │   └── upload → install → stop → start → health check
    │
    └── goroutine 2 → server prod-2 (52.78.100.2)
        └── upload → install → stop → start → health check
    │
    └── aggregate results → report success/failure
```

Results are aggregated — if any server fails, the entire operation is reported as failed.

## Lifecycle Hooks

Hooks allow custom actions at key points:

```
pre_build → build → post_build → pre_deploy → [upload → install → stop → start] → post_deploy
                                                          ↑              ↑
                                                     pre_start      post_start
                                                     pre_stop       post_stop
```

```yaml
modules:
  api-server:
    hooks:
      pre_build: "echo 'Running tests...' && ./gradlew test"
      post_deploy: "curl -X POST https://slack.webhook/deploy-notification"
      pre_stop: "curl -X POST http://localhost:8080/graceful-shutdown"
```
