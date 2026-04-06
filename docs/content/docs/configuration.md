---
title: Configuration
---

# Configuration Reference

Tow uses a single `tow.yaml` file to define your entire deployment topology. This page covers every configuration option.

## File Location

Tow looks for configuration in this order:

1. Path specified by `--config` flag
2. `tow.yaml` in the current directory
3. `tow.local.yaml` is merged on top (for local overrides)

> **Tip**: Add `tow.local.yaml` to `.gitignore`. Use it for developer-specific SSH key paths or custom settings that shouldn't be shared.

## Full Schema

### `project`

Top-level project metadata.

```yaml
project:
  name: my-project        # Required. Project name
  version: 1.0.0          # Project version
  base_dir: /app           # Base directory on remote servers (default: /app)
```

### `defaults`

Global defaults applied to all environments and modules.

```yaml
defaults:
  ssh_user: ec2-user          # Default SSH username (default: ec2-user)
  ssh_port: 22                # Default SSH port (default: 22)
  ssh_key_path: ~/.ssh/id_rsa # Path to SSH private key
  deploy_dir: deploy          # Deploy directory name (default: deploy)
  deploy_path: "{module}"     # Deploy path template (default: "{module}")
  log_dir: log                # Log subdirectory name (default: log)
  log_file: std.log           # Default log filename (default: std.log)
  health_check:
    type: tcp                  # tcp | http | log | command
    timeout: 300               # Total timeout in seconds (default: 300)
    interval: 5                # Check interval in seconds (default: 5)
    retries: 60                # Max retry count (default: 60)
```

### `environments`

Define target environments with their servers.

```yaml
environments:
  dev:
    ssh_user: ubuntu
    # ssh_key_path in tow.local.yaml (don't commit secrets)
    branch: develop
    variables:
      LOG_LEVEL: debug
    servers:
      # Named server — target with: tow deploy -s api-1
      - name: api-1
        host: 10.0.1.10
        modules: [api-server]
        labels:
          role: api

  prod:
    # ssh_key_path in tow.local.yaml
    branch_policy:
      allowed: [main, release/*, hotfix/*]
      commands: [deploy, auto, start, restart]
    servers:
      # Multiple servers with same config → hosts shorthand
      - name: api
        hosts: [52.78.100.1, 52.78.100.2]    # auto-expands to api-1, api-2
        modules: [api-server]

      # Servers with per-server config → individual entries
      - name: kafka-1
        host: 10.0.2.1
        modules: [kafka]
      - name: kafka-2
        host: 10.0.2.2
        modules: [kafka]
```

### Server naming

Servers are identified by `name` (recommended) or `number` (legacy):

```yaml
servers:
  # ✅ Recommended: use name
  - name: api-1
    host: 10.0.1.10
    modules: [api-server]

  # ✅ Shorthand: same config for multiple hosts
  - name: redis
    hosts: [10.0.3.1, 10.0.3.2, 10.0.3.3]  # → redis-1, redis-2, redis-3
    modules: [redis]

  # ⚠️ Legacy: number-based (still supported)
  - number: 1
    host: 10.0.1.10
```

Target a specific server by name:

```bash
tow deploy -e prod -m kafka -s kafka-2      # by name
tow logs -e prod -m api-server -s api-1     # by name
tow deploy -e prod -m api-server -s 1       # by number (legacy)
```

### Per-server configuration

Use hierarchical config directories with server names:

```
config/
├── application.yml              ← shared defaults
├── prod/
│   └── application.yml          ← prod overrides
├── prod-kafka-1/
│   └── server.properties        ← kafka-1 specific (broker.id=1)
├── prod-kafka-2/
│   └── server.properties        ← kafka-2 specific (broker.id=2)
└── prod-kafka-3/
    └── server.properties        ← kafka-3 specific (broker.id=3)
```

Resolution order: `config/{env}-{serverName}/` > `config/{env}/` > `config/`

#### `defaults.deploy_path`

Controls the remote directory name for each module instance.

| Value | Resolves to | Example |
|-------|-------------|---------|
| `"{module}"` (default) | Module name only | `/app/api-server/` |
| `"{module}-{server}"` | Module + server number (legacy) | `/app/api-server-1/` |

If you are migrating from a system that used numbered directories, set `deploy_path: "{module}-{server}"` to preserve compatibility.

#### `defaults.log_dir`

Name of the log subdirectory inside each module directory. Defaults to `"log"`. For example, with the default value, logs are stored at `/app/api-server/log/`.

#### `defaults.log_file`

Default log filename within `log_dir`. Defaults to `"std.log"`. This is used by `tow logs` when no explicit `log_path` is set on the module.

### `modules`

Define deployable services/applications.

```yaml
modules:
  api-server:
    type: springboot                   # Module type (see below)
    version: 3.2.1                     # Version pin (required for plugin modules)
    port: 8080                         # Application port
    build_cmd: ./gradlew :api-server:bootJar
    artifact_path: api-server/build/libs/*.jar
    start_cmd: bin/server start        # Custom start command
    stop_cmd: bin/server stop          # Custom stop command
    status_cmd: bin/server status      # Custom status command
    log_path: log/std.log              # Log file path (relative to module dir)
    deploy_dir: deploy                 # Override deploy directory
    config_dir: config                 # Config directory for hierarchical resolution

    package_includes:                  # Files/dirs to include in package
      - lib/
      - bin/
      - conf/

    package_layout:                    # Customizable package directory structure
      "script/":               "bin/"
      "build/libs/*.jar":      "lib/"
      "config/${ENV}/":        "conf/"

    data_dirs:                         # Persistent directories (survive deploys)
      - data
      - cache

    variables:                         # Module-level variables
      JAVA_OPTS: -Xms512m -Xmx1024m

    health_check:
      type: http                       # tcp | http | log | command
      target: http://localhost:8080/actuator/health
      timeout: 120
      interval: 3

    hooks:
      pre_build: ./gradlew test
      post_build: echo "Build complete"
      pre_deploy: echo "Starting deploy"
      post_deploy: "curl -X POST https://hooks.slack.com/services/xxx"
      post_install: |                  # Runs after extract + symlink (before stop/start)
        cd current && python -m venv .venv
        cd current && .venv/bin/pip install -r requirements.txt
      pre_start: ""
      post_start: ""
      pre_stop: ""
      post_stop: ""

    ssh:                               # Per-module SSH override
      user: app-deploy
      port: 2222
      auth: key                        # key | password | agent
      key_path: ~/.ssh/special-key.pem
```

#### `modules.{name}.version`

Version pinning for plugin modules (e.g., Kafka, Redis, PostgreSQL). When set, Tow downloads and installs that exact version. This is **required** for plugin modules to prevent accidental upgrades.

```yaml
modules:
  kafka:
    type: kafka
    version: 3.6.1    # Required — pinned version
    port: 9092
```

If `version` is omitted for a plugin module, Tow prints a warning:

```
⚠  WARNING: Module 'kafka' (type: kafka) has no version pin.
   Set 'version' in tow.yaml to prevent accidental upgrades.
```

For built-in application types (springboot, node, python, etc.), `version` is optional and purely informational.

## Module Types

### Built-in (Language/Framework)

| Type | Description | Default Build | Default Health Check |
|------|-------------|--------------|---------------------|
| `springboot` | Spring Boot applications | `./gradlew :mod:bootJar` | HTTP `/actuator/health` |
| `java` | Generic Java applications | `./gradlew :mod:build` | TCP port |
| `node` | Node.js applications | `npm ci && npm run build` | TCP port |
| `python` | Python applications | `pip install -r requirements.txt` | TCP port |
| `go` | Go applications | `go build -o bin/mod` | TCP port |
| `rust` | Rust applications | `cargo build --release` | TCP port |
| `php` | PHP applications | `composer install` | TCP port |
| `ruby` | Ruby applications | `bundle install` | TCP port |
| `dotnet` | C#/.NET applications | `dotnet publish` | TCP port |
| `kotlin` | Kotlin applications | `./gradlew build` | TCP port |
| `elixir` | Elixir applications | `mix release` | TCP port |
| `generic` | Custom applications | (must specify) | TCP port |

### YAML Plugins (Infrastructure — 35 services)

Infrastructure services are loaded from `plugins/` directory. No Go code needed.

| Category | Services |
|----------|----------|
| **Databases** | MySQL, PostgreSQL, MariaDB, MongoDB, ClickHouse, InfluxDB |
| **Message Brokers** | Kafka, RabbitMQ, ZooKeeper |
| **Kafka Ecosystem** | Kafka Connect, Kafka Lag Exporter, KMinion, CMAK |
| **Caching & Storage** | Redis, Memcached, MinIO, etcd |
| **Monitoring** | Prometheus, Grafana, Node Exporter, Kibana |
| **Logging** | Elasticsearch, Logstash, Fluentd, Loki, Promtail |
| **Web & Proxy** | Nginx, HAProxy |
| **Security** | Vault, Consul, Keycloak |
| **CI/CD** | Jenkins, SonarQube |
| **Data & Workflow** | Airflow, Superset |

Usage example:

```yaml
modules:
  my-db:
    type: postgresql     # Loaded from plugins/postgresql.yaml
    port: 5432

  my-cache:
    type: redis          # Loaded from plugins/redis.yaml
    port: 6379
```

All defaults can be overridden in `tow.yaml`. See [plugins/README.md](https://github.com/neurosamAI/tow-cli/tree/main/plugins) for full list and plugin authoring guide.

## Package Layout

The `package_layout` option lets you customize how files are arranged inside the deployment package. Each key is a source glob pattern (relative to the project root) and the value is the destination directory inside the package.

```yaml
modules:
  api-server:
    type: springboot
    package_layout:
      "script/":               "bin/"
      "build/libs/*.jar":      "lib/"
      "config/${ENV}/":        "conf/"
```

When `package_layout` is not set, Tow uses the default layout based on `package_includes`. When set, `package_layout` takes precedence and gives you full control over the mapping from source paths to package paths.

- Source patterns support globs (`*`, `**`) and environment variable interpolation (`${ENV}`)
- Trailing `/` on the source copies the directory contents; without it, the file is copied directly
- Destination paths are always relative to the package root

## Hooks

Tow supports lifecycle hooks at various stages of the deployment pipeline:

| Hook | When it runs | Execution |
|------|-------------|-----------|
| `pre_build` | Before build command | Local |
| `post_build` | After build command | Local |
| `pre_deploy` | Before deploy pipeline starts | Local |
| `post_deploy` | After deploy pipeline completes | Local |
| `post_install` | After extract + symlink, before stop/start | Remote (on each server) |
| `pre_start` | Before start command | Remote |
| `post_start` | After start command | Remote |
| `pre_stop` | Before stop command | Remote |
| `post_stop` | After stop command | Remote |

The `post_install` hook is particularly useful for environment setup that must happen after new code is in place but before the service restarts. Common uses include creating Python virtualenvs, running `pip install`, and executing database migrations.

## Health Check Types

### TCP

Checks if a port is accepting connections.

```yaml
health_check:
  type: tcp
  target: ":8080"          # Port to check (default: module port)
  timeout: 60
  interval: 3
```

### HTTP

Sends an HTTP GET and expects a 2xx response.

```yaml
health_check:
  type: http
  target: http://localhost:8080/actuator/health
  timeout: 120
  interval: 5
```

### Log

Watches a log file for a specific pattern.

```yaml
health_check:
  type: log
  target: "Started Application in"   # Pattern to grep for
  timeout: 120
  interval: 3
```

### Command

Runs a custom command; success (exit code 0) means healthy.

```yaml
health_check:
  type: command
  target: "redis-cli ping | grep PONG"
  timeout: 30
  interval: 5
```

## Environment Variable Interpolation

Use `${VAR_NAME}` syntax anywhere in `tow.yaml`:

```yaml
environments:
  prod:
    servers:
      - number: 1
        host: ${PROD_SERVER_1}    # Resolved from OS environment
      - number: 2
        host: ${PROD_SERVER_2}

modules:
  api-server:
    ssh:
      password: ${DEPLOY_PASSWORD}   # Secrets from environment
```

> **Security**: Never hardcode secrets in `tow.yaml`. Use environment variables or `tow.local.yaml` (in `.gitignore`).

## Retention Policy

Control how many old deployments are kept on remote servers.

```yaml
retention:
  keep: 5              # Keep the 5 most recent deployments (default: 5)
  auto_cleanup: true   # Automatically clean up after deploy/auto (default: false)
```

- The current active deployment is never removed
- Manual cleanup: `tow cleanup -e prod -m api-server --keep 3`
- Without `auto_cleanup`, old deployments accumulate until you run `tow cleanup`

## Notifications

Send deployment event notifications to Slack, Discord, or any webhook.

```yaml
notifications:
  - type: slack
    url: ${SLACK_WEBHOOK_URL}

  - type: discord
    url: ${DISCORD_WEBHOOK_URL}

  - type: webhook
    url: https://my-service.com/deploy-events
```

Events sent: `deploy_start`, `deploy_success`, `deploy_failed`, `auto_rollback`, `rollback_success`, `rollback_failed`.

Webhook payload format:

```json
{
  "project": "my-platform",
  "environment": "prod",
  "module": "api-server",
  "event": "deploy_success",
  "message": "deployment completed",
  "timestamp": "2026-03-26T14:30:00Z"
}
```

## Local Overrides (`tow.local.yaml`)

`tow.local.yaml` is deep-merged over `tow.yaml`. Perfect for:

- Developer-specific SSH keys
- Local port overrides
- Custom build commands for development

```yaml
# tow.local.yaml — in .gitignore
defaults:
  ssh_key_path: ~/.ssh/my-personal-key.pem

environments:
  dev:
    servers:
      - number: 1
        host: 192.168.1.100    # Local dev server
```

Merge rules:
- Scalar values are **replaced**
- Maps are **deep-merged**
- Lists are **replaced** (not appended)

## Hierarchical Config Directories

Tow supports hierarchical configuration resolution for your application config files:

```
config/
├── application.yml             ← base (all environments)
├── dev/
│   └── application.yml         ← dev environment override
├── prod/
│   └── application.yml         ← prod environment override
└── prod-1/
    └── application.yml         ← prod server-1 specific
```

Resolution order (highest priority first):

1. `config/{env}-{server_number}/`
2. `config/{env}/`
3. `config/`

When packaging, Tow copies the resolved config files to the `conf/` directory in the deployment archive.

## Complete Example

```yaml
project:
  name: my-saas
  version: 2.1.0
  base_dir: /app

defaults:
  ssh_user: ec2-user
  ssh_port: 22
  ssh_key_path: ~/.ssh/deploy.pem
  health_check:
    type: tcp
    timeout: 120
    interval: 5

environments:
  dev:
    ssh_key_path: ~/.ssh/dev.pem
    branch: develop
    variables:
      SPRING_PROFILES_ACTIVE: dev
    servers:
      - number: 1
        host: 10.0.1.10

  staging:
    ssh_key_path: ~/.ssh/staging.pem
    branch_policy:
      allowed: [develop, release/*]
    servers:
      - number: 1
        host: 10.0.2.10

  prod:
    ssh_key_path: ~/.ssh/prod.pem
    branch_policy:
      allowed: [main, hotfix/*]
      commands: [deploy, auto, start, restart, rollback]
    servers:
      - number: 1
        host: ${PROD_SERVER_1}
        modules: [api-server, batch-server]
      - number: 2
        host: ${PROD_SERVER_2}
        modules: [api-server]
      - number: 3
        host: ${PROD_SERVER_3}
        modules: [kafka, redis]

modules:
  api-server:
    type: springboot
    port: 8080
    build_cmd: ./gradlew :api-server:bootJar -x test
    artifact_path: api-server/build/libs/api-server-*.jar
    config_dir: config
    data_dirs: [uploads, cache]
    health_check:
      type: http
      target: http://localhost:8080/actuator/health
      timeout: 120
    hooks:
      pre_build: ./gradlew :api-server:test
      post_deploy: |
        curl -X POST ${SLACK_WEBHOOK} \
          -d '{"text":"api-server deployed to prod"}'

  batch-server:
    type: springboot
    port: 8081
    build_cmd: ./gradlew :batch-server:bootJar
    health_check:
      type: log
      target: "Started BatchApplication in"

  kafka:
    type: kafka
    port: 9092
    data_dirs: [kafka-logs]

  redis:
    type: redis
    port: 6379
    data_dirs: [redis-data]
```
