---
title: Home
---

<div class="hero">
  <h1>Tow</h1>
  <p class="subtitle">Deploy to bare-metal servers and VMs — without the complexity of Kubernetes.</p>
  <div class="hero-buttons">
    <a href="/docs/getting-started/" class="btn btn-primary">Get Started</a>
    <a href="https://github.com/neurosamAI/tow-cli" class="btn btn-secondary">GitHub</a>
  </div>
</div>

## What is Tow?

Tow is a **lightweight, agentless deployment orchestrator** written in Go. It deploys your applications to bare-metal servers or cloud VMs via SSH — no agents, no containers, no Kubernetes required.

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
    <h3>Zero Dependencies</h3>
    <p>Single binary. Only requires SSH access to your servers. No agents, no runtime, no Docker.</p>
  </div>
  <div class="feature">
    <h3>Auto-Detection</h3>
    <p><code>tow init</code> scans your project and generates config for 10 languages, 40+ frameworks. Plus 35 infrastructure plugins (Kafka, Redis, MySQL, etc.).</p>
  </div>
  <div class="feature">
    <h3>Instant Rollback</h3>
    <p>Symlink-based atomic deployments. Switch to any previous version in milliseconds.</p>
  </div>
  <div class="feature">
    <h3>Secure by Default</h3>
    <p>Secrets never touch git. Environment variables, local overrides, SSH agent, and known_hosts verification built-in.</p>
  </div>
  <div class="feature">
    <h3>Multi-Environment</h3>
    <p>Dev, staging, production — each with its own servers, SSH keys, and branch policies.</p>
  </div>
  <div class="feature">
    <h3>Parallel & Safe</h3>
    <p>Deploy to multiple servers simultaneously. Built-in deploy locks prevent concurrent deploys.</p>
  </div>
  <div class="feature">
    <h3>Multi-Server Ops</h3>
    <p><code>tow ssh</code>, <code>tow logs --all</code>, <code>tow diff</code> — run ad-hoc commands, stream logs from all servers, and compare deployed vs local code.</p>
  </div>
  <div class="feature">
    <h3>AI Agent Native</h3>
    <p>Built-in MCP Server for Claude, Cursor, Windsurf. Deploy and monitor with natural language commands.</p>
  </div>
</div>

## How It Works

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

Rollback is instant — just switch the `current` symlink to any previous deployment directory.

## Quick Start

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

## Why Not...?

| | Tow | Ansible | Capistrano | Kamal |
|---|:---:|:---:|:---:|:---:|
| Single binary, zero deps | **Yes** | Python + pip | Ruby + Bundler | Ruby + Docker |
| Auto-detection | **Yes** | No | No | No |
| No Docker required | **Yes** | Yes | Yes | **No** |
| Multi-language native | **12 types** | Manual | Ruby-first | Docker-only |
| Built-in health checks | **4 types** | Manual | Plugin | HTTP only |
| Instant rollback | **Symlink** | Re-run playbook | Symlink | Container swap |
| Multi-server log streaming | **Yes** | No | No | No |
| Pre-deploy diff | **Yes** | No | No | No |
| AI agent support (MCP) | **Yes** | No | No | No |

<p style="text-align: center; margin-top: 1rem;">
  <a href="/docs/comparison/" style="color: #60a5fa;">See full comparison →</a>
</p>

## Real-World Scenarios

### "We have 3 Spring Boot services, Kafka, and Redis on 5 EC2 instances"

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

With Ansible, this would require playbooks, roles, inventory files, and Jinja templates. With Tow, it's **one YAML file and one command**.

### "Someone deployed a feature branch to production last Friday"

```yaml
environments:
  prod:
    branch_policy:
      allowed: [main, hotfix/*]
```

Tow blocks the deploy before it starts. No post-mortem needed.

### "Our new developer spent 2 days writing deployment scripts"

```bash
tow init    # 5 seconds. Done.
```

Tow auto-detects everything — language, framework, build tool, monorepo modules — and generates a complete, working config.

## Who Is This For?

- Teams deploying to **EC2, DigitalOcean, Hetzner**, or any Linux VMs
- Projects that don't need (or aren't ready for) Kubernetes
- Developers who want **reliable deployments without learning Ansible/Terraform**
- Startups that need **something between manual SSH and full orchestration**
- **Polyglot teams** running Java, Node.js, Python, Go, and Rust services together

## Born from Real Experience

Tow is the Go rewrite of bash-based deployment systems used across multiple companies and projects — managing Spring Boot microservices, Kafka clusters, Redis, and monitoring stacks. The patterns are proven; the Go implementation has been validated on 22 production servers.

Every feature exists because a real production incident demanded it:

| Feature | The incident that demanded it |
|---------|-------------------------------|
| **Deploy locking** | Two developers deployed at the same time → inconsistent state across servers |
| **Branch policies** | Feature branch accidentally deployed to production on a Friday evening |
| **Health checks** | Deploy "succeeded" but app was crash-looping — nobody noticed for 30 minutes |
| **Hierarchical config** | Server 1 needed different JVM heap settings than server 2, but shared everything else |
| **Instant rollback** | Bad deploy took down the API — took 15 minutes to rebuild and redeploy the previous version |

<div class="line-glow"></div>

<p class="brand-footer">
  Created by <a href="https://github.com/comchangs">Murry Jeong</a> &middot; Supported by <a href="https://neurosam.ai">neurosam.AI</a> &middot; MIT License &middot; <a href="https://oss.neurosam.ai">Open Source</a>
</p>
