---
title: 홈
---

<div class="hero">
  <h1>Tow</h1>
  <p class="subtitle">Kubernetes의 복잡함 없이 베어메탈 서버와 VM에 배포하세요.</p>
  <div class="hero-buttons">
    <a href="/ko/docs/getting-started/" class="btn btn-primary">시작하기</a>
    <a href="https://github.com/neurosamAI/tow-cli" class="btn btn-secondary">GitHub</a>
  </div>
</div>

## Tow란?

Tow는 Go로 작성된 **경량 에이전트리스 배포 오케스트레이터**입니다. SSH를 통해 애플리케이션을 베어메탈 서버나 클라우드 VM에 배포합니다 — 에이전트도, 컨테이너도, Kubernetes도 필요 없습니다.

```bash
# Detect your project and generate config
tow init

# Deploy to production
tow auto -e prod -m api-server

# Instant rollback
tow rollback -e prod -m api-server

# Run commands on all servers
tow ssh -e prod -m kafka --all -- "df -h"

# Compare deployed vs local before deploying
tow diff -e prod -m api-server
```

<div class="features">
  <div class="feature">
    <h3>제로 디펜던시</h3>
    <p>단일 바이너리. 서버에 대한 SSH 접근만 있으면 됩니다. 에이전트도, 런타임도, Docker도 필요 없습니다.</p>
  </div>
  <div class="feature">
    <h3>자동 감지</h3>
    <p><code>tow init</code>이 프로젝트를 스캔해서 10개 언어, 40개 이상의 프레임워크에 대한 설정을 생성합니다. 여기에 35개 인프라 플러그인(Kafka, Redis, MySQL 등)까지 지원합니다.</p>
  </div>
  <div class="feature">
    <h3>즉시 롤백</h3>
    <p>심링크 기반 원자적 배포. 밀리초 단위로 이전 버전으로 전환할 수 있습니다.</p>
  </div>
  <div class="feature">
    <h3>기본값부터 안전하게</h3>
    <p>시크릿은 git에 절대 남지 않습니다. 환경 변수, 로컬 오버라이드, SSH 에이전트, known_hosts 검증이 기본 내장되어 있습니다.</p>
  </div>
  <div class="feature">
    <h3>다중 환경</h3>
    <p>개발, 스테이징, 프로덕션 — 각각 고유한 서버, SSH 키, 브랜치 정책을 가집니다.</p>
  </div>
  <div class="feature">
    <h3>병렬 & 안전</h3>
    <p>여러 서버에 동시에 배포합니다. 내장된 배포 락이 동시 배포를 방지합니다.</p>
  </div>
  <div class="feature">
    <h3>멀티서버 운영</h3>
    <p><code>tow ssh</code>, <code>tow logs --all</code>, <code>tow diff</code> — 애드혹 명령을 실행하고, 모든 서버의 로그를 스트리밍하고, 배포된 코드와 로컬 코드를 비교합니다.</p>
  </div>
  <div class="feature">
    <h3>AI 에이전트 네이티브</h3>
    <p>Claude, Cursor, Windsurf를 위한 내장 MCP 서버. 자연어 명령으로 배포하고 모니터링하세요.</p>
  </div>
</div>

## 동작 방식

```
Local Machine                    Remote Server
─────────────                    ─────────────
1. build      →  compile/jar
2. package    →  tar.gz artifact
3. upload     →  ─── SCP ───→   ~/upload/module.tar.gz
4. install    →  ─── SSH ───→   deploy/20240626-101530/  (extract)
                                current → deploy/20240626-101530  (symlink)
5. stop/start →  ─── SSH ───→   script/server stop → start
```

롤백은 즉시 이루어집니다 — `current` 심링크를 이전 배포 디렉토리로 전환하기만 하면 됩니다.

## 빠른 시작

```bash
# Install (pick one)
brew install neurosamAI/tap/tow                    # Homebrew
curl -fsSL https://raw.githubusercontent.com/neurosamAI/tow-cli/main/install.sh | bash  # Script
npm install -g @neurosamai/tow                     # npm
go install github.com/neurosamAI/tow-cli/cmd/tow@latest  # Go

# Initialize in your project directory
cd my-project
tow init               # Or: tow init --with-ai (for Claude/MCP integration)

# Edit tow.yaml — set server IPs and SSH key paths
# Then deploy
tow auto -e dev -m my-project
```

## 왜 다른 도구를 안 쓰나요?

| | Tow | Ansible | Capistrano | Kamal |
|---|:---:|:---:|:---:|:---:|
| 단일 바이너리, 제로 디펜던시 | **지원** | Python + pip | Ruby + Bundler | Ruby + Docker |
| 자동 감지 | **지원** | 미지원 | 미지원 | 미지원 |
| Docker 불필요 | **지원** | 지원 | 지원 | **미지원** |
| 다중 언어 네이티브 지원 | **12가지 타입** | 수동 | Ruby 우선 | Docker 전용 |
| 내장 헬스체크 | **4가지 타입** | 수동 | 플러그인 | HTTP만 |
| 즉시 롤백 | **심링크** | 플레이북 재실행 | 심링크 | 컨테이너 교체 |
| 멀티서버 로그 스트리밍 | **지원** | 미지원 | 미지원 | 미지원 |
| 배포 전 diff | **지원** | 미지원 | 미지원 | 미지원 |
| AI 에이전트 지원 (MCP) | **지원** | 미지원 | 미지원 | 미지원 |

<p style="text-align: center; margin-top: 1rem;">
  <a href="/ko/docs/comparison/" style="color: #60a5fa;">전체 비교 보기 →</a>
</p>

## 실전 시나리오

### "Spring Boot 서비스 3개, Kafka, Redis를 EC2 인스턴스 5대에 운영하고 있어요"

```yaml
# One config file. That's it.
modules:
  api-server:
    type: springboot
    port: 8080
  batch-server:
    type: springboot
    port: 8081
  kafka:
    type: kafka
    port: 9092
  redis:
    type: redis
    port: 6379
```

```bash
tow auto -e prod              # Deploy everything
tow status -e prod            # Check everything
tow rollback -e prod -m api-server   # Roll back just the API
```

Ansible이라면 플레이북, 롤, 인벤토리 파일, Jinja 템플릿이 필요합니다. Tow라면 **YAML 파일 하나와 명령어 하나**면 충분합니다.

### "지난 금요일에 누군가 피처 브랜치를 프로덕션에 배포했어요"

```yaml
environments:
  prod:
    branch_policy:
      allowed: [main, hotfix/*]
```

Tow는 배포가 시작되기도 전에 막습니다. 포스트모템도 필요 없습니다.

### "새로 온 개발자가 배포 스크립트 작성에 이틀을 썼어요"

```bash
tow init    # 5 seconds. Done.
```

Tow는 언어, 프레임워크, 빌드 도구, 모노레포 모듈까지 모든 걸 자동으로 감지해서 완전히 동작하는 설정을 생성합니다.

## 이런 분들께 추천합니다

- **EC2, DigitalOcean, Hetzner** 등 모든 Linux VM에 배포하는 팀
- Kubernetes가 필요 없거나 아직 도입할 준비가 되지 않은 프로젝트
- **Ansible/Terraform을 배우지 않고도 안정적으로 배포하고 싶은** 개발자
- 수동 SSH와 완전한 오케스트레이션 사이의 **중간 지점이 필요한** 스타트업
- Java, Node.js, Python, Go, Rust 서비스를 함께 운영하는 **폴리글랏 팀**

## 실전 경험에서 태어났다

Tow는 여러 회사와 프로젝트에서 사용해온 bash 기반 배포 시스템을 Go로 다시 작성한 것입니다 — Spring Boot 마이크로서비스, Kafka 클러스터, Redis, 모니터링 스택을 관리해왔습니다. 패턴은 이미 검증되었고, Go 구현은 22대의 프로덕션 서버에서 검증을 마쳤습니다.

모든 기능은 실제 프로덕션 장애가 요구했기 때문에 존재합니다:

| 기능 | 이 기능을 만든 장애 |
|---------|-------------------------------|
| **배포 락** | 두 명의 개발자가 동시에 배포 → 서버마다 상태가 달라짐 |
| **브랜치 정책** | 금요일 저녁 실수로 피처 브랜치가 프로덕션에 배포됨 |
| **헬스체크** | 배포는 "성공"했지만 앱이 크래시 루프에 빠짐 — 30분간 아무도 눈치채지 못함 |
| **계층형 설정** | 서버 1은 서버 2와 다른 JVM 힙 설정이 필요했지만, 나머지는 모두 공유해야 함 |
| **즉시 롤백** | 잘못된 배포로 API가 다운됨 — 이전 버전을 다시 빌드하고 재배포하는 데 15분 소요 |

<div class="line-glow"></div>

<p class="brand-footer">
  Created by <a href="https://github.com/comchangs">Murry Jeong</a> &middot; Supported by <a href="https://neurosam.ai">neurosam.AI</a> &middot; MIT License &middot; <a href="https://oss.neurosam.ai">Open Source</a>
</p>
