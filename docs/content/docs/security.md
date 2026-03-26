---
title: Secrets & Security
---

# Secrets & Security

Tow is designed with security-first principles. This page covers how to keep your deployments secure.

## SSH Authentication

Tow supports three SSH authentication methods:

### SSH Key (Default)

```yaml
defaults:
  ssh_key_path: ~/.ssh/deploy.pem

# Per-environment override
environments:
  prod:
    ssh_key_path: ~/.ssh/prod-deploy.pem
```

Best practices:
- Use dedicated deploy keys, not personal SSH keys
- Set restrictive permissions: `chmod 600 ~/.ssh/deploy.pem`
- Rotate keys regularly
- Use different keys per environment

### SSH Agent

```yaml
modules:
  api-server:
    ssh:
      auth: agent    # Uses SSH_AUTH_SOCK
```

Ideal for:
- CI/CD pipelines where keys are injected into the agent
- Developers who use `ssh-add` for key management
- Systems with hardware security keys (YubiKey, etc.)

### Password Authentication

```yaml
modules:
  api-server:
    ssh:
      auth: password
      password: ${DEPLOY_PASSWORD}    # Always use env vars!
```

> **Warning**: Password auth is the least secure option. Use it only when key-based auth is not available. Never hardcode passwords in `tow.yaml`.

## Secrets Management

### Rule #1: Never Commit Secrets

Tow provides multiple mechanisms to keep secrets out of version control:

#### Environment Variables

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

#### Local Overrides (`tow.local.yaml`)

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

#### Application Config Secrets

Your application config files (e.g., `application.yml`, `redis.conf`) often contain secrets like database passwords and API keys. Tow automatically expands `${VAR}` environment variables in config files during deployment:

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

At deploy time, Tow reads your environment variables and substitutes them before uploading to the server. The actual secrets never touch git.

```bash
# Set secrets as environment variables
export DB_PASSWORD="my-secret-password"
export REDIS_PASSWORD="redis-secret"
export API_SECRET_KEY="sk-xxxxx"

# Deploy — secrets are injected into config files automatically
tow deploy -e prod -m api-server
```

> **Note**: If a `${VAR}` is not set in your environment, it will be replaced with an empty string. Tow does not validate that all variables are set — this is your responsibility.

#### `.gitignore` Defaults

Tow's default `.gitignore` excludes:

```
*.pem
*.key
credentials.*
tow.local.yaml
```

## Host Key Verification

By default, Tow verifies server identities using `~/.ssh/known_hosts`:

```bash
# Normal mode — verifies host keys
tow deploy -e prod -m api-server

# Insecure mode — skips verification (not recommended for production)
tow deploy -e prod -m api-server --insecure
```

> **Recommendation**: Always maintain an up-to-date `known_hosts` file. Run `ssh-keyscan your-server >> ~/.ssh/known_hosts` for new servers.

## Production Confirmation Prompt

Tow automatically asks for confirmation when deploying to production-like environments (`prod`, `production`, `live`):

```
⚠  WARNING: You are about to deploy api-server in PROD
  Type 'yes' to confirm:
```

Skip with the `-y` / `--yes` flag (for CI/CD pipelines):

```bash
tow deploy -e prod -m api-server -y
tow auto -e prod -m api-server --auto-rollback -y
```

This prevents accidental production deployments from a quick `tow deploy` typo.

## Branch Policies

Prevent accidental deployments from wrong branches:

### Simple Mode

```yaml
environments:
  prod:
    branch: main    # Only allow deploys from 'main' branch
```

### Advanced Mode

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

This prevents scenarios like deploying a feature branch to production.

## Deploy Locking

Tow uses atomic lock files to prevent concurrent deployments:

```
Developer A: tow deploy -e prod → acquires lock → deploys → releases lock
Developer B: tow deploy -e prod → "Deploy locked by alice@machine since 14:30" → blocked
```

This prevents race conditions where two developers deploy simultaneously, potentially causing inconsistent states.

If a deploy is interrupted and the lock isn't released:

```bash
tow unlock -e prod -m api-server
```

## Per-Module SSH Configuration

Different modules can use different credentials:

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

This supports the principle of least privilege — each service uses only the credentials it needs.

## Network Security Recommendations

### Firewall Rules

- Restrict SSH access to deployment machines only
- Use a bastion/jump host for production environments
- Close all ports except those required by your applications

### SSH Hardening

On your target servers:

```bash
# /etc/ssh/sshd_config
PasswordAuthentication no          # Disable password auth
PermitRootLogin no                 # Disable root login
AllowUsers ec2-user deploy-user   # Whitelist users
MaxAuthTries 3                    # Limit auth attempts
```

### CI/CD Integration

When using Tow in CI/CD pipelines:

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

## Security Checklist

- [ ] SSH keys have `600` permissions
- [ ] `tow.local.yaml` is in `.gitignore`
- [ ] No secrets hardcoded in `tow.yaml`
- [ ] Production uses `branch` or `branch_policy` restriction
- [ ] `--insecure` flag is not used in production
- [ ] Different SSH keys per environment
- [ ] Deploy user has minimal required permissions
- [ ] `~/.ssh/known_hosts` is maintained for all servers
- [ ] CI/CD secrets are stored in the platform's secret manager
