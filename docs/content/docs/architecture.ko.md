---
title: 아키텍처
---

# 아키텍처

Tow는 몇 가지 핵심 원칙을 중심으로 설계되었습니다: **단순함**, **원자성**, 그리고 **원격 서버에 대한 제로 디펜던시**.

## 설계 철학

### 에이전트리스

Tow는 전적으로 SSH를 통해 동작합니다. 서버에는 아무것도 설치할 필요가 없습니다 — 에이전트도, 런타임도, Docker 데몬도 필요 없습니다. SSH로 접속할 수 있다면 Tow로 배포할 수 있습니다.

### 심링크를 통한 원자적 배포

배포할 때마다 타임스탬프가 찍힌 디렉토리가 생성됩니다. `current` 심링크는 현재 활성 버전을 가리킵니다. 버전 전환(배포든 롤백이든)은 단일 원자적 `ln -sfn` 연산입니다.

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

### 단일 설정 파일

모든 것이 하나의 `tow.yaml`에 정의됩니다. 인벤토리 파일도, 플레이북도, Dockerfile도 필요 없습니다 — 배포 토폴로지 전체를 기술하는 파일 하나면 충분합니다.

## 시스템 개요

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

## 핵심 컴포넌트

### Config Loader (`internal/config/`)

`tow.yaml`을 로드하고 검증하는 역할을 합니다. 주요 기능:

- **환경 변수 보간**: 문자열 값 어디든 있는 `${MY_VAR}`는 로드 시점에 해석됩니다
- **로컬 오버라이드**: `tow.local.yaml`은 `tow.yaml` 위에 딥 머지됩니다 — 개발자별 설정(SSH 키, 로컬 포트)에 이상적입니다
- **기본값 캐스케이드**: 전역 기본값 → 환경 레벨 → 모듈 레벨 → 모듈별 SSH 설정
- **검증**: 필수 필드, SSH 키 존재 여부, 포트 충돌

```yaml
# tow.yaml (committed to git)
defaults:
  ssh_key_path: ~/.ssh/deploy.pem

# tow.local.yaml (in .gitignore)
defaults:
  ssh_key_path: ~/.ssh/my-personal-key.pem
```

### Pipeline Engine (`internal/pipeline/`)

여러 단계로 이루어진 배포 워크플로우를 이름이 붙은 파이프라인으로 오케스트레이션합니다:

| 파이프라인 | 단계 |
|----------|-------|
| `auto` | `build` → `package` → `upload` → `install` → `stop` → `start` |
| `deploy` | `package` → `upload` → `install` → `stop` → `start` |

각 단계는:
- 단계 카운터로 진행 상황을 로그로 남깁니다 (`[2/6] Packaging...`)
- **dry-run 모드**를 지원합니다 (실행 없이 명령어만 표시)
- 빌드 명령어의 `${ENV}`와 `${MODULE}`을 치환합니다
- 라이프사이클 훅을 트리거합니다 (`pre_build`, `post_build`, `pre_deploy`, `post_deploy`)

### Deploy Engine (`internal/deploy/`)

모든 원격 서버 작업을 처리합니다:

| 작업 | 하는 일 |
|-----------|-------------|
| `Setup` | 원격 서버에 디렉토리 구조 생성 |
| `Upload` | SCP로 아티팩트 전송 |
| `Install` | 아카이브 추출, `current` 심링크 갱신 |
| `Start` / `Stop` | 원격에서 제어 스크립트 실행 |
| `Status` | PID, 업타임, 메모리 사용량 리포트 |
| `Rollback` | 이전/지정 버전으로 심링크 전환 |
| `Logs` | 선택적 grep 필터와 함께 원격 로그 파일 스트리밍 |
| `ListDeployments` | 타임스탬프와 함께 배포 이력 표시 |

### Module Handlers (`internal/module/`)

각 모듈 타입은 합리적인 기본값을 제공합니다:

| 핸들러 | 빌드 명령어 | 시작 | 헬스체크 |
|---------|--------------|-------|-------------|
| **springboot** | `./gradlew :mod:bootJar` | `bin/server start` | HTTP `/actuator/health` |
| **java** | `./gradlew :mod:build` | `bin/server start` | TCP 포트 |
| **node** | `npm ci && npm run build` | `node dist/main.js` | TCP 포트 |
| **python** | `pip install -r requirements.txt` | `python -m app` | TCP 포트 |
| **go** | `go build -o bin/mod` | `bin/mod` | TCP 포트 |
| **rust** | `cargo build --release` | `bin/mod` | TCP 포트 |
| **kafka** | — | `bin/kafka-server-start.sh` | TCP `:9092` |
| **redis** | — | `redis-server conf` | `redis-cli ping` |
| **generic** | (커스텀) | (커스텀) | TCP 포트 |

`tow.yaml`에서 모든 기본값을 오버라이드할 수 있습니다.

### SSH Client (`internal/ssh/`)

모든 원격 연결을 관리합니다:

- **커넥션 풀링**: keepalive 검증과 함께 SSH 연결을 재사용합니다
- **인증**: SSH 키, SSH 에이전트(`SSH_AUTH_SOCK`), 또는 비밀번호
- **모듈별 오버라이드**: 모듈마다 다른 SSH 자격 증명 사용 (예: Kafka는 다른 사용자로)
- **호스트 키 검증**: 기본적으로 `~/.ssh/known_hosts`를 사용하며, `--insecure` 플래그로 건너뛸 수 있습니다
- **작업**: 명령어 실행, 출력 스트리밍, SCP 업로드, 디렉토리용 rsync

### Initializer (`internal/initializer/`)

`tow init` 명령어의 핵심 로직입니다:

1. **스캔**: 프로젝트 디렉토리에서 빌드 파일, 프레임워크, 설정 패턴을 찾습니다
2. **감지**: 신뢰도 점수와 함께 프로젝트 타입을 감지합니다:
   - `pom.xml` + Spring Boot 의존성 → `springboot` (높은 신뢰도)
   - `package.json` + NestJS → NestJS 프레임워크를 사용하는 `node`
   - `pyproject.toml` + FastAPI → FastAPI 프레임워크를 사용하는 `python`
3. **모노레포 모듈 탐색**: `settings.gradle` 또는 `pom.xml <modules>`를 읽습니다
4. **라이브러리 모듈 필터링**: `-common`, `-core`, `-support` 등으로 끝나는 모듈을 제외합니다
5. **생성**: `tow.yaml`과 모듈별 제어 스크립트(`env.sh`, `server`)를 생성합니다

### Deploy Locking (`internal/deploy/lock.go`)

동시 배포를 방지합니다:

```
Server A: tow deploy → acquires lock (.tow-lock/info) → deploys → releases lock
Server B: tow deploy → lock exists → "Deploy in progress by user@host since..." → abort
```

- 첫 번째 대상 서버에서 원자적 `mkdir`로 락을 생성합니다
- 배포자 정보(사용자, 타임스탬프, 명령어)를 포함합니다
- 완료 시 자동 해제되며, 수동 해제는 `tow unlock`을 사용합니다

### Branch Verification (`internal/deploy/branch.go`)

프로덕션 안전성을 보장합니다:

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

- 명령어 실행 전 로컬 git 브랜치를 확인합니다
- 유연한 브랜치 정책을 위한 glob 패턴을 지원합니다
- 명령어별로 설정 가능합니다 (예: `status`는 어떤 브랜치에서든 허용)

## 계층형 설정 해석

설정 파일은 우선순위 순서로 해석됩니다:

```
config/{env}-{serverNum}/   ← Highest priority (server-specific)
config/{env}/               ← Environment-specific
config/                     ← Base configuration
```

이를 통해 세밀한 커스터마이징이 가능합니다:

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

## 병렬 실행

여러 서버에 배포할 때, Tow는 작업을 동시에 실행합니다:

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

결과는 취합됩니다 — 서버 중 하나라도 실패하면 전체 작업이 실패로 보고됩니다.

## 라이프사이클 훅

훅을 사용하면 주요 시점에 커스텀 동작을 실행할 수 있습니다:

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
