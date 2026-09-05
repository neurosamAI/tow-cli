---
title: 설정
---

# 설정 레퍼런스

Tow는 단일 `tow.yaml` 파일로 배포 토폴로지 전체를 정의합니다. 이 페이지는 모든 설정 옵션을 다룹니다.

## 파일 위치

Tow는 다음 순서로 설정을 찾습니다:

1. `--config` 플래그로 지정된 경로
2. 현재 디렉토리의 `tow.yaml`
3. 로컬 오버라이드를 위해 `tow.local.yaml`이 그 위에 병합됩니다

> **팁**: `tow.local.yaml`을 `.gitignore`에 추가하세요. 공유하면 안 되는 개발자별 SSH 키 경로나 커스텀 설정에 사용하세요.

## 전체 스키마

### `project`

최상위 프로젝트 메타데이터입니다.

```yaml
project:
  name: my-project        # Required. Project name
  version: 1.0.0          # Project version
  base_dir: /app           # Base directory on remote servers (default: /app)
```

### `defaults`

모든 환경과 모듈에 적용되는 전역 기본값입니다.

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

서버와 함께 대상 환경을 정의합니다.

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

### 서버 이름 지정

서버는 `name`(권장) 또는 `number`(레거시)로 식별됩니다:

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

이름으로 특정 서버를 지정합니다:

```bash
tow deploy -e prod -m kafka -s kafka-2      # by name
tow logs -e prod -m api-server -s api-1     # by name
tow deploy -e prod -m api-server -s 1       # by number (legacy)
```

### 서버별 설정

서버 이름과 함께 계층형 설정 디렉토리를 사용합니다:

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

해석 순서: `config/{env}-{serverName}/` > `config/{env}/` > `config/`

#### `defaults.deploy_path`

각 모듈 인스턴스의 원격 디렉토리 이름을 제어합니다.

| 값 | 해석 결과 | 예시 |
|-------|-------------|---------|
| `"{module}"` (기본값) | 모듈 이름만 | `/app/api-server/` |
| `"{module}-{server}"` | 모듈 + 서버 번호 (레거시) | `/app/api-server-1/` |

번호 기반 디렉토리를 쓰던 시스템에서 마이그레이션하는 경우, 호환성을 유지하려면 `deploy_path: "{module}-{server}"`로 설정하세요.

#### `defaults.log_dir`

각 모듈 디렉토리 안의 로그 서브디렉토리 이름입니다. 기본값은 `"log"`입니다. 예를 들어 기본값을 사용하면 로그는 `/app/api-server/log/`에 저장됩니다.

#### `defaults.log_file`

`log_dir` 안의 기본 로그 파일명입니다. 기본값은 `"std.log"`입니다. 모듈에 명시적인 `log_path`가 설정되지 않았을 때 `tow logs`가 이 값을 사용합니다.

### `modules`

배포 가능한 서비스/애플리케이션을 정의합니다.

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

플러그인 모듈(예: Kafka, Redis, PostgreSQL)의 버전을 고정합니다. 설정하면 Tow는 정확히 그 버전을 다운로드해서 설치합니다. 의도치 않은 업그레이드를 막기 위해 플러그인 모듈에는 **필수**입니다.

```yaml
modules:
  kafka:
    type: kafka
    version: 3.6.1    # Required — pinned version
    port: 9092
```

플러그인 모듈에 `version`을 생략하면 Tow가 경고를 출력합니다:

```
⚠  WARNING: Module 'kafka' (type: kafka) has no version pin.
   Set 'version' in tow.yaml to prevent accidental upgrades.
```

내장 애플리케이션 타입(springboot, node, python 등)의 경우 `version`은 선택 사항이며 단순 정보 제공용입니다.

## 모듈 타입

### 내장 (언어/프레임워크)

| 타입 | 설명 | 기본 빌드 | 기본 헬스체크 |
|------|-------------|--------------|---------------------|
| `springboot` | Spring Boot 애플리케이션 | `./gradlew :mod:bootJar` | HTTP `/actuator/health` |
| `java` | 범용 Java 애플리케이션 | `./gradlew :mod:build` | TCP 포트 |
| `node` | Node.js 애플리케이션 | `npm ci && npm run build` | TCP 포트 |
| `python` | Python 애플리케이션 | `pip install -r requirements.txt` | TCP 포트 |
| `go` | Go 애플리케이션 | `go build -o bin/mod` | TCP 포트 |
| `rust` | Rust 애플리케이션 | `cargo build --release` | TCP 포트 |
| `php` | PHP 애플리케이션 | `composer install` | TCP 포트 |
| `ruby` | Ruby 애플리케이션 | `bundle install` | TCP 포트 |
| `dotnet` | C#/.NET 애플리케이션 | `dotnet publish` | TCP 포트 |
| `kotlin` | Kotlin 애플리케이션 | `./gradlew build` | TCP 포트 |
| `elixir` | Elixir 애플리케이션 | `mix release` | TCP 포트 |
| `generic` | 커스텀 애플리케이션 | (직접 지정 필요) | TCP 포트 |

### YAML 플러그인 (인프라 — 35개 서비스)

인프라 서비스는 `plugins/` 디렉토리에서 로드됩니다. Go 코드가 필요 없습니다.

| 카테고리 | 서비스 |
|----------|----------|
| **데이터베이스** | MySQL, PostgreSQL, MariaDB, MongoDB, ClickHouse, InfluxDB |
| **메시지 브로커** | Kafka, RabbitMQ, ZooKeeper |
| **Kafka 생태계** | Kafka Connect, Kafka Lag Exporter, KMinion, CMAK |
| **캐싱 & 스토리지** | Redis, Memcached, MinIO, etcd |
| **모니터링** | Prometheus, Grafana, Node Exporter, Kibana |
| **로깅** | Elasticsearch, Logstash, Fluentd, Loki, Promtail |
| **웹 & 프록시** | Nginx, HAProxy |
| **보안** | Vault, Consul, Keycloak |
| **CI/CD** | Jenkins, SonarQube |
| **데이터 & 워크플로우** | Airflow, Superset |

사용 예시:

```yaml
modules:
  my-db:
    type: postgresql     # Loaded from plugins/postgresql.yaml
    port: 5432

  my-cache:
    type: redis          # Loaded from plugins/redis.yaml
    port: 6379
```

모든 기본값은 `tow.yaml`에서 오버라이드할 수 있습니다. 전체 목록과 플러그인 작성 가이드는 [plugins/README.md](https://github.com/neurosamAI/tow-cli/tree/main/plugins)를 참고하세요.

## 패키지 레이아웃

`package_layout` 옵션을 사용하면 배포 패키지 안에 파일이 배치되는 방식을 커스터마이징할 수 있습니다. 각 키는 (프로젝트 루트 기준) 소스 glob 패턴이고, 값은 패키지 안의 대상 디렉토리입니다.

```yaml
modules:
  api-server:
    type: springboot
    package_layout:
      "script/":               "bin/"
      "build/libs/*.jar":      "lib/"
      "config/${ENV}/":        "conf/"
```

`package_layout`을 설정하지 않으면 Tow는 `package_includes`를 기반으로 한 기본 레이아웃을 사용합니다. 설정하면 `package_layout`이 우선 적용되어 소스 경로에서 패키지 경로로의 매핑을 완전히 제어할 수 있습니다.

- 소스 패턴은 glob(`*`, `**`)과 환경 변수 보간(`${ENV}`)을 지원합니다
- 소스 끝에 `/`가 있으면 디렉토리 내용을 복사하고, 없으면 파일을 그대로 복사합니다
- 대상 경로는 항상 패키지 루트를 기준으로 합니다

## 훅

Tow는 배포 파이프라인의 다양한 단계에서 라이프사이클 훅을 지원합니다:

| 훅 | 실행 시점 | 실행 위치 |
|------|-------------|-----------|
| `pre_build` | 빌드 명령 실행 전 | 로컬 |
| `post_build` | 빌드 명령 실행 후 | 로컬 |
| `pre_deploy` | 배포 파이프라인 시작 전 | 로컬 |
| `post_deploy` | 배포 파이프라인 완료 후 | 로컬 |
| `post_install` | 추출 + 심링크 적용 후, stop/start 이전 | 원격 (각 서버에서) |
| `pre_start` | start 명령 실행 전 | 원격 |
| `post_start` | start 명령 실행 후 | 원격 |
| `pre_stop` | stop 명령 실행 전 | 원격 |
| `post_stop` | stop 명령 실행 후 | 원격 |

`post_install` 훅은 새 코드가 배치된 후, 서비스가 재시작되기 전에 이루어져야 하는 환경 설정 작업에 특히 유용합니다. 주로 Python 가상환경 생성, `pip install` 실행, 데이터베이스 마이그레이션 실행 등에 사용됩니다.

## 헬스체크 타입

### TCP

포트가 연결을 받아들이는지 확인합니다.

```yaml
health_check:
  type: tcp
  target: ":8080"          # Port to check (default: module port)
  timeout: 60
  interval: 3
```

### HTTP

HTTP GET을 보내고 2xx 응답을 기대합니다.

```yaml
health_check:
  type: http
  target: http://localhost:8080/actuator/health
  timeout: 120
  interval: 5
```

### Log

로그 파일에서 특정 패턴을 감시합니다.

```yaml
health_check:
  type: log
  target: "Started Application in"   # Pattern to grep for
  timeout: 120
  interval: 3
```

### Command

커스텀 명령을 실행하며, 성공(종료 코드 0)하면 정상으로 간주합니다.

```yaml
health_check:
  type: command
  target: "redis-cli ping | grep PONG"
  timeout: 30
  interval: 5
```

## 환경 변수 보간

`tow.yaml` 어디서든 `${VAR_NAME}` 문법을 사용할 수 있습니다:

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

> **보안**: `tow.yaml`에 시크릿을 하드코딩하지 마세요. 환경 변수나 (`.gitignore`에 포함된) `tow.local.yaml`을 사용하세요.

## 보존 정책

원격 서버에 보관할 이전 배포 개수를 제어합니다.

```yaml
retention:
  keep: 5              # Keep the 5 most recent deployments (default: 5)
  auto_cleanup: true   # Automatically clean up after deploy/auto (default: false)
```

- 현재 활성 배포는 절대 삭제되지 않습니다
- 수동 정리: `tow cleanup -e prod -m api-server --keep 3`
- `auto_cleanup`이 없으면 `tow cleanup`을 실행할 때까지 이전 배포가 계속 쌓입니다

## 알림

배포 이벤트 알림을 Slack, Discord, 또는 임의의 웹훅으로 전송합니다.

```yaml
notifications:
  - type: slack
    url: ${SLACK_WEBHOOK_URL}

  - type: discord
    url: ${DISCORD_WEBHOOK_URL}

  - type: webhook
    url: https://my-service.com/deploy-events
```

전송되는 이벤트: `deploy_start`, `deploy_success`, `deploy_failed`, `auto_rollback`, `rollback_success`, `rollback_failed`.

웹훅 페이로드 형식:

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

## 로컬 오버라이드 (`tow.local.yaml`)

`tow.local.yaml`은 `tow.yaml` 위에 딥 머지됩니다. 다음과 같은 경우에 유용합니다:

- 개발자별 SSH 키
- 로컬 포트 오버라이드
- 개발용 커스텀 빌드 명령

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

병합 규칙:
- 스칼라 값은 **교체**됩니다
- 맵은 **딥 머지**됩니다
- 리스트는 (추가되지 않고) **교체**됩니다

## 계층형 설정 디렉토리

Tow는 애플리케이션 설정 파일에 대해 계층형 설정 해석을 지원합니다:

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

해석 순서 (우선순위 높은 순):

1. `config/{env}-{server_number}/`
2. `config/{env}/`
3. `config/`

패키징 시, Tow는 해석된 설정 파일을 배포 아카이브의 `conf/` 디렉토리로 복사합니다.

## 전체 예제

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
