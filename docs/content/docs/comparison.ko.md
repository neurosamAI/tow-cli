---
title: 비교
---

# 왜 Tow인가?

세상에는 다양한 배포 도구가 있습니다. Tow가 어떻게 비교되는지, 그리고 왜 우리가 이 도구를 만들었는지 살펴봅니다.

## 문제

서버가 몇 대 있습니다. 앱을 안정적으로 배포해야 합니다. 선택지는 다음과 같습니다:

- **수동 SSH** — 실수하기 쉽고, 롤백이 없고, 일관성이 없습니다
- **Ansible** — 강력하지만 복잡합니다. YAML 플레이북, 인벤토리 파일, 롤, Galaxy — Ansible 전문가가 되어야 합니다
- **Capistrano** — Ruby에는 훌륭하지만, 로컬에 Ruby가 설치되어 있어야 합니다. 다른 언어는 2급 시민 취급을 받습니다
- **Kamal** — 현대적이고 우아하지만, 모든 서버에 Docker가 필요합니다
- **Kubernetes** — 서버 2~5대에는 지나친 운영 오버헤드입니다

Tow는 그 빈틈을 채웁니다: **복잡함 없이 프로덕션급 배포**.

### 실제 예시

Spring Boot API + Kafka + Redis를 EC2 인스턴스 3대에 배포해야 한다고 해봅시다. 각 도구가 요구하는 것은 다음과 같습니다:

**Ansible** — 5개 이상 파일에 걸쳐 약 150줄:
```
ansible/
├── inventory/prod.yml          # Server definitions
├── group_vars/prod.yml         # Variables
├── roles/
│   ├── api-server/tasks/main.yml
│   ├── kafka/tasks/main.yml
│   └── redis/tasks/main.yml
├── deploy.yml                  # Main playbook
└── rollback.yml                # Separate rollback playbook
```

**Kamal** — 모든 서버에 Docker가 필요하고, 컨테이너 레지스트리도 필요합니다:
```bash
# Every server needs: Docker daemon, Docker CLI, login to registry
# Every app needs: Dockerfile, image build, push to registry, pull on server
```

**Tow** — 파일 하나, 자동 생성:
```bash
tow init          # Generates tow.yaml (detected: springboot + kafka + redis)
vim tow.yaml      # Add server IPs
tow auto -e prod  # Done.
```

이게 바로 차이입니다. 서류상의 기능 차이가 아니라, **첫 배포까지 걸리는 시간**의 차이입니다.

## 비교표

| | **Tow** | **Capistrano** | **Ansible** | **Kamal** | **Fabric** | **Deployer** | **PM2** |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **단일 바이너리** | ● | | | | | | |
| **런타임 의존성 없음** | ● | Ruby | Python | Ruby + Docker | Python | PHP | Node.js |
| **Docker 불필요** | ● | ● | ● | | ● | ● | ● |
| **에이전트리스 (SSH만)** | ● | ● | ● | ● | ● | ● | ● |
| **원자적 심링크 배포** | ● | ● | | | | ● | |
| **자동 감지** | ● | | | | | | |
| **내장 헬스체크** | ● | | | ● | | | ● |
| **다중 언어 네이티브 지원** | ● | | ● | ● | ● | | |
| **모노레포 지원** | ● | | | | | | |
| **배포 락** | ● | | | | | | |
| **계층형 설정** | ● | | ● | | | | |
| **인프라 서비스** | ● (35개 플러그인) | | ● | ● | | | |
| **AI 에이전트 지원 (MCP)** | ● | | | | | | |
| **롤링 배포** | ● | | ● | ● | | | |
| **자동 롤백** | ● | | | ● | | | |
| **내장 알림** | ● | ● | | | | | |
| **러닝커브** | 낮음 | 중간 | 높음 | 중간 | 중간 | 중간 | 낮음 |
| **GitHub Stars** | 신규 | ~13k | ~63k | ~14k | ~15k | ~11k | ~43k |

## Tow vs. Ansible

**Ansible**은 설정 관리와 인프라 자동화의 업계 표준입니다. 놀라울 정도로 강력하지만, 놀라울 정도로 복잡합니다.

| 항목 | Tow | Ansible |
|--------|-----|---------|
| **목적** | 애플리케이션 배포 | 인프라 + 배포 + 설정 관리 |
| **설정 형식** | 단일 `tow.yaml` | 플레이북 + 인벤토리 + 롤 + group_vars + 템플릿 |
| **러닝커브** | 몇 분 | 며칠 ~ 몇 주 |
| **설치** | `go install` (단일 바이너리) | `pip install ansible` + 의존성 |
| **배포 모델** | 심링크 기반 원자적 | 태스크 기반 (또는 ansistrano 롤) |
| **롤백** | 내장, 즉시 | 수동 (이전 플레이북 재실행 또는 ansistrano 사용) |
| **헬스체크** | 내장 (HTTP, TCP, log, command) | 수동 (직접 태스크 작성) |

**Ansible을 선택하세요** 전체 인프라 관리(패키지 설치, 사용자 관리, 방화벽 설정)가 필요하다면. **Tow를 선택하세요** 애플리케이션을 안정적으로 배포하는 것만 필요하다면.

> Tow와 Ansible은 상호 배타적이지 않습니다. 서버 프로비저닝에는 Ansible을, 애플리케이션 배포에는 Tow를 사용하세요.

### 실전에서

흔히 있는 실제 이야기: 한 팀이 배포에 Ansible을 도입합니다. 1개월 차엔 잘 돌아갑니다. 6개월 차가 되면 아무도 완전히 이해하지 못하는 2,000줄짜리 플레이북이 쌓여 있습니다. 새로 온 개발자는 배포 스크립트를 건드리기 무서워합니다. 롤백이요? 어느 서버에 어떤 플레이북 버전이 돌고 있었는지 기억하고 있기를 바랄 뿐입니다.

Tow라면 배포 설정 전체가 읽기 쉬운 YAML 파일 하나입니다. 롤백은 `tow rollback` 한 줄입니다. 새로 온 개발자도 첫날부터 배포할 수 있습니다.

## Tow vs. Capistrano

**Capistrano**는 심링크 기반 원자적 배포를 개척했습니다. Tow는 이 검증된 패턴을 그대로 가져와 언어에 구애받지 않게 만들었습니다.

| 항목 | Tow | Capistrano |
|--------|-----|------------|
| **언어** | Go (단일 바이너리) | Ruby (Ruby + Bundler 필요) |
| **대상 생태계** | Java, Node, Python, Go, Rust, Kafka, Redis 등 | Ruby/Rails (다른 언어는 플러그인 경유) |
| **자동 감지** | 프로젝트 타입, 프레임워크, 빌드 도구 감지 | 수동 설정 |
| **헬스체크** | 내장 | 플러그인을 통해서만 |
| **모노레포** | 네이티브 멀티모듈 지원 | 앱별 수동 설정 |
| **설정** | YAML | Ruby DSL |

**Capistrano를 선택하세요** 기존 Capistrano 레시피가 있는 Ruby 조직이라면. **Tow를 선택하세요** 폴리글랏 팀이거나 Ruby가 아닌 프로젝트라면.

### 실전에서

팀이 Spring Boot API, Node.js 프론트엔드, Python ML 서비스를 운영하고 있다고 해봅시다. Capistrano를 쓴다면 모든 개발자 머신에 Ruby를 설치하고, Rake DSL을 배우고, 유지보수될지 안 될지 모르는 Java·Node.js용 커뮤니티 플러그인을 이어붙여야 합니다.

Tow라면 세 가지 모두 1급 시민입니다:

```bash
tow auto -e prod -m api-server       # Spring Boot — knows about gradlew, bootJar, actuator
tow auto -e prod -m web-frontend     # Node.js — knows about npm ci, dist/, health endpoints
tow auto -e prod -m ml-service       # Python — knows about pip, gunicorn, virtualenvs
```

도구 하나. 설정 하나. Ruby 필요 없음.

## Tow vs. Kamal

**Kamal**(DHH / 37signals 제작)은 철학적으로 가장 가까운 경쟁자입니다 — 둘 다 "Kubernetes 없이 VM에 배포"를 지향합니다. 핵심 차이는: Kamal은 Docker가 필요합니다.

| 항목 | Tow | Kamal |
|--------|-----|-------|
| **배포 단위** | 애플리케이션 아티팩트 (JAR, 바이너리, dist/) | Docker 컨테이너 |
| **Docker 필요 여부** | 불필요 | 필요 (모든 서버에) |
| **트래픽 관리** | 애플리케이션 레벨 | kamal-proxy (내장 리버스 프록시) |
| **무중단 배포** | 심링크 전환 + 헬스체크 | 프록시를 통한 컨테이너 교체 |
| **빌드 파이프라인** | 로컬 빌드 → 패키징 → 업로드 | 로컬 빌드 → 레지스트리에 푸시 → 서버에서 풀 |
| **인프라 서비스** | 네이티브 핸들러 (Kafka, Redis) | Docker 액세서리 |
| **리소스 오버헤드** | 최소 (직접 프로세스) | Docker 데몬 + 컨테이너 오버헤드 |

**Kamal을 선택하세요** 팀이 이미 Docker를 쓰고 있고 컨테이너 기반 배포를 원한다면. **Tow를 선택하세요** 컨테이너 오버헤드 없이 OS에 직접 배포하는 걸 선호한다면.

### Docker가 맞지 않는 경우

- **JVM 애플리케이션**: 이미 자체적으로 프로세스 라이프사이클을 관리합니다. Docker는 이점 없이 오버헤드만 추가합니다
- **레거시 인프라**: Docker 설치가 불가능한 서버
- **리소스가 제한된 환경**: 작은 VM에서는 Docker 데몬 + 컨테이너 오버헤드가 부담됩니다
- **GPU 워크로드**: Docker GPU 패스스루는 복잡함을 더합니다
- **규제 환경**: 일부 컴플라이언스 프레임워크는 컨테이너 사용을 제한합니다

### 실전에서

한 스타트업이 t3.medium EC2 인스턴스(2 vCPU, 4GB RAM)에서 Spring Boot 서비스 3개를 운영하고 있습니다. JVM은 이미 `-Xmx`로 메모리를 관리하고 있습니다. Docker를 추가하면:

- 각 서버에 Docker 데몬을 설치하고 유지 관리해야 합니다 (RAM 오버헤드 300MB 이상)
- Docker 이미지를 빌드해야 합니다 (레지스트리 필요 — ECR, Docker Hub, 또는 셀프 호스팅)
- 디버깅이 그냥 `ssh` 대신 `docker exec -it`가 됩니다
- 단순 파일 대신 Docker의 로그 드라이버로 로그를 관리해야 합니다
- JVM 힙 튜닝이 Docker의 메모리 제한과 충돌하게 됩니다

Tow라면 JVM이 OS에서 직접 실행됩니다. RAM을 온전히 쓸 수 있습니다. 로그는 평범한 파일입니다. SSH로 직접 접근할 수 있습니다. 앱과 사용자 사이에 추상화 레이어가 없습니다.

이미 컨테이너화되어 있다면 Kamal이 옳은 선택입니다. 하지만 Docker가 아직 스택의 일부가 아니라면, Tow는 그 복잡함을 더하지 않고도 배포할 수 있게 해줍니다.

## Tow vs. Fabric

**Fabric**은 Python SSH 태스크 러너입니다 — 배포 프레임워크가 아니라 저수준 빌딩 블록입니다.

| 항목 | Tow | Fabric |
|--------|-----|--------|
| **추상화 수준** | 완전한 배포 파이프라인 | SSH 태스크 러너 |
| **기본 제공 기능** | 빌드 → 배포 → 헬스체크 → 롤백 | SSH로 명령 실행 |
| **배포 전략** | 심링크 기반 원자적 (내장) | 직접 구축 |
| **롤백** | 내장 (`tow rollback`) | 직접 작성 |
| **헬스체크** | 내장 (4가지 타입) | 직접 작성 |

**Fabric을 선택하세요** 커스텀 자동화를 위한 프로그래밍 가능한 SSH 툴킷이 필요하다면. **Tow를 선택하세요** 바로 쓸 수 있는 완전한 배포 솔루션을 원한다면.

## Tow vs. PM2 Deploy

**PM2**는 기본적인 배포 기능을 갖춘 Node.js 프로세스 매니저입니다.

| 항목 | Tow | PM2 Deploy |
|--------|-----|------------|
| **언어 지원** | Java, Node, Python, Go, Rust 등 | Node.js만 |
| **배포 모델** | 아티팩트 기반, 심링크 원자적 | Git 기반 (서버에서 git pull) |
| **롤백** | 즉시 심링크 전환 | Git revert |
| **프로세스 관리** | 외부 (시스템 스크립트) | 내장 (클러스터 모드, 자동 재시작) |
| **헬스체크** | HTTP, TCP, log, command | 프로세스 모니터링 |

**PM2를 선택하세요** 단일 Node.js 앱을 운영하며 내장 프로세스 클러스터링을 원한다면. **Tow를 선택하세요** 멀티서비스, 멀티언어 배포라면.

## Tow만의 차별점

### 1. 실제로 동작하는 자동 감지

```bash
$ cd my-springboot-project
$ tow init
✔ Detected: springboot (gradle, multi-module)
  Modules: api-server, batch-server, admin-server
  Excluded: common-lib, data-core (library modules)
✔ Generated tow.yaml + control scripts
```

프로젝트를 스캔해서 완전히 동작하는 설정을 생성해주는 배포 도구는 달리 없습니다. Tow는 다음을 감지합니다:

- **언어**: Java, Node.js, Python, Go, Rust
- **프레임워크**: Spring Boot, NestJS, Express, FastAPI, Django, Flask, Next.js
- **빌드 도구**: Gradle, Maven, npm, yarn, pnpm, poetry, uv, cargo
- **모노레포 구조**: Gradle `settings.gradle`, Maven `pom.xml <modules>`
- **라이브러리 모듈 vs. 배포 모듈**: `-common`, `-core`, `-support` 모듈을 걸러냅니다

### 2. 진짜 폴리글랏 — 나중에 끼워맞춘 게 아니다

대부분의 배포 도구는 하나의 생태계만 네이티브로 지원하고 나머지는 억지로 끼워 맞춥니다:

- Capistrano: Ruby 우선, 다른 언어는 커뮤니티 플러그인 경유
- Deployer: PHP 우선, PHP 생태계에 국한
- PM2: Node.js만
- Kamal: 언어에 구애받지 않지만 모든 것에 Docker가 필요

Tow는 12가지 모듈 타입에 대해 **1급 핸들러**를 갖추고 있으며, 각각 빌드 명령어, start/stop 스크립트, 헬스체크, 아티팩트에 대한 합리적인 기본값을 제공합니다.

### 3. 서버에 설치할 것이 없다

- 설치하거나 업데이트할 에이전트가 없습니다
- 유지 관리할 Docker 데몬이 없습니다
- 프로비저닝할 런타임이 없습니다
- SSH로 접속할 수 있다면 배포할 수 있습니다

### 4. 프로덕션 경험에서 태어났다

Tow는 장난감 프로젝트가 아닙니다. 여러 회사와 프로젝트에서 프로덕션에 사용되어 온 배포 시스템을 Go로 다시 작성한 것입니다 — Spring Boot 마이크로서비스, Kafka 클러스터, Redis, MongoDB, 모니터링 스택 등을 관리해왔습니다.

Tow의 모든 기능은 실전에서 얻은 상처가 안전장치로 바뀐 것입니다:

| 기능 | 이 기능을 만든 실제 장애 |
|---------|---------------------------|
| **배포 락** | 두 명의 개발자가 동시에 배포를 실행했습니다. 서버 1은 버전 A를, 서버 2는 버전 B를 받았습니다. 요청을 어느 서버가 처리하느냐에 따라 API 호출이 무작위로 실패했습니다. |
| **브랜치 정책** | 한 엔지니어가 금요일 오후 6시에 피처 브랜치를 프로덕션에 배포했습니다. 그 브랜치에는 데이터베이스 스키마를 망가뜨리는 마이그레이션이 들어 있었습니다. 주말 내내 장애 대응을 했습니다. |
| **헬스체크** | 배포 스크립트는 "성공"이라고 보고했지만 앱은 크래시 루프에 빠져 있었습니다. 원인은 누락된 환경 변수였습니다. 고객이 에러를 신고할 때까지 30분간 아무도 눈치채지 못했습니다. |
| **계층형 설정** | 프로덕션 서버 1(주)은 4GB 힙이, 서버 2(부)는 2GB가 필요했습니다. 하지만 나머지 설정은 모두 공유해야 했습니다. 설정 파일을 복사-붙여넣기 하다 보니 서서히 어긋났습니다. |
| **즉시 롤백** | 잘못된 배포가 나갔습니다. 팀은 git에서 이전 버전을 다시 빌드하고, 재패키징하고, 재업로드하고, 재설치해야 했습니다. 15분이 걸렸습니다. 심링크 롤백을 쓰면 1초면 됩니다. |
| **병렬 배포** | 서비스 5개를 서버 3대에 순차적으로 배포하는 데 25분이 걸렸습니다. 엔지니어들은 다른 일로 넘어갔다가 마지막 몇 개를 확인하는 걸 잊었습니다. |

이건 이론적인 기능이 아닙니다. 실제 팀의 실제 시간을 갉아먹는 문제를 해결합니다.

### 5. 첫 배포까지 걸리는 시간

궁극적인 차별점은 **제로에서 동작하는 배포 파이프라인까지 얼마나 빨리 도달하는가**입니다:

| 도구 | 첫 배포까지의 단계 |
|------|----------------------|
| **Ansible** | Python + Ansible 설치 → YAML/Jinja 학습 → 인벤토리 작성 → 플레이북 작성 → 롤 작성 → 템플릿 작성 → 디버그 → 배포 |
| **Capistrano** | Ruby + Bundler 설치 → Rake DSL 학습 → deploy.rb 작성 → 스테이지 설정 작성 → 언어별 플러그인 찾기 → 디버그 → 배포 |
| **Kamal** | Ruby 설치 → Docker 설치 (모든 서버) → 컨테이너 레지스트리 설정 → Dockerfile 작성 → deploy.yml 작성 → 디버그 → 배포 |
| **Tow** | `go install ...` → `tow init` → 3줄 수정 (서버 IP) → `tow auto -e prod` |

Tow는 **첫 배포가 며칠이 아니라 몇 분 만에** 이루어지도록 설계되었습니다.

## 마이그레이션 가이드

### 셸 스크립트에서

현재 커스텀 셸 스크립트로 배포하고 있다면:

1. 프로젝트에서 `tow init` 실행
2. 스크립트를 Tow 작업에 매핑:
   - `build.sh` → `tow auto` (빌드 단계 포함)
   - `deploy.sh` → `tow deploy`
   - `restart.sh` → `tow restart`
3. 스크립트의 서버 IP를 `tow.yaml`로 이동
4. 첫 실제 배포 전에 `--dry-run`으로 확인

### Capistrano에서

1. `deploy.rb`의 스테이지를 `tow.yaml`의 환경으로 매핑
2. `linked_dirs`를 `data_dirs`로 매핑
3. `linked_files`를 `config_dir` 계층으로 매핑
4. Capistrano의 `current` / `releases` 패턴은 Tow의 심링크 모델에 그대로 대응됩니다

### Ansible에서

1. 인벤토리 호스트를 `tow.yaml`의 환경/서버로 매핑
2. 배포 플레이북 태스크를 Tow의 내장 파이프라인으로 매핑
3. 서버 프로비저닝은 Ansible을 유지하고, 애플리케이션 배포는 Tow를 사용
