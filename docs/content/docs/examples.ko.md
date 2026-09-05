---
title: 예제
---

# 예제 & 사용 사례

완전한 설정과 함께 살펴보는 실전 배포 시나리오입니다.

## Spring Boot 마이크로서비스

API 서버, 배치 프로세서, 인프라 서비스로 구성된 전형적인 백엔드입니다.

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

### 배포 명령어

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

## Node.js / NestJS 애플리케이션

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

## Python / FastAPI 애플리케이션

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

## Go 마이크로서비스

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

## 커스텀 패키지 레이아웃

`package_layout`을 사용해서 배포 패키지 안에 파일이 배치되는 방식을 정확히 제어할 수 있습니다. 레거시 호환이나 비표준 프로젝트 구조에 유용합니다.

### NSM 레거시 호환

`bin/`, `lib/`, `conf/` 구조를 기대하는 레거시 셸 스크립트 배포에서 마이그레이션합니다:

```yaml
modules:
  nsm-api:
    type: springboot
    port: 8080
    build_cmd: ./gradlew :nsm-api:bootJar
    package_layout:
      "script/nsm-api/":              "bin/"
      "nsm-api/build/libs/*.jar":     "lib/"
      "config/${ENV}/":               "conf/"
      "config/logback-${ENV}.xml":    "conf/"
    health_check:
      type: http
      target: http://localhost:8080/actuator/health
```

결과 패키지는 다음과 같은 모습입니다:

```
nsm-api.tar.gz
├── bin/
│   ├── env.sh
│   └── server
├── lib/
│   └── nsm-api-1.2.3.jar
└── conf/
    ├── application.yml
    └── logback-prod.xml
```

### 커스텀 레이아웃을 사용한 Node.js

Next.js standalone 빌드를 깔끔한 디렉토리 구조로 패키징합니다:

```yaml
modules:
  web-app:
    type: node
    port: 3000
    build_cmd: npm ci && npm run build
    package_layout:
      ".next/standalone/":            "./"
      ".next/static/":                ".next/static/"
      "public/":                      "public/"
    hooks:
      post_install: |
        cd current && npm rebuild --arch=x64 --platform=linux
    health_check:
      type: http
      target: http://localhost:3000/api/health
```

## 계층형 설정을 사용한 다중 환경

환경별, 서버별로 다른 설정이 필요한 프로젝트를 위한 예제입니다:

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

`api-server`를 `prod` 서버 `1`에 배포할 때:
1. `config/application.yml`이 복사됩니다 (base)
2. `config/prod/application.yml`이 오버레이됩니다 (environment)
3. `config/prod-1/application.yml`이 오버레이됩니다 (server-specific)

## CI/CD 연동

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

## 점진적 롤아웃 (서버별)

한 번에 서버 하나씩 배포하고 계속 진행하기 전에 확인합니다:

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

어느 단계에서든 문제가 생기면:

```bash
# Instant rollback on the affected server
tow rollback -e prod -m api-server -s 1
```

## 인프라 서비스 (28개 플러그인)

Tow는 YAML 플러그인을 통해 애플리케이션 서비스와 함께 인프라도 관리합니다. Go 코드가 필요 없습니다 — `type`을 플러그인 이름으로 설정하기만 하면 됩니다.

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

## 자동 롤백을 사용한 롤링 배포

프로덕션에 배포하는 가장 안전한 방법 — 한 번에 서버 하나씩, 문제가 생기면 자동으로 되돌립니다:

```bash
# Deploy one server at a time, verify health before moving to next
tow auto -e prod -m api-server --rolling --auto-rollback -y
```

동작 과정:
1. 로컬에서 빌드하고 패키징
2. 모든 서버에 업로드
3. 모든 서버에 설치
4. 모든 서버 중지
5. **server-1 시작** → 헬스체크 대기 → 통과 ✓
6. **server-2 시작** → 헬스체크 대기 → 통과 ✓
7. 어떤 서버든 헬스체크에 실패하면 → 모든 서버를 이전 버전으로 **자동 롤백**

## 알림 (Slack / Discord / 웹훅)

배포 이벤트에 대한 알림을 받으세요:

```yaml
# In tow.yaml
notifications:
  - type: slack
    url: ${SLACK_WEBHOOK_URL}
  - type: webhook
    url: https://my-monitoring.com/deploy-events
```

자동으로 전송되는 이벤트: `deploy_start`, `deploy_success`, `deploy_failed`, `auto_rollback`.

## 보존 정책을 사용한 자동 정리

디스크 공간을 관리하세요:

```yaml
# In tow.yaml
retention:
  keep: 5              # Keep 5 most recent deployments
  auto_cleanup: true   # Clean up automatically after each deploy
```

또는 수동으로 정리:

```bash
tow cleanup -e prod -m api-server --keep 3
```

## AI 에이전트 연동

### Claude Code / MCP

AI 연동과 함께 초기화하세요:

```bash
tow init --with-ai
```

다음이 생성됩니다:
- `.claude/skills/tow-deploy.md` — Claude Code가 Tow 명령어를 이해하게 됩니다
- `.claude/settings.json` — MCP 서버가 자동으로 설정됩니다

그런 다음 Claude에게 물어보세요: *"api-server를 staging에 배포해줘"* 또는 *"prod 상태를 확인하고 에러 로그를 보여줘"*.

### 수동 MCP 설정

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

## 프리셋을 사용한 멀티서버 로그 조회

클러스터 전체를 디버깅할 때, 여러 서버의 로그를 한 번에 보면 시간이 절약됩니다:

```bash
# View logs from all Kafka brokers simultaneously
tow logs -e prod -m kafka --all

# View logs from specific brokers only
tow logs -e prod -m kafka -s kafka-1,kafka-3

# Follow logs in real time with an error filter
tow logs -e prod -m kafka --all -F -f "ERROR"

# Save a frequently used log query as a preset
tow logs -e prod -m api-server -f "OutOfMemoryError" --save-preset oom-check

# Replay the saved preset later
tow logs --preset oom-check

# Manage presets
tow logs --list-presets
tow logs --delete-preset oom-check
```

## 멀티모듈 로그 조회

관련 서비스를 함께 모니터링하세요:

```bash
tow logs -e prod -m noriter-kafka,noriter-zookeeper --all -n 3
```

```
[noriter-kafka/kafka-1]      2026-03-30 14:35:49 GC Pause Young 765M→702M 17ms
[noriter-kafka/kafka-2]      2026-03-30 14:35:55 GC Pause Young 340M→292M 18ms
[noriter-zookeeper/zk-1]     2026-03-30 14:36:01 Session established
```

## 배포 메트릭

```bash
tow metrics -e prod --days 7
```

```
Deployments (last 7 days):
  Total:        7

By action:
  deploy        4
  auto          2
  rollback      1

By module:
  api-server    ████████████████████ 4
  admin         █████ 1
  kafka         █████ 1
  market        █████ 1
```

## `tow ssh`로 애드혹 명령 실행

인터랙티브 세션을 열지 않고 여러 서버에서 명령을 실행합니다:

```bash
# Check memory on all Kafka brokers
tow ssh -e prod -m kafka --all -- "free -h"

# Check disk usage on API servers
tow ssh -e prod -m api-server --all -- "df -h"

# View Java version on a specific server
tow ssh -e prod -m api-server -s 1 -- "java -version"

# Restart a system service (requires appropriate permissions)
tow ssh -e prod -m kafka -s kafka-2 -- "sudo systemctl restart kafka"

# Run on selected servers
tow ssh -e prod -m kafka -s kafka-1,kafka-3 -- "cat /etc/os-release"
```

## `tow config`로 설정 관리

CLI에서 직접 서버, 모듈, 할당을 추가하고 제거합니다:

```bash
# Add a new server to the prod environment
tow config server add -e prod --name api-3 --host 10.0.1.30

# List all servers in prod
tow config server list -e prod

# Add a new module
tow config module add --name payment-api --type springboot --port 8083

# Assign the new module to the new server
tow config assign -e prod --server api-3 --module payment-api

# Verify the assignment
tow config server list -e prod

# Remove the assignment and clean up
tow config unassign -e prod --server api-3 --module payment-api
tow config server remove -e prod --name api-3
tow config module remove --name payment-api
```

## `tow diff`로 배포된 버전과 로컬 비교

배포하기 전에 원격 서버에서 정확히 무엇이 바뀔지 확인하세요:

```bash
# Compare local build against what's currently deployed
tow diff -e prod -m api-server

# Check a specific server
tow diff -e prod -m api-server -s 1
```

출력 예시:

```
Comparing api-server: local vs prod/api-1 (deployed 2026-03-28T10:15:00Z)

  Modified:  lib/api-server-1.0.1.jar  (size: 42.1MB → 42.3MB)
  Added:     conf/new-feature.properties
  Unchanged: bin/server, conf/application.yml (2 files)

Summary: 1 modified, 1 added, 2 unchanged
```

예상한 변경 사항만 배포되는지 확인해서, 프로덕션에 도달하기 전에 예상치 못한 것들을 잡아낼 수 있습니다.

## 프로덕션 이슈 디버깅

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
