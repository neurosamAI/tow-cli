---
title: 명령어
---

# 명령어 레퍼런스

## 전역 플래그

다음 플래그는 모든 명령어에서 사용할 수 있습니다:

| 플래그 | 축약형 | 설명 | 기본값 |
|------|-------|-------------|---------|
| `--config` | `-c` | 설정 파일 경로 | `tow.yaml` |
| `--environment` | `-e` | 대상 환경 | — |
| `--module` | `-m` | 대상 모듈 | — |
| `--server` | `-s` | 대상 서버 번호 (0 = 전체) | `0` |
| `--verbose` | `-v` | 디버그 출력 활성화 | `false` |
| `--dry-run` | | 실행 없이 동작만 표시 | `false` |
| `--insecure` | | SSH 호스트 키 검증 건너뛰기 | `false` |

## 명령어

### `tow init`

프로젝트 타입을 자동으로 감지하고 설정을 생성합니다.

```bash
tow init
```

프로젝트 디렉토리를 스캔해서 다음을 감지합니다:
- **언어**: Java, Node.js, Python, Go, Rust
- **프레임워크**: Spring Boot, NestJS, Express, FastAPI, Django, Flask, Next.js
- **빌드 도구**: Gradle, Maven, npm, yarn, pnpm, poetry, uv, cargo
- **모노레포 모듈**: `settings.gradle`, `pom.xml <modules>`
- **CI/CD**: Dockerfile, docker-compose, GitHub Actions, GitLab CI, Jenkins

생성 항목:
- 감지된 모듈 타입과 합리적인 기본값이 담긴 `tow.yaml`
- `script/{module}/env.sh` — 모듈별 환경 변수
- `script/{module}/server` — start/stop/status/restart 제어 스크립트

> **참고**: 라이브러리 모듈(`-common`, `-core`, `-support`, `-lib`, `-shared`, `-util`로 끝나는 모듈)은 배포 모듈에서 자동으로 제외됩니다.

### `tow validate`

서버에 연결하지 않고 설정 파일을 검증합니다.

```bash
tow validate
tow validate -c custom-config.yaml
```

검사 항목:
- 필수 필드 (`project.name`, 서버, 모듈)
- SSH 키 파일 존재 여부
- 모듈 타입 유효성
- 헬스체크 설정

### `tow deploy`

배포 파이프라인을 실행합니다: **package → upload → install → stop → start**.

```bash
tow deploy -e prod -m api-server
tow deploy -e prod -m api-server -s 1    # Deploy to server 1 only
tow deploy -e prod                        # Deploy all modules
tow deploy -e prod --dry-run              # Preview without executing
```

### `tow auto`

전체 파이프라인을 실행합니다: **build → package → upload → install → stop → start**.

```bash
tow auto -e dev -m api-server
tow auto -e prod -m api-server --dry-run
```

소스 코드부터 실행 중인 서비스까지 엔드투엔드로 배포하는 가장 일반적인 명령어입니다.

### `tow start`

원격 서버에서 모듈을 시작합니다.

```bash
tow start -e prod -m api-server
tow start -e prod -m api-server -s 1
```

시작 후, Tow는 모듈의 `health_check` 설정에 따라 헬스체크를 실행합니다.

### `tow stop`

원격 서버에서 모듈을 중지합니다.

```bash
tow stop -e prod -m api-server
tow stop -e prod -m api-server -s 2
```

### `tow restart`

모듈을 중지한 다음 시작합니다.

```bash
tow restart -e prod -m api-server
```

### `tow status`

모듈의 현재 상태를 확인합니다.

```bash
tow status -e prod -m api-server
```

표시 항목:
- 프로세스 ID (PID)
- 업타임
- 메모리 사용량
- 현재 배포 버전 (심링크 대상)

### `tow rollback`

이전 배포 버전으로 전환합니다.

```bash
# Roll back to the previous deployment
tow rollback -e prod -m api-server

# Roll back to a specific version
tow rollback -e prod -m api-server -t 20240626-101530
```

롤백은 즉시 이루어집니다 — `current` 심링크를 전환하고 서비스를 재시작합니다. 재업로드나 재빌드가 필요 없습니다.

### `tow logs`

하나 이상의 서버에서 원격 로그 파일을 스트리밍합니다.

```bash
tow logs -e prod -m api-server
tow logs -e prod -m api-server -f "ERROR"                 # Filter with grep
tow logs -e prod -m api-server -n 100                     # Last 100 lines
tow logs -e prod -m api-server -s kafka-1                 # Specific server
tow logs -e prod -m api-server -s kafka-1,kafka-3         # Multiple servers (comma-separated)
tow logs -e prod -m api-server --all                      # All servers simultaneously
tow logs -e prod -m api-server -F                         # Follow (live tail)
```

#### 로그 프리셋

자주 쓰는 로그 쿼리를 저장하고 재사용합니다:

```bash
tow logs -e prod -m api-server -f "ERROR" --save-preset error-logs
tow logs --preset error-logs                              # Replay saved query
tow logs --list-presets                                   # List all saved presets
tow logs --delete-preset error-logs                       # Remove a preset
```

| 플래그 | 축약형 | 설명 |
|------|-------|-------------|
| `--all` | | 모든 서버의 로그를 동시에 스트리밍 |
| `-s` | | 대상 서버 (쉼표로 구분, 예: `kafka-1,kafka-3`) |
| `-F` | | Follow 모드 — 로그 출력 실시간 tail |
| `--preset` | | 저장된 로그 쿼리 프리셋 재실행 |
| `--save-preset` | | 현재 쿼리를 이름 붙여 프리셋으로 저장 |
| `--list-presets` | | 저장된 모든 로그 프리셋 목록 표시 |
| `--delete-preset` | | 저장된 로그 프리셋 삭제 |

#### 멀티모듈 로그

여러 모듈의 로그를 동시에 스트리밍합니다:

```bash
tow logs -e prod -m kafka,zookeeper --all -n 5    # multiple modules
tow logs -e prod -m kafka,zookeeper --all -F      # stream multiple modules
```

각 줄 앞에 `[module/server]` 접두사가 붙습니다.

### `tow ssh`

인터랙티브 세션을 열지 않고 SSH로 원격 서버에서 애드혹 명령을 실행합니다.

```bash
tow ssh -e prod -m kafka --all -- "free -h"
tow ssh -e prod -m api-server -s 1 -- "df -h"
tow ssh -e prod -m kafka -s kafka-1,kafka-3 -- "cat /etc/os-release"
```

`--` 뒤의 명령어는 매칭된 모든 서버에서 실행됩니다. 가독성을 위해 출력 앞에 서버 이름이 붙습니다.

### `tow diff`

원격 서버에 배포된 코드를 로컬 빌드 아티팩트 또는 소스와 비교합니다.

```bash
tow diff -e prod -m api-server
tow diff -e prod -m api-server -s 1
```

현재 배포된 버전과 로컬 아티팩트 사이에 차이가 나는 파일의 요약을 보여줍니다. `tow deploy` 실행 전에 무엇이 바뀔지 확인하는 데 유용합니다.

### `tow config`

명령줄에서 `tow.yaml`의 서버, 모듈, 할당을 관리합니다.

#### 서버 관리

```bash
tow config server add -e prod --name api-3 --host 10.0.1.30
tow config server remove -e prod --name api-3
tow config server list -e prod
```

#### 모듈 관리

```bash
tow config module add --name payment-api --type springboot --port 8083
tow config module remove --name payment-api
tow config module list
```

#### 모듈을 서버에 할당/해제

```bash
tow config assign -e prod --server api-3 --module payment-api
tow config unassign -e prod --server api-3 --module payment-api
```

### `tow setup`

원격 서버 디렉토리 구조를 초기화합니다.

```bash
tow setup -e dev
tow setup -e prod -m api-server
```

각 서버에 생성되는 항목:

```
{base_dir}/{module}-{server_number}/
├── deploy/      # Deployment versions
├── upload/      # Upload staging
├── log/         # Persistent logs
├── conf/        # Persistent configuration
└── {data_dirs}  # Custom data directories
```

### `tow upload`

파일을 원격 서버로 전송합니다.

```bash
tow upload -e prod -m api-server
tow upload -e prod -m api-server path/to/file.tar.gz
```

### `tow install`

업로드된 패키지를 추출하고 `current` 심링크를 갱신합니다.

```bash
tow install -e prod -m api-server
```

`deploy/`에 타임스탬프가 찍힌 새 디렉토리를 생성하고, `current` 심링크를 갱신하며, 영구 심링크를 생성합니다:

- `current/log` → `{baseDir}/log`
- `current/{data_dir}` → `{baseDir}/{data_dir}` (`data_dirs`의 각 항목에 대해)

이를 통해 별도 설정 없이도 로그와 데이터가 배포를 거쳐도 유지됩니다.

추출과 심링크 적용 후, `post_install` 훅이 정의되어 있으면 실행됩니다 (예: 가상환경 생성, `pip install` 실행, 데이터베이스 마이그레이션 실행).

### `tow list`

설정 정보를 나열합니다.

```bash
tow list modules       # List all modules
tow list envs          # List all environments
tow list deployments -e prod -m api-server  # List deployment history
```

### `tow login`

서버에 대한 인터랙티브 SSH 세션을 엽니다.

```bash
tow login -e prod -m api-server -s 1
```

> **팁**: 여러 서버나 모듈이 매칭되면 Tow는 인터랙티브 선택 화면을 보여줍니다. 서버를 이름으로 지정하려면 `-s`를, 모듈을 지정하려면 `-m`을 사용하세요. 둘 중 하나만 주어지면 Tow가 나머지를 인터랙티브하게 선택하게 해줍니다.

### `tow unlock`

배포 락을 강제로 해제합니다.

```bash
tow unlock -e prod -m api-server
```

배포가 중단되어 락이 정리되지 않았을 때 사용하세요.

### `tow cleanup`

원격 서버에서 오래된 배포 디렉토리를 제거합니다.

```bash
tow cleanup -e prod -m api-server              # Keep default (from config or 5)
tow cleanup -e prod -m api-server --keep 3     # Keep only 3 most recent
```

현재 활성 배포는 절대 제거되지 않습니다. `tow.yaml`에서 자동 정리를 설정할 수 있습니다:

```yaml
retention:
  keep: 5
  auto_cleanup: true    # Clean up after every deploy/auto
```

### `tow download`

원격 서버에서 로컬로 파일을 다운로드합니다.

```bash
tow download -e prod -m api-server logs/std.log
tow download -e prod -m api-server /var/log/syslog -d ./local-logs/
```

경로가 상대 경로면 모듈 기준 디렉토리를 기준으로 해석됩니다.

### `tow provision`

새 서버에 기본 요구사항을 프로비저닝합니다.

```bash
tow provision -e prod -m api-server --timezone Asia/Seoul --locale en_US.UTF-8 --jre --tools
```

| 플래그 | 설명 |
|------|-------------|
| `--timezone` | 서버 타임존 설정 (예: `Asia/Seoul`) |
| `--locale` | 서버 로케일 설정 (예: `en_US.UTF-8`) |
| `--jre` | Java Runtime Environment 설치 |
| `--tools` | 필수 도구 설치 (lsof, nc, curl, tar) |

이 명령어는 배포 디렉토리 구조를 만들기 위해 `tow setup`도 함께 실행합니다.

### `tow threaddump`

디버깅을 위해 Java/Spring Boot 모듈에서 스레드 덤프를 트리거합니다.

```bash
tow threaddump -e prod -m api-server
tow threaddump -e prod -m api-server -s 1
```

JVM 프로세스에 `kill -3`을 보냅니다. 스레드 덤프 출력은 애플리케이션 로그 파일에 기록됩니다.

> 타입이 `springboot` 또는 `java`인 모듈에서만 사용할 수 있습니다.

### `tow plugin`

인프라 플러그인을 관리합니다.

```bash
# List all installed plugins (bundled + external)
tow plugin list

# Install bundled plugins to ~/.tow/plugins/ (for customization)
tow plugin install kafka redis
tow plugin install --all

# Add community plugins from GitHub or URL
tow plugin add someuser/tow-plugin-mssql
tow plugin add myorg/infra-plugins/oracle.yaml
tow plugin add https://example.com/my-plugin.yaml

# Remove
tow plugin remove mssql
```

35개 플러그인이 바이너리에 내장되어 있습니다. 커뮤니티 플러그인은 `~/.tow/plugins/`에 설치됩니다.

전체 목록과 플러그인 작성 가이드는 [plugins/README.md](https://github.com/neurosamAI/tow-cli/tree/main/plugins)를 참고하세요.

### `tow metrics`

감사 로그를 기반으로 배포 통계를 보여줍니다.

```bash
tow metrics                           # last 30 days, all modules
tow metrics -e prod                   # filter by environment
tow metrics -m api-server             # filter by module
tow metrics --days 7                  # last 7 days
```

전체 배포 수, 액션별(deploy/auto/rollback) 분류, 모듈별(막대 그래프 포함) 분류, 요일별 분류를 보여줍니다.

---

### `tow doctor`

사전 점검 진단 — 설정, SSH, 원격 디렉토리, 디스크 공간, 배포 락, 서버 의존성을 검사합니다.

```bash
tow doctor -e prod -m api-server --insecure

  ✓ tow.yaml is valid
  ✓ Environment 'prod' exists
  ✓ SSH key exists
  ✓ Servers configured (22)
  ✓ SSH connection to 10.0.1.10
  ✓ Remote dir exists: /home/ec2-user/api-server-1
  ✓ Disk space — Available: 15GB
  ✓ Branch policy
  ✓ No active deploy lock
  ✓ Server dependencies OK
```

#### 서버 의존성 검사

`tow doctor`는 각 원격 서버에 필요한 도구가 있는지 확인합니다:

| 도구 | 상태 | 사용처 |
|------|--------|---------|
| `bash` | 필수 | 모든 명령어 (서버 스크립트) |
| `tar` | 필수 | `install`, `deploy`, `auto` (패키지 추출) |
| `lsof` | 필수 | `status`, `stop`, `doctor` (포트/프로세스 확인) |
| `curl` | 필수 | `health_check` (HTTP 타입), `provision` |
| `nc` | 권장 | `health_check` (TCP 타입) |

필수 도구가 없으면 설치 안내와 함께 에러로 보고됩니다:

```
  ✗ Missing required: lsof
    → Install: sudo apt install lsof  (or: sudo yum install lsof)
    → Affects: tow status, tow stop, tow doctor
  ⚠ Missing recommended: nc
    → Install: sudo apt install netcat-openbsd  (or: sudo yum install nmap-ncat)
    → Affects: tow start (TCP health check)
```

`-m` 없이 실행하면 환경 레벨의 연결 상태를 확인합니다.

---

### `tow mcp-server`

AI 에이전트 연동을 위한 MCP(Model Context Protocol) 서버를 시작합니다.

```bash
tow mcp-server
tow mcp-server -c /path/to/tow.yaml
```

이를 통해 Tow의 작업들이 AI 어시스턴트(Claude, Cursor, Windsurf)를 위한 도구로 노출됩니다. 아래 [AI 에이전트 연동](#ai-에이전트-연동)을 참고하세요.

## 고급 플래그

### 롤링 배포

한 번에 서버 하나씩 배포하고, 다음으로 넘어가기 전에 헬스체크로 확인합니다:

```bash
tow deploy -e prod -m api-server --rolling
tow auto -e prod -m api-server --rolling
tow start -e prod -m api-server --rolling
```

### 자동 롤백

시작 후 헬스체크가 실패하면 자동으로 이전 배포로 되돌립니다:

```bash
tow auto -e prod -m api-server --auto-rollback
```

### 프로덕션 확인

프로덕션 환경(`prod`, `production`, `live`)은 인터랙티브 확인이 필요합니다:

```
⚠  WARNING: You are about to deploy api-server in PROD
  Type 'yes' to confirm:
```

CI/CD에서는 `-y` / `--yes` 플래그로 건너뛸 수 있습니다:

```bash
tow deploy -e prod -m api-server -y
```

### JSON 출력

CI/CD 파이프라인을 위한 머신 리더블 출력입니다:

```bash
tow status -e prod -m api-server -o json
tow list deployments -e prod -m api-server -o json
```

## AI 에이전트 연동

### MCP 서버

Claude Desktop, Cursor, 또는 MCP를 지원하는 클라이언트에서 설정하세요:

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

**사용 가능한 MCP 도구:**

| 도구 | 설명 |
|------|-------------|
| `tow_status` | 모듈 상태 확인 (PID, 업타임, 메모리) |
| `tow_deploy` | 모듈 배포 (프로덕션 안전 게이트 포함) |
| `tow_rollback` | 이전 배포로 롤백 |
| `tow_logs` | 최근 로그 라인 조회 |
| `tow_list_deployments` | 배포 이력 조회 |
| `tow_list_modules` | 설정된 모듈 목록 조회 |
| `tow_list_environments` | 설정된 환경 목록 조회 |

## 명령어 파이프라인

| 명령어 | build | package | upload | install | stop | start | 헬스체크 |
|---------|:-----:|:-------:|:------:|:-------:|:----:|:-----:|:------------:|
| `auto` | ● | ● | ● | ● | ● | ● | ● |
| `deploy` | | ● | ● | ● | ● | ● | ● |
| `start` | | | | | | ● | ● |
| `stop` | | | | | ● | | |
| `restart` | | | | | ● | ● | ● |
| `rollback` | | | | ● | ● | ● | ● |
| `upload` | | | ● | | | | |
| `install` | | | | ● | | | |

> **참고**: `install` 단계는 추출과 심링크 생성 후 `post_install` 훅을 실행합니다. 가상환경 생성, `pip install` 실행, 데이터베이스 마이그레이션 실행과 같은 환경 설정 작업에 사용하세요.

## Dry-Run 모드

어떤 명령어에든 `--dry-run`을 추가하면 실행 없이 동작을 미리 볼 수 있습니다:

```bash
tow auto -e prod -m api-server --dry-run
```

```
[1/6] Building api-server...
  → Would run: ./gradlew :api-server:bootJar
[2/6] Packaging api-server...
  → Would create: build/api-server.tar.gz
[3/6] Uploading to prod-1 (52.78.100.1)...
  → Would SCP: build/api-server.tar.gz → ~/upload/
[4/6] Installing on prod-1...
  → Would extract to: deploy/20240626-143022/
  → Would update symlink: current → deploy/20240626-143022
[5/6] Stopping api-server on prod-1...
  → Would run: bin/server stop
[6/6] Starting api-server on prod-1...
  → Would run: bin/server start
```
