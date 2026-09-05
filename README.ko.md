<p align="center">
  <strong>&#9875; Tow</strong>
</p>

<p align="center">
  <a href="README.md">English</a> | <b>한국어</b>
</p>

<p align="center">
  <em>베어메탈 서버와 클라우드 VM을 위한 경량 에이전트리스 배포 오케스트레이터.</em>
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

많은 팀이 EC2 인스턴스, 베어메탈 서버, 단순한 VM 위에서 서비스를 운영한다. Kubernetes까지는 필요 없지만, 그래도 안정적이고 반복 가능한 배포와 무중단 롤백은 원한다. **Tow는 이 빈틈을 채운다.**

```bash
tow init                              # Auto-detect project → generate config
tow auto -e prod -m api-server        # Build → package → upload → deploy
tow rollback -e prod -m api-server    # Instant rollback
```

<details>
<summary><b>실제 동작 화면 — 실제 프로덕션 출력</b></summary>

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
  ✓ Server dependencies OK
  10 passed, 0 failed
```

</details>

## Why Tow?

| | Tow | Ansible | Capistrano | Kamal |
|---|:---:|:---:|:---:|:---:|
| 단일 바이너리, 의존성 없음 | **Yes** | Python + pip | Ruby + Bundler | Ruby + Docker |
| 자동 감지 | **Yes** | No | No | No |
| Docker 불필요 | **Yes** | Yes | Yes | **No** |
| 다중 언어 네이티브 지원 | **12개 타입** | Manual | Ruby-first | Docker-only |
| 내장 헬스체크 | **4가지 타입** | Manual | Plugin | HTTP only |
| 즉시 롤백 | **Symlink** | Re-run playbook | Symlink | Container swap |

> 자세한 분석은 [전체 비교](https://tow-cli.neurosam.ai/comparison)를 참고하라.

### Tow가 다른 이유

**대부분의 배포 도구는 자신들의 세계로 들어오라고 강요한다.** Ansible을 쓰려면 YAML/Jinja 전문가가 되어야 한다. Capistrano는 당신이 Ruby 진영이라고 전제한다. Kamal은 모든 서버에 Docker를 요구한다. Tow는 이미 가지고 있는 것들과 함께 동작한다.

<details>
<summary><b>다른 어떤 도구에도 없는 자동 감지</b></summary>
<br>

다른 배포 도구는 전부 빈 설정 파일에서 시작해서 처음부터 직접 써야 한다. Tow는 프로젝트를 스캔해서 모든 것을 생성한다.

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

언어, 프레임워크(NestJS, FastAPI, Django, Flask, Next.js...), 빌드 도구, 모노레포 서브 모듈을 감지하고, `-common`이나 `-core` 같은 라이브러리 모듈은 걸러내기까지 한다. **몇 초 만에 배포 가능한 설정으로.**

</details>

<details>
<summary><b>Docker가 없다고? 문제없다. (Kamal과 달리)</b></summary>
<br>

[Kamal](https://kamal-deploy.org/)은 훌륭한 도구다 — 하지만 대상 서버마다 Docker를 요구한다. 이는 다음과 같은 경우엔 애초에 선택지가 될 수 없다.

- 이미 자체 라이프사이클을 관리하고 있어 컨테이너화로 얻을 게 없는 **JVM 애플리케이션**
- Docker 설치가 아예 선택지에 없는 **레거시 인프라**
- Docker 데몬 오버헤드가 부담되는 **리소스 제약 VM**
- Docker GPU 패스스루가 복잡도를 더하는 **GPU 워크로드**
- 컨테이너 사용에 제약이 있는 **규제 환경**

Tow는 OS에 직접 배포한다. SSH만 열려 있으면 배포할 수 있다.

</details>

<details>
<summary><b>애플리케이션과 인프라, 한 도구로 전부</b></summary>
<br>

대부분의 배포 도구는 애플리케이션 코드만 다룬다. VM 위에서 Kafka, Redis, MongoDB까지 운영한다면 별도의 자동화가 또 필요하다.

Tow는 이 모든 것을 하나의 인터페이스로 관리한다.

```bash
tow auto -e prod -m api-server     # Spring Boot app
tow auto -e prod -m kafka          # Kafka broker
tow auto -e prod -m redis          # Redis server
tow status -e prod                  # Check everything
```

동일한 설정 파일. 동일한 명령어. 동일한 롤백 메커니즘.

</details>

<details>
<summary><b>이론이 아니라 실전에서 검증된 패턴</b></summary>
<br>

Tow는 주말 프로젝트가 아니다. 여러 회사의 프로덕션에서 실제로 운영되던 배포 시스템 — Spring Boot 마이크로서비스, Kafka 클러스터, Redis, MongoDB, Prometheus, Grafana 등을 관리하던 시스템 — 을 Go로 다시 작성한 것이다.

모든 기능은 실제 프로덕션 장애가 요구해서 생겨났다.

- **배포 잠금(Deploy locking)** → 두 명의 개발자가 동시에 배포해서 상태가 꼬였던 사고
- **브랜치 정책(Branch policies)** → 누군가 피처 브랜치를 프로덕션에 배포했던 사고
- **헬스체크(Health checks)** → 배포는 "성공"이라고 나왔지만 앱이 크래시 루프에 빠져 있던 사고
- **계층형 설정(Hierarchical config)** → 서버 1과 서버 2가 서로 다른 JVM 설정을 필요로 했던 상황

이론적인 기능이 아니다. 실전에서 얻은 흉터가 안전장치로 바뀐 것들이다.

</details>

## Core Features

- **에이전트리스(Agentless)** — 대상 서버에 필요한 것은 SSH뿐
- **심링크 기반 원자적 배포** — 심링크 전환만으로 즉시 롤백
- **자동 감지** — `tow init`이 프로젝트 타입, 프레임워크, 빌드 도구, 모노레포 모듈을 감지
- **12가지 내장 모듈 핸들러** — Spring Boot, Java, Node.js, Python, Go, Rust, Kafka, Redis, generic
- **4가지 헬스체크 타입** — HTTP, TCP, 로그 패턴, 커스텀 커맨드
- **계층형 설정** — 환경 단위 기본값 + 서버별 오버라이드
- **브랜치 정책** — 잘못된 브랜치가 프로덕션에 배포되는 것을 방지
- **배포 잠금** — 동시 배포 방지
- **병렬 실행** — 여러 서버에 동시 배포
- **라이프사이클 훅** — 빌드/배포/시작/정지 전후 + post_install
- **커스텀 패키지 레이아웃** — 소스 파일을 패키지 내 원하는 디렉토리 구조로 재배치

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

> 자세한 안내는 [Getting Started 가이드](https://tow-cli.neurosam.ai/getting-started)를 참고하라.

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

**롤백**은 즉시 이루어진다 — `current` 심링크를 이전 배포 디렉토리로 전환하기만 하면 된다.

## Commands

| Command | Description |
|---------|-------------|
| `tow init` | 프로젝트 타입을 자동 감지하고 `tow.yaml` 생성 |
| `tow validate` | 설정 파일 검증 |
| `tow setup` | 원격 서버 디렉토리 초기화 |
| `tow auto` | 전체 파이프라인: build → package → upload → install → restart |
| `tow deploy` | 배포 파이프라인: package → upload → install → restart |
| `tow start` | 모듈 시작 |
| `tow stop` | 모듈 정지 |
| `tow restart` | 모듈 재시작 |
| `tow status` | 모듈 상태 확인 (PID, uptime, memory) |
| `tow rollback` | 이전 배포로 전환 |
| `tow logs` | grep 필터를 곁들여 원격 로그 스트리밍 |
| `tow ssh` | 원격 서버에서 명령 실행 |
| `tow diff` | 배포된 코드와 로컬 코드 비교 |
| `tow config` | 서버, 모듈, 할당 관리 |
| `tow upload` | 대상 서버에 파일 업로드 |
| `tow install` | 업로드된 패키지 설치 (압축 해제 + 심링크) |
| `tow login` | 서버에 SSH 접속 |
| `tow unlock` | 배포 잠금 강제 해제 |
| `tow list` | 모듈, 환경, 배포 이력 목록 조회 |
| `tow cleanup` | 오래된 배포 디렉토리 정리 |
| `tow download` | 원격 서버에서 파일 다운로드 |
| `tow provision` | 신규 서버 프로비저닝 (타임존, JRE, 도구) |
| `tow metrics` | 감사 로그 기반 배포 통계 표시 |
| `tow doctor` | 사전 점검 진단 |
| `tow mcp-server` | AI 에이전트 연동을 위한 MCP 서버 시작 |

**고급 플래그:**

```bash
tow deploy --rolling          # Deploy one server at a time
tow auto --auto-rollback      # Auto-rollback if health check fails
tow auto -y                   # Skip production confirmation prompt
tow status -o json            # Machine-readable JSON output
```

> 모든 플래그와 사용법은 [전체 커맨드 레퍼런스](https://tow-cli.neurosam.ai/commands)를 참고하라.

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

인프라 서비스는 [35개의 내장 YAML 플러그인](plugins/)으로 지원된다 — Kafka, Redis, MySQL, PostgreSQL, MongoDB, Elasticsearch, ZooKeeper, Nginx, Prometheus, Grafana, Loki, Vault, Jenkins 등.

커뮤니티 플러그인은 GitHub에서 설치할 수 있다.

```bash
tow plugin add someuser/tow-plugin-mssql          # GitHub repo
tow plugin add myorg/infra-plugins/oracle.yaml     # specific file
tow plugin add https://example.com/custom.yaml     # any URL
tow plugin list                                     # list all (bundled + external)
```

## Documentation

전체 문서는 **[tow-cli.neurosam.ai](https://tow-cli.neurosam.ai)**에서 확인할 수 있다.

- [Getting Started](https://tow-cli.neurosam.ai/getting-started) — 5분 안에 설치하고 배포하기
- [Architecture](https://tow-cli.neurosam.ai/architecture) — Tow의 내부 동작 방식
- [Configuration](https://tow-cli.neurosam.ai/configuration) — `tow.yaml` 전체 레퍼런스
- [Commands](https://tow-cli.neurosam.ai/commands) — CLI 레퍼런스
- [Security](https://tow-cli.neurosam.ai/security) — 시크릿 관리와 SSH 하드닝
- [Examples](https://tow-cli.neurosam.ai/examples) — 실전 배포 시나리오
- [Comparison](https://tow-cli.neurosam.ai/comparison) — Tow vs. Ansible, Capistrano, Kamal 등

## AI Agent Integration

Tow는 AI 에이전트를 네이티브로 지원하는 최초의 배포 도구다.

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

이제 AI 어시스턴트에게 이렇게 물어보라. *"prod 환경의 api-server 상태를 확인해줘"* 또는 *"API를 롤백해줘"*.

### Claude Code Skill

[`integrations/claude-skill/tow-deploy.md`](integrations/claude-skill/tow-deploy.md)를 프로젝트의 `.claude/skills/`로 복사하라.

### VS Code Extension

[`integrations/vscode/`](integrations/vscode/)를 참고하라 — 환경, 모듈, 배포 제어를 위한 사이드바 UI를 제공한다.

## Production-Tested

Tow는 [Balkari Inc.](https://balkari.io)의 **실제 프로덕션 인프라**에서 검증되었으며, 다음을 관리하고 있다.

- Spring Boot 마이크로서비스, Kafka 클러스터, Redis, MongoDB, Prometheus, Grafana 등을 아우르는 **22대 서버**
- **멀티서버 운영**: 3노드 Kafka 클러스터 전체 상태 확인, 다중화된 로그 뷰잉, 병렬 SSH 실행
- **레거시 호환성**: 셸 스크립트로 만들어진 기존 배포(deploy_path `{module}-{server}`)를 매끄럽게 관리

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
  ✓ Server dependencies OK
  10 passed, 0 failed
```

## Current Status

Tow는 현재 **v0.4** 단계로, 프로덕션에서 검증되었으며 활발히 개발되고 있다.

**알려진 제약:**
- 대용량 파일 업로드가 중단되었을 때 재개 기능 없음
- 블루-그린 배포 전략은 아직 미지원 (롤링 배포는 지원)
- IDE 플러그인(VS Code, JetBrains)은 동작은 하지만 아직 초기 단계

**v1.0까지의 로드맵:**
- 블루-그린 배포 전략
- 배포 상태를 보여주는 웹 대시보드
- 민감한 값을 위한 설정 암호화
- 테스트 커버리지 80% 이상을 위한 SSH 인터페이스 추상화

문제를 발견했다면 [GitHub 이슈](https://github.com/neurosamAI/tow-cli/issues)를 열어달라. 버그 리포트는 진지하게 다룬다.

## Contributing

기여를 환영한다! 가이드라인은 [CONTRIBUTING.md](CONTRIBUTING.md)를 참고하라.

## License

[MIT](LICENSE) — Copyright (c) 2026-present [Murry Jeong (comchangs)](https://github.com/comchangs) and [neurosam.AI](https://neurosam.ai) (Neurosam AI Inc.)

---

<p align="center">
  <sub>Created by <a href="https://github.com/comchangs"><strong>Murry Jeong (comchangs)</strong></a> &middot; Supported by <a href="https://neurosam.ai"><strong>neurosam.AI</strong></a></sub>
</p>
