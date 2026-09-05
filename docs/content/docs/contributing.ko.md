---
title: 기여하기
---

# 기여하기

Tow는 MIT 라이선스로 공개된 오픈소스입니다. 코드, 문서, 버그 리포트, 기능 요청 등 모든 종류의 기여를 환영합니다.

## 시작하기

### 1. Fork하고 Clone하기

```bash
git clone https://github.com/your-username/tow-cli.git
cd tow-cli
```

### 2. 의존성 설치

```bash
go mod download
```

### 3. 빌드

```bash
make build       # Build binary to ./bin/tow
make install     # Install to $GOPATH/bin
```

### 4. 테스트

```bash
make test        # Run all tests
make lint        # Run golangci-lint
```

## 프로젝트 구조

```
tow-cli/
├── cmd/tow/            # CLI entry point (Cobra commands)
│   └── main.go
├── internal/
│   ├── config/         # Configuration loading and validation
│   ├── deploy/         # Deployment operations (upload, install, rollback)
│   ├── initializer/    # Project auto-detection and scaffolding
│   ├── logger/         # Colored logging system
│   ├── module/         # Module type handlers (springboot, node, etc.)
│   ├── pipeline/       # Multi-step deployment workflows
│   └── ssh/            # SSH connection management
├── docs/               # GitHub Pages documentation (this site)
├── examples/           # Example configuration files
├── Makefile            # Build automation
└── go.mod              # Go module definition
```

## 개발 가이드라인

### 코드 스타일

- 표준 Go 컨벤션을 따르세요 (`gofmt`, `golangci-lint`)
- 패키지는 집중도 있게 유지하세요 — 각 패키지는 단일 책임을 가집니다
- 영리한 코드보다 명시적인 코드를 선호하세요
- 에러 메시지는 실행 가능해야 합니다: "key error"보다는 "SSH key not found at ~/.ssh/deploy.pem" 같은 식으로

### 새 모듈 타입 추가하기

1. `internal/module/handler.go`에 핸들러를 추가합니다:

```go
func init() {
    Register("mytype", Handler{
        DefaultBuildCmd:    "my-build-command",
        DefaultStartCmd:    "bin/server start",
        DefaultStopCmd:     "bin/server stop",
        DefaultArtifact:    "build/{module}.tar.gz",
        DefaultHealthCheck: config.HealthCheckConfig{Type: "tcp"},
    })
}
```

2. `internal/initializer/initializer.go`에 감지 로직을 추가합니다
3. `internal/initializer/initializer_test.go`에 테스트를 추가합니다
4. 문서를 업데이트합니다

### 새 명령어 추가하기

1. `cmd/tow/main.go`에 Cobra 명령어를 추가합니다
2. 적절한 `internal/` 패키지에 로직을 구현합니다
3. 테스트를 추가합니다
4. [명령어](/ko/docs/commands/) 문서를 업데이트합니다

### 테스트 작성하기

- Config 테스트는 `internal/config/config_test.go`에 작성합니다
- Initializer 테스트는 `internal/initializer/initializer_test.go`에 작성합니다
- 적절한 곳에는 테이블 기반 테스트를 사용하세요
- 배포 테스트에는 SSH 연결을 모킹하세요

## 기여 유형

### 버그 리포트

다음 내용으로 이슈를 열어주세요:
- 예상했던 동작
- 실제 일어난 일
- 재현 단계
- Tow 버전 (`tow --version`)
- OS와 Go 버전

### 기능 요청

다음을 설명하는 이슈를 열어주세요:
- 사용 사례 (무엇을 이루려고 하시나요?)
- 제안하는 해결책
- 고려했던 대안

### Pull Request

1. `main`에서 피처 브랜치를 생성합니다
2. 테스트와 함께 변경 사항을 작성합니다
3. `make test`와 `make lint`가 통과하는지 확인합니다
4. 명확한 PR 설명을 작성합니다
5. 관련 이슈를 참조합니다

## 로드맵

우리는 Tow를 베어메탈과 VM 환경의 팀들이 가장 먼저 찾는 배포 도구로 만들어가고 있습니다. 특히 기여를 환영하는 영역은 다음과 같습니다:

- **롤링 배포 전략** — 헬스체크 게이트와 함께 서버에 하나씩 배포
- **블루-그린 배포** — 두 서버 집합 사이에서 트래픽 전환
- **플러그인 시스템** — 커뮤니티가 만든 모듈 핸들러 허용
- **웹 대시보드** — 실시간 배포 상태와 이력
- **알림 연동** — Slack, Discord, Teams, PagerDuty
- **배포 메트릭** — 배포 빈도, 소요 시간, 실패율 추적
- **다중 리전 지원** — 리전 간 배포 조정
- **설정 암호화** — `tow.yaml`의 민감한 값 암호화

## 커뮤니티

- **GitHub Issues**: [github.com/neurosamAI/tow-cli/issues](https://github.com/neurosamAI/tow-cli/issues)
- **Discussions**: [github.com/neurosamAI/tow-cli/discussions](https://github.com/neurosamAI/tow-cli/discussions)

## 라이선스

Tow는 [MIT 라이선스](https://github.com/neurosamAI/tow-cli/blob/main/LICENSE)로 배포됩니다.
