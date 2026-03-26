---
title: Examples
---

# Examples & Use Cases

Real-world deployment scenarios with complete configurations.

## Spring Boot Microservices

A typical backend with API server, batch processor, and infrastructure services.

```yaml
project:
  name: my-platform
  version: 1.0.0
  base_dir: /app

defaults:
  ssh_user: ec2-user
  ssh_key_path: ~/.ssh/deploy.pem
  health_check:
    timeout: 120
    interval: 5

environments:
  dev:
    ssh_key_path: ~/.ssh/dev.pem
    branch: develop
    servers:
      - number: 1
        host: 10.0.1.10

  prod:
    ssh_key_path: ~/.ssh/prod.pem
    branch_policy:
      allowed: [main, hotfix/*]
    servers:
      - number: 1
        host: 52.78.100.1
        modules: [api-server, admin-server]
      - number: 2
        host: 52.78.100.2
        modules: [api-server]
      - number: 3
        host: 52.78.100.3
        modules: [batch-server, kafka, redis]

modules:
  api-server:
    type: springboot
    port: 8080
    build_cmd: ./gradlew :api-server:bootJar -x test
    artifact_path: api-server/build/libs/api-server-*.jar
    config_dir: config
    health_check:
      type: http
      target: http://localhost:8080/actuator/health
    variables:
      JAVA_OPTS: -Xms512m -Xmx2048m -XX:+UseG1GC
    hooks:
      pre_build: ./gradlew :api-server:test
      post_deploy: |
        curl -X POST ${SLACK_WEBHOOK} \
          -H 'Content-type: application/json' \
          -d '{"text":"api-server deployed to prod"}'

  admin-server:
    type: springboot
    port: 8081
    build_cmd: ./gradlew :admin-server:bootJar
    health_check:
      type: http
      target: http://localhost:8081/actuator/health

  batch-server:
    type: springboot
    port: 8082
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

### Deploy Commands

```bash
# Deploy API server to all prod servers
tow auto -e prod -m api-server

# Deploy only to prod server 1
tow auto -e prod -m api-server -s 1

# Deploy all modules to dev
tow auto -e dev

# Check status across production
tow status -e prod -m api-server

# Rollback API server if something goes wrong
tow rollback -e prod -m api-server
```

## Node.js / NestJS Application

```yaml
project:
  name: my-api
  base_dir: /app

defaults:
  ssh_user: ubuntu
  ssh_key_path: ~/.ssh/deploy.pem

environments:
  staging:
    branch: develop
    servers:
      - number: 1
        host: 10.0.1.20

  prod:
    branch: main
    servers:
      - number: 1
        host: api-1.example.com
      - number: 2
        host: api-2.example.com

modules:
  api:
    type: node
    port: 3000
    build_cmd: npm ci && npm run build
    start_cmd: node dist/main.js
    artifact_path: build/api.tar.gz
    package_includes:
      - dist/
      - node_modules/
      - package.json
    health_check:
      type: http
      target: http://localhost:3000/health
      timeout: 60
    variables:
      NODE_ENV: production
      PM2_INSTANCES: 4
```

## Python / FastAPI Application

```yaml
project:
  name: ml-service
  base_dir: /opt/services

defaults:
  ssh_user: deploy
  ssh_key_path: ~/.ssh/ml-deploy.pem

environments:
  prod:
    branch: main
    servers:
      - number: 1
        host: gpu-server-1.internal
      - number: 2
        host: gpu-server-2.internal

modules:
  inference-api:
    type: python
    port: 8000
    build_cmd: pip install -r requirements.txt && python -m pytest
    start_cmd: |
      cd current && \
      gunicorn app.main:app \
        --workers 4 \
        --worker-class uvicorn.workers.UvicornWorker \
        --bind 0.0.0.0:8000
    package_includes:
      - app/
      - requirements.txt
      - models/
    health_check:
      type: http
      target: http://localhost:8000/health
      timeout: 180    # Models take time to load
    data_dirs:
      - model-cache
      - uploads
```

## Go Microservice

```yaml
project:
  name: gateway
  base_dir: /opt/services

defaults:
  ssh_user: deploy
  ssh_key_path: ~/.ssh/deploy.pem

environments:
  prod:
    branch: main
    servers:
      - number: 1
        host: edge-1.example.com
      - number: 2
        host: edge-2.example.com

modules:
  gateway:
    type: go
    port: 8080
    build_cmd: CGO_ENABLED=0 GOOS=linux go build -o bin/gateway ./cmd/gateway
    start_cmd: bin/gateway --config conf/config.yaml
    artifact_path: bin/gateway
    package_includes:
      - bin/gateway
      - conf/
    health_check:
      type: http
      target: http://localhost:8080/healthz
      timeout: 30
```

## Multi-Environment with Hierarchical Config

For projects that need different configuration per environment and per server:

```
config/
├── application.yml              # Shared defaults
├── dev/
│   └── application.yml          # Dev overrides (debug logging, local DB)
├── staging/
│   └── application.yml          # Staging (test DB, reduced resources)
├── prod/
│   └── application.yml          # Prod (production DB, full resources)
├── prod-1/
│   └── application.yml          # Prod server 1 (primary node config)
└── prod-2/
    └── application.yml          # Prod server 2 (secondary node config)
```

```yaml
modules:
  api-server:
    type: springboot
    port: 8080
    config_dir: config    # Enable hierarchical config resolution
```

When deploying `api-server` to `prod` server `1`:
1. `config/application.yml` is copied (base)
2. `config/prod/application.yml` is overlaid (environment)
3. `config/prod-1/application.yml` is overlaid (server-specific)

## CI/CD Integration

### GitHub Actions

```yaml
name: Deploy
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Install Tow
        run: go install github.com/neurosamAI/tow-cli/cmd/tow@latest

      - name: Setup SSH Key
        run: |
          mkdir -p ~/.ssh
          echo "${{ secrets.SSH_PRIVATE_KEY }}" > ~/.ssh/deploy.pem
          chmod 600 ~/.ssh/deploy.pem
          ssh-keyscan ${{ secrets.PROD_SERVER_1 }} >> ~/.ssh/known_hosts
          ssh-keyscan ${{ secrets.PROD_SERVER_2 }} >> ~/.ssh/known_hosts

      - name: Deploy to Production
        env:
          PROD_SERVER_1: ${{ secrets.PROD_SERVER_1 }}
          PROD_SERVER_2: ${{ secrets.PROD_SERVER_2 }}
        run: tow auto -e prod -m api-server

      - name: Verify Deployment
        run: tow status -e prod -m api-server

      - name: Cleanup
        if: always()
        run: rm -f ~/.ssh/deploy.pem
```

### GitLab CI

```yaml
deploy-prod:
  stage: deploy
  image: golang:1.21
  only:
    - main
  before_script:
    - go install github.com/neurosamAI/tow-cli/cmd/tow@latest
    - mkdir -p ~/.ssh
    - echo "$SSH_PRIVATE_KEY" > ~/.ssh/deploy.pem
    - chmod 600 ~/.ssh/deploy.pem
    - ssh-keyscan $PROD_SERVER_1 >> ~/.ssh/known_hosts
  script:
    - tow auto -e prod -m api-server
    - tow status -e prod -m api-server
  after_script:
    - rm -f ~/.ssh/deploy.pem
```

## Gradual Rollout (Server-by-Server)

Deploy to one server at a time and verify before continuing:

```bash
# Deploy to server 1 first
tow auto -e prod -m api-server -s 1

# Verify it's healthy
tow status -e prod -m api-server -s 1
tow logs -e prod -m api-server -s 1 -f "ERROR"

# If looks good, deploy to server 2
tow auto -e prod -m api-server -s 2

# Verify
tow status -e prod -m api-server -s 2
```

If something goes wrong at any step:

```bash
# Instant rollback on the affected server
tow rollback -e prod -m api-server -s 1
```

## Infrastructure Services (28 Plugins)

Tow manages infrastructure alongside application services through YAML plugins. No Go code needed — just set the `type` to a plugin name.

```yaml
modules:
  # Message broker
  kafka:
    type: kafka
    port: 9092
    data_dirs: [kafka-logs]

  # Cache
  redis:
    type: redis
    port: 6379
    data_dirs: [redis-data]

  # Database
  postgres:
    type: postgresql
    port: 5432
    data_dirs: [pgdata]

  # Monitoring
  prometheus:
    type: prometheus
    port: 9090

  grafana:
    type: grafana
    port: 3000

  # Search
  elasticsearch:
    type: elasticsearch
    port: 9200
    data_dirs: [es-data]

  # Secret management
  vault:
    type: vault
    port: 8200
```

```bash
# Deploy infrastructure
tow auto -e prod -m kafka
tow auto -e prod -m redis
tow auto -e prod -m postgres
tow status -e prod          # Check everything

# Deploy Redis
tow auto -e prod -m redis -s 1
tow status -e prod -m redis
```

## Rolling Deploy with Auto-Rollback

The safest way to deploy to production — one server at a time, with automatic revert if anything goes wrong:

```bash
# Deploy one server at a time, verify health before moving to next
tow auto -e prod -m api-server --rolling --auto-rollback -y
```

What happens:
1. Build and package locally
2. Upload to all servers
3. Install on all servers
4. Stop all servers
5. **Start server-1** → wait for health check → pass ✓
6. **Start server-2** → wait for health check → pass ✓
7. If any server fails health check → **auto-rollback all servers** to previous version

## Notifications (Slack / Discord / Webhook)

Get notified about deployment events:

```yaml
# In tow.yaml
notifications:
  - type: slack
    url: ${SLACK_WEBHOOK_URL}
  - type: webhook
    url: https://my-monitoring.com/deploy-events
```

Events sent automatically: `deploy_start`, `deploy_success`, `deploy_failed`, `auto_rollback`.

## Auto-Cleanup with Retention Policy

Keep disk space under control:

```yaml
# In tow.yaml
retention:
  keep: 5              # Keep 5 most recent deployments
  auto_cleanup: true   # Clean up automatically after each deploy
```

Or manual cleanup:

```bash
tow cleanup -e prod -m api-server --keep 3
```

## AI Agent Integration

### Claude Code / MCP

Initialize with AI integration:

```bash
tow init --with-ai
```

This generates:
- `.claude/skills/tow-deploy.md` — Claude Code understands Tow commands
- `.claude/settings.json` — MCP server auto-configured

Then ask Claude: *"Deploy api-server to staging"* or *"Check prod status and show me error logs"*.

### Manual MCP Setup

```json
{
  "mcpServers": {
    "tow": {
      "command": "tow",
      "args": ["mcp-server"]
    }
  }
}
```

## Debugging Production Issues

```bash
# Check status on all prod servers (human-readable)
tow status -e prod -m api-server

# Machine-readable for scripts/CI
tow status -e prod -m api-server -o json

# Stream logs with error filter
tow logs -e prod -m api-server -f "ERROR"
tow logs -e prod -m api-server -f "OutOfMemoryError"

# Download logs to local for deeper analysis
tow download -e prod -m api-server logs/std.log

# SSH into a specific server
tow login -e prod -m api-server -s 1

# List all deployed versions
tow list deployments -e prod -m api-server

# Rollback to a known-good version
tow rollback -e prod -m api-server -t 20240625-091500
```
