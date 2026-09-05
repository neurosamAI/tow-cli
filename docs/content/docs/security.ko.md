---
title: 시크릿 & 보안
---

# 시크릿 & 보안

Tow는 보안 우선 원칙으로 설계되었습니다. 이 페이지는 배포를 안전하게 유지하는 방법을 다룹니다.

## SSH 인증

Tow는 세 가지 SSH 인증 방식을 지원합니다:

### SSH 키 (기본값)

```yaml
defaults:
  ssh_key_path: ~/.ssh/deploy.pem

# Per-environment override
environments:
  prod:
    ssh_key_path: ~/.ssh/prod-deploy.pem
```

모범 사례:
- 개인 SSH 키가 아닌 전용 배포 키를 사용하세요
- 제한적인 권한을 설정하세요: `chmod 600 ~/.ssh/deploy.pem`
- 키를 정기적으로 교체하세요
- 환경마다 다른 키를 사용하세요

### SSH 에이전트

```yaml
modules:
  api-server:
    ssh:
      auth: agent    # Uses SSH_AUTH_SOCK
```

다음과 같은 경우에 적합합니다:
- 키가 에이전트에 주입되는 CI/CD 파이프라인
- 키 관리에 `ssh-add`를 사용하는 개발자
- 하드웨어 보안 키(YubiKey 등)를 사용하는 시스템

### 비밀번호 인증

```yaml
modules:
  api-server:
    ssh:
      auth: password
      password: ${DEPLOY_PASSWORD}    # Always use env vars!
```

> **경고**: 비밀번호 인증은 가장 안전하지 않은 옵션입니다. 키 기반 인증을 사용할 수 없을 때만 사용하세요. `tow.yaml`에 비밀번호를 하드코딩하지 마세요.

## 시크릿 관리

### 규칙 #1: 시크릿을 절대 커밋하지 않는다

Tow는 시크릿을 버전 관리 밖에 두기 위한 여러 메커니즘을 제공합니다:

#### 환경 변수

```yaml
environments:
  prod:
    servers:
      - number: 1
        host: ${PROD_SERVER_IP}

modules:
  api-server:
    ssh:
      password: ${DEPLOY_PASSWORD}
    variables:
      DB_PASSWORD: ${DB_PASSWORD}
```

#### 로컬 오버라이드 (`tow.local.yaml`)

```yaml
# tow.local.yaml — always in .gitignore
defaults:
  ssh_key_path: ~/.ssh/my-personal-key.pem

environments:
  dev:
    servers:
      - number: 1
        host: 192.168.1.100
```

#### 애플리케이션 설정 시크릿

애플리케이션 설정 파일(예: `application.yml`, `redis.conf`)에는 데이터베이스 비밀번호나 API 키 같은 시크릿이 흔히 포함됩니다. Tow는 배포 중에 설정 파일 안의 `${VAR}` 환경 변수를 자동으로 확장합니다:

```yaml
# config/prod/application.yml (safe to commit — no real secrets)
spring:
  datasource:
    url: jdbc:mysql://${DB_HOST}:3306/mydb
    password: ${DB_PASSWORD}
  redis:
    password: ${REDIS_PASSWORD}

external-api:
  key: ${API_SECRET_KEY}
```

배포 시점에 Tow는 환경 변수를 읽어서 서버에 업로드하기 전에 치환합니다. 실제 시크릿은 git에 절대 남지 않습니다.

```bash
# Set secrets as environment variables
export DB_PASSWORD="my-secret-password"
export REDIS_PASSWORD="redis-secret"
export API_SECRET_KEY="sk-xxxxx"

# Deploy — secrets are injected into config files automatically
tow deploy -e prod -m api-server
```

> **참고**: 환경에 `${VAR}`가 설정되어 있지 않으면 빈 문자열로 치환됩니다. Tow는 모든 변수가 설정되어 있는지 검증하지 않습니다 — 이는 사용자의 책임입니다.

#### `.gitignore` 기본값

Tow의 기본 `.gitignore`는 다음을 제외합니다:

```
*.pem
*.key
credentials.*
tow.local.yaml
```

## 호스트 키 검증

기본적으로 Tow는 `~/.ssh/known_hosts`를 사용해서 서버 신원을 검증합니다:

```bash
# Normal mode — verifies host keys
tow deploy -e prod -m api-server

# Insecure mode — skips verification (not recommended for production)
tow deploy -e prod -m api-server --insecure
```

> **권장 사항**: 항상 최신 상태의 `known_hosts` 파일을 유지하세요. 새 서버에는 `ssh-keyscan your-server >> ~/.ssh/known_hosts`를 실행하세요.

## 프로덕션 확인 프롬프트

Tow는 프로덕션과 유사한 환경(`prod`, `production`, `live`)에 배포할 때 자동으로 확인을 요청합니다:

```
⚠  WARNING: You are about to deploy api-server in PROD
  Type 'yes' to confirm:
```

CI/CD 파이프라인에서는 `-y` / `--yes` 플래그로 건너뛸 수 있습니다:

```bash
tow deploy -e prod -m api-server -y
tow auto -e prod -m api-server --auto-rollback -y
```

이는 `tow deploy` 오타로 인한 실수 배포를 방지합니다.

## 브랜치 정책

잘못된 브랜치에서의 실수 배포를 방지합니다:

### 단순 모드

```yaml
environments:
  prod:
    branch: main    # Only allow deploys from 'main' branch
```

### 고급 모드

```yaml
environments:
  prod:
    branch_policy:
      allowed:
        - main
        - release/*
        - hotfix/*
      commands:
        - deploy
        - auto
        - start
        - restart
        - rollback
```

이는 피처 브랜치를 프로덕션에 배포하는 것과 같은 시나리오를 방지합니다.

## 배포 락

Tow는 원자적 락 파일을 사용해서 동시 배포를 방지합니다:

```
Developer A: tow deploy -e prod → acquires lock → deploys → releases lock
Developer B: tow deploy -e prod → "Deploy locked by alice@machine since 14:30" → blocked
```

이는 두 명의 개발자가 동시에 배포해서 상태가 불일치하게 되는 경쟁 조건을 방지합니다.

배포가 중단되어 락이 해제되지 않았다면:

```bash
tow unlock -e prod -m api-server
```

## 모듈별 SSH 설정

모듈마다 다른 자격 증명을 사용할 수 있습니다:

```yaml
modules:
  api-server:
    ssh:
      user: app-deploy
      key_path: ~/.ssh/app-key.pem

  kafka:
    ssh:
      user: kafka-admin
      port: 2222
      key_path: ~/.ssh/infra-key.pem
```

이는 최소 권한 원칙을 지원합니다 — 각 서비스는 필요한 자격 증명만 사용합니다.

## 네트워크 보안 권장 사항

### 방화벽 규칙

- SSH 접근을 배포 머신으로만 제한하세요
- 프로덕션 환경에는 배스천/점프 호스트를 사용하세요
- 애플리케이션에 필요한 포트를 제외한 모든 포트를 닫으세요

### SSH 강화

대상 서버에서:

```bash
# /etc/ssh/sshd_config
PasswordAuthentication no          # Disable password auth
PermitRootLogin no                 # Disable root login
AllowUsers ec2-user deploy-user   # Whitelist users
MaxAuthTries 3                    # Limit auth attempts
```

### CI/CD 연동

CI/CD 파이프라인에서 Tow를 사용할 때:

```yaml
# GitHub Actions example
- name: Deploy to production
  env:
    PROD_SERVER_1: ${{ secrets.PROD_SERVER_1 }}
    DEPLOY_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
  run: |
    echo "$DEPLOY_KEY" > /tmp/deploy.pem
    chmod 600 /tmp/deploy.pem
    tow deploy -e prod -m api-server -c tow.yaml
    rm /tmp/deploy.pem
```

## 보안 체크리스트

- [ ] SSH 키가 `600` 권한을 가지고 있다
- [ ] `tow.local.yaml`이 `.gitignore`에 포함되어 있다
- [ ] `tow.yaml`에 하드코딩된 시크릿이 없다
- [ ] 프로덕션이 `branch` 또는 `branch_policy` 제한을 사용한다
- [ ] 프로덕션에서 `--insecure` 플래그를 사용하지 않는다
- [ ] 환경마다 다른 SSH 키를 사용한다
- [ ] 배포 사용자가 필요한 최소 권한만 가지고 있다
- [ ] 모든 서버에 대해 `~/.ssh/known_hosts`가 유지 관리된다
- [ ] CI/CD 시크릿이 플랫폼의 시크릿 매니저에 저장되어 있다
