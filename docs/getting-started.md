---
layout: default
title: Getting Started
---

# Getting Started

Get up and running with Tow in under 5 minutes.

## Prerequisites

- **Go 1.21+** (for `go install`) or download the pre-built binary
- **SSH access** to your target servers (key-based or agent)
- A project to deploy (Java, Node.js, Python, Go, Rust, or any custom application)

## Installation

### Option 1: Homebrew (Recommended for macOS / Linux)

```bash
brew install neurosamAI/tap/tow
```

### Option 2: Bash Installer

```bash
curl -fsSL https://raw.githubusercontent.com/neurosamAI/tow-cli/main/install.sh | bash
```

Auto-detects your platform and installs the latest version.

### Option 3: npm

```bash
npm install -g @neurosamai/tow
```

### Option 4: Go Install

```bash
go install github.com/neurosamAI/tow-cli/cmd/tow@latest
```

### Option 5: Download Binary

```bash
# macOS (Apple Silicon)
curl -L https://github.com/neurosamAI/tow-cli/releases/latest/download/tow-darwin-arm64 -o tow
chmod +x tow && sudo mv tow /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/neurosamAI/tow-cli/releases/latest/download/tow-linux-amd64 -o tow
chmod +x tow && sudo mv tow /usr/local/bin/
```

### Option 6: Build from Source

```bash
git clone https://github.com/neurosamAI/tow-cli.git
cd tow-cli
make build
# Binary is at ./bin/tow
```

## Quick Start

### Step 1: Initialize Your Project

Navigate to your project directory and run:

```bash
cd my-project
tow init

# Or with AI agent integration (Claude Code + MCP server):
tow init --with-ai
```

Tow automatically detects your project type, framework, build tool, and generates:
- **`tow.yaml`** — deployment configuration
- **`script/`** — server control scripts (`env.sh`, `server`)
- With `--with-ai`: **`.claude/skills/tow-deploy.md`** + **`.claude/settings.json`** for AI agent integration

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

### Step 2: Configure Your Servers

Edit `tow.yaml` to set your server details:

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

### Step 3: Initialize Remote Servers

Set up the directory structure on your remote servers:

```bash
tow setup -e dev
```

This creates the following on each server:

```
/app/api-server-1/
├── deploy/       # Timestamped deployment versions
├── upload/       # File upload staging
├── log/          # Application logs
└── conf/         # Configuration files
```

### Step 4: Deploy

Run a full deployment pipeline:

```bash
# Full auto pipeline: build → package → upload → install → restart
tow auto -e dev -m api-server

# Or step by step:
tow deploy -e dev -m api-server  # package → upload → install → restart
```

### Step 5: Verify

```bash
# Check process status
tow status -e dev -m api-server

# Stream logs
tow logs -e dev -m api-server

# List deployment history
tow list deployments -e dev -m api-server
```

### Rollback if Needed

```bash
# Roll back to the previous deployment
tow rollback -e dev -m api-server

# Roll back to a specific version
tow rollback -e dev -m api-server -t 20240626-101530
```

---

## Deployment Flow Overview

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

Rollback is instant — just switch the `current` symlink to any previous deployment directory.

---

## What's Next?

- [Architecture]({{ '/architecture' | relative_url }}) — Understand how Tow works under the hood
- [Configuration]({{ '/configuration' | relative_url }}) — Full reference for `tow.yaml`
- [Commands]({{ '/commands' | relative_url }}) — Complete CLI reference
- [Examples]({{ '/examples' | relative_url }}) — Real-world deployment scenarios
