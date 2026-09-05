---
title: 시작하기
---

# 시작하기

5분 안에 Tow를 시작해보세요.

## 사전 요구사항

- **Go 1.21+** (`go install`을 사용하는 경우) 또는 사전 빌드된 바이너리 다운로드
- 대상 서버에 대한 **SSH 접근 권한** (키 기반 또는 에이전트)
- 배포할 프로젝트 (Java, Node.js, Python, Go, Rust, 또는 커스텀 애플리케이션)

## 설치

### 옵션 1: Homebrew (macOS / Linux 권장)

```bash
brew install neurosamAI/tap/tow
```

### 옵션 2: Bash 설치 스크립트

```bash
curl -fsSL https://raw.githubusercontent.com/neurosamAI/tow-cli/main/install.sh | bash
```

플랫폼을 자동으로 감지해서 최신 버전을 설치합니다.

### 옵션 3: npm

```bash
npm install -g @neurosamai/tow
```

### 옵션 4: Go Install

```bash
go install github.com/neurosamAI/tow-cli/cmd/tow@latest
```

### 옵션 5: 바이너리 직접 다운로드

```bash
# macOS (Apple Silicon)
curl -L https://github.com/neurosamAI/tow-cli/releases/latest/download/tow-darwin-arm64 -o tow
chmod +x tow && sudo mv tow /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/neurosamAI/tow-cli/releases/latest/download/tow-linux-amd64 -o tow
chmod +x tow && sudo mv tow /usr/local/bin/
```

### 옵션 6: 소스에서 빌드

```bash
git clone https://github.com/neurosamAI/tow-cli.git
cd tow-cli
make build
# Binary is at ./bin/tow
```

## 빠른 시작

### 1단계: 프로젝트 초기화

프로젝트 디렉토리로 이동해서 다음을 실행하세요:

```bash
cd my-project
tow init

# Or with AI agent integration (Claude Code + MCP server):
tow init --with-ai
```

Tow는 프로젝트 타입, 프레임워크, 빌드 도구를 자동으로 감지하고 다음을 생성합니다:
- **`tow.yaml`** — 배포 설정 파일
- **`script/`** — 서버 제어 스크립트 (`env.sh`, `server`)
- `--with-ai` 옵션 사용 시: AI 에이전트 연동을 위한 **`.claude/skills/tow-deploy.md`** + **`.claude/settings.json`**

```
✔ Detected project type: springboot
  Framework: Spring Boot
  Build tool: gradle
  Modules: api-server, batch-server
✔ Generated tow.yaml
✔ Generated script/api-server/env.sh
✔ Generated script/api-server/server
✔ Generated script/batch-server/env.sh
✔ Generated script/batch-server/server
```

### 2단계: 서버 설정

`tow.yaml`을 편집해서 서버 정보를 설정하세요:

```yaml
project:
  name: my-project
  version: 0.1.0

defaults:
  ssh_user: ec2-user
  ssh_key_path: ~/.ssh/my-key.pem

environments:
  dev:
    servers:
      - number: 1
        host: 10.0.1.10

  prod:
    branch: main    # Only allow deploys from main branch
    servers:
      - number: 1
        host: 52.78.100.1
      - number: 2
        host: 52.78.100.2

modules:
  api-server:
    type: springboot
    port: 8080
```

### 3단계: 원격 서버 초기화

원격 서버에 디렉토리 구조를 설정합니다:

```bash
tow setup -e dev
```

각 서버에 다음과 같은 구조가 생성됩니다:

```
/app/api-server-1/
├── deploy/       # Timestamped deployment versions
├── upload/       # File upload staging
├── log/          # Application logs
└── conf/         # Configuration files
```

### 4단계: 배포

전체 배포 파이프라인을 실행합니다:

```bash
# Full auto pipeline: build → package → upload → install → restart
tow auto -e dev -m api-server

# Or step by step:
tow deploy -e dev -m api-server  # package → upload → install → restart
```

### 5단계: 검증

```bash
# Check process status
tow status -e dev -m api-server

# Stream logs
tow logs -e dev -m api-server

# List deployment history
tow list deployments -e dev -m api-server
```

### 필요하다면 롤백

```bash
# Roll back to the previous deployment
tow rollback -e dev -m api-server

# Roll back to a specific version
tow rollback -e dev -m api-server -t 20240626-101530
```

## 배포 흐름 개요

```
┌─────────────────────────────────────────────────────────────┐
│  Local Machine                                              │
│                                                             │
│  1. build    →  Compile your application                    │
│  2. package  →  Create tar.gz with binaries + configs       │
│                                                             │
│  3. upload   →  ─── SCP ──→  Remote: ~/upload/module.tar.gz │
│  4. install  →  ─── SSH ──→  Extract to deploy/{timestamp}/ │
│                              Symlink: current → latest      │
│  5. restart  →  ─── SSH ──→  Stop old → Start new           │
│  6. health   →  ─── SSH ──→  TCP/HTTP/log check             │
└─────────────────────────────────────────────────────────────┘
```

롤백은 즉시 이루어집니다 — `current` 심링크를 이전 배포 디렉토리로 전환하기만 하면 됩니다.

## 다음 단계

- [아키텍처](/ko/docs/architecture/) — Tow의 내부 동작 원리 이해하기
- [설정](/ko/docs/configuration/) — `tow.yaml` 전체 레퍼런스
- [명령어](/ko/docs/commands/) — 전체 CLI 레퍼런스
- [예제](/ko/docs/examples/) — 실전 배포 시나리오
