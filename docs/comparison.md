---
layout: default
title: Comparison
---

# Why Tow?

There are many deployment tools out there. Here's how Tow compares — and why we built it.

---

## The Problem

You have a few servers. You need to deploy your app reliably. Your options:

- **Manual SSH** — Error-prone, no rollback, no consistency
- **Ansible** — Powerful but complex. YAML playbooks, inventory files, roles, Galaxy — you need to be an Ansible expert
- **Capistrano** — Great for Ruby, but requires Ruby installed locally. Other languages are second-class citizens
- **Kamal** — Modern and elegant, but requires Docker on every server
- **Kubernetes** — Massive operational overhead for 2-5 servers

Tow fills the gap: **production-grade deployments without the complexity**.

### A practical example

Let's say you need to deploy a Spring Boot API + Kafka + Redis to 3 EC2 instances. Here's what each tool requires:

**Ansible** — ~150 lines across 5+ files:
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

**Kamal** — Requires Docker on every server, plus a container registry:
```bash
# Every server needs: Docker daemon, Docker CLI, login to registry
# Every app needs: Dockerfile, image build, push to registry, pull on server
```

**Tow** — One file, auto-generated:
```bash
tow init          # Generates tow.yaml (detected: springboot + kafka + redis)
vim tow.yaml      # Add server IPs
tow auto -e prod  # Done.
```

That's the difference. Not in features on paper — in **time to first deploy**.

---

## Comparison Table

| | **Tow** | **Capistrano** | **Ansible** | **Kamal** | **Fabric** | **Deployer** | **PM2** |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **Single binary** | ● | | | | | | |
| **No runtime dependency** | ● | Ruby | Python | Ruby + Docker | Python | PHP | Node.js |
| **No Docker required** | ● | ● | ● | | ● | ● | ● |
| **Agentless (SSH only)** | ● | ● | ● | ● | ● | ● | ● |
| **Atomic symlink deploy** | ● | ● | | | | ● | |
| **Auto-detection** | ● | | | | | | |
| **Built-in health checks** | ● | | | ● | | | ● |
| **Multi-language native** | ● | | ● | ● | ● | | |
| **Monorepo support** | ● | | | | | | |
| **Deploy locking** | ● | | | | | | |
| **Hierarchical config** | ● | | ● | | | | |
| **Infrastructure services** | ● (35 plugins) | | ● | ● | | | |
| **AI agent support (MCP)** | ● | | | | | | |
| **Rolling deploy** | ● | | ● | ● | | | |
| **Auto-rollback** | ● | | | ● | | | |
| **Built-in notifications** | ● | ● | | | | | |
| **Learning curve** | Low | Medium | High | Medium | Medium | Medium | Low |
| **GitHub Stars** | New | ~13k | ~63k | ~14k | ~15k | ~11k | ~43k |

---

## Tow vs. Ansible

**Ansible** is the industry standard for configuration management and infrastructure automation. It's incredibly powerful — and incredibly complex.

| Aspect | Tow | Ansible |
|--------|-----|---------|
| **Purpose** | Application deployment | Infrastructure + deployment + config management |
| **Config format** | Single `tow.yaml` | Playbooks + inventory + roles + group_vars + templates |
| **Learning curve** | Minutes | Days to weeks |
| **Setup** | `go install` (single binary) | `pip install ansible` + dependencies |
| **Deployment model** | Symlink-based atomic | Task-based (or ansistrano role) |
| **Rollback** | Built-in, instant | Manual (re-run previous playbook or use ansistrano) |
| **Health checks** | Built-in (HTTP, TCP, log, command) | Manual (write your own tasks) |

**Choose Ansible** if you need full infrastructure management (install packages, manage users, configure firewalls). **Choose Tow** if you just need to deploy applications reliably.

> Tow and Ansible are not mutually exclusive. Use Ansible for server provisioning, Tow for application deployment.

### In practice

A common real-world story: a team adopts Ansible for deployments. Month 1, it works. By month 6, they have 2,000 lines of playbooks that nobody fully understands. A new developer joins and is afraid to touch the deploy scripts. Rollback? Hope you remember which playbook version was running on which server.

With Tow, your entire deployment config is a single readable YAML file. Rollback is `tow rollback`. A new developer can deploy on day one.

---

## Tow vs. Capistrano

**Capistrano** pioneered symlink-based atomic deployments. Tow takes the same proven pattern and makes it language-agnostic.

| Aspect | Tow | Capistrano |
|--------|-----|------------|
| **Language** | Go (single binary) | Ruby (requires Ruby + Bundler) |
| **Target ecosystems** | Java, Node, Python, Go, Rust, Kafka, Redis, ... | Ruby/Rails (others via plugins) |
| **Auto-detection** | Detects project type, framework, build tool | Manual configuration |
| **Health checks** | Built-in | Via plugins only |
| **Monorepo** | Native multi-module support | Manual per-app configuration |
| **Config** | YAML | Ruby DSL |

**Choose Capistrano** if you're a Ruby shop with existing Capistrano recipes. **Choose Tow** for polyglot teams or non-Ruby projects.

### In practice

Your team runs a Spring Boot API, a Node.js frontend, and a Python ML service. With Capistrano, you'd need to install Ruby on every developer's machine, learn the Rake DSL, and cobble together community plugins for Java and Node.js that may or may not be maintained.

With Tow, all three are first-class citizens:

```bash
tow auto -e prod -m api-server       # Spring Boot — knows about gradlew, bootJar, actuator
tow auto -e prod -m web-frontend     # Node.js — knows about npm ci, dist/, health endpoints
tow auto -e prod -m ml-service       # Python — knows about pip, gunicorn, virtualenvs
```

One tool. One config. No Ruby required.

---

## Tow vs. Kamal

**Kamal** (by DHH / 37signals) is the closest philosophical competitor — both target "deploy to VMs without Kubernetes." The key difference: Kamal requires Docker.

| Aspect | Tow | Kamal |
|--------|-----|-------|
| **Deployment unit** | Application artifacts (JAR, binary, dist/) | Docker containers |
| **Docker required** | No | Yes (on every server) |
| **Traffic management** | Application-level | kamal-proxy (built-in reverse proxy) |
| **Zero-downtime** | Symlink switch + health check | Container swap via proxy |
| **Build pipeline** | Local build → package → upload | Local build → push to registry → pull on server |
| **Infrastructure services** | Native handlers (Kafka, Redis) | Docker accessories |
| **Resource overhead** | Minimal (direct process) | Docker daemon + container overhead |

**Choose Kamal** if your team already uses Docker and wants container-based deployments. **Choose Tow** if you prefer deploying directly to the OS without container overhead.

### When Docker Isn't the Right Fit

- **JVM applications**: Already manage their own process lifecycle; Docker adds overhead without benefit
- **Legacy infrastructure**: Servers where installing Docker isn't an option
- **Resource-constrained environments**: Docker daemon + container overhead matters on small VMs
- **GPU workloads**: Docker GPU passthrough adds complexity
- **Regulatory environments**: Some compliance frameworks restrict container usage

### In practice

A startup runs 3 Spring Boot services on t3.medium EC2 instances (2 vCPU, 4GB RAM). The JVM already manages memory via `-Xmx`. Adding Docker means:

- Installing and maintaining Docker daemon on each server (~300MB+ RAM overhead)
- Building Docker images (needs a registry — ECR, Docker Hub, or self-hosted)
- Debugging becomes `docker exec -it` instead of just `ssh`
- Log management through Docker's log driver instead of simple files
- JVM heap tuning now fights with Docker's memory limits

With Tow, the JVM runs directly on the OS. Full RAM is available. Logs are plain files. SSH gives you direct access. No abstraction layers between you and your app.

Kamal is the right choice if you're already containerized. But if Docker isn't already part of your stack, Tow lets you ship without adding that complexity.

---

## Tow vs. Fabric

**Fabric** is a Python SSH task runner — a low-level building block, not a deployment framework.

| Aspect | Tow | Fabric |
|--------|-----|--------|
| **Abstraction level** | Complete deployment pipeline | SSH task runner |
| **Out-of-the-box** | Build → deploy → health check → rollback | Run commands over SSH |
| **Deployment strategy** | Symlink-based atomic (built-in) | Build your own |
| **Rollback** | Built-in (`tow rollback`) | Write your own |
| **Health checks** | Built-in (4 types) | Write your own |

**Choose Fabric** if you need a programmable SSH toolkit for custom automation. **Choose Tow** if you want a complete deployment solution out of the box.

---

## Tow vs. PM2 Deploy

**PM2** is a Node.js process manager with basic deployment capabilities.

| Aspect | Tow | PM2 Deploy |
|--------|-----|------------|
| **Language support** | Java, Node, Python, Go, Rust, ... | Node.js only |
| **Deployment model** | Artifact-based, symlink atomic | Git-based (git pull on server) |
| **Rollback** | Instant symlink switch | Git revert |
| **Process management** | External (system scripts) | Built-in (cluster mode, auto-restart) |
| **Health checks** | HTTP, TCP, log, command | Process monitoring |

**Choose PM2** if you're running a single Node.js app and want built-in process clustering. **Choose Tow** for multi-service, multi-language deployments.

---

## What Makes Tow Unique

### 1. Auto-Detection That Actually Works

```bash
$ cd my-springboot-project
$ tow init
✔ Detected: springboot (gradle, multi-module)
  Modules: api-server, batch-server, admin-server
  Excluded: common-lib, data-core (library modules)
✔ Generated tow.yaml + control scripts
```

No other deployment tool scans your project and generates a complete, working configuration. Tow detects:

- **Languages**: Java, Node.js, Python, Go, Rust
- **Frameworks**: Spring Boot, NestJS, Express, FastAPI, Django, Flask, Next.js
- **Build tools**: Gradle, Maven, npm, yarn, pnpm, poetry, uv, cargo
- **Monorepo structure**: Gradle `settings.gradle`, Maven `pom.xml <modules>`
- **Library vs. deployable modules**: Filters out `-common`, `-core`, `-support` modules

### 2. True Polyglot — Not an Afterthought

Most deployment tools support one ecosystem natively and bolt on others:

- Capistrano: Ruby-first, other languages via community plugins
- Deployer: PHP-first, limited to PHP ecosystem
- PM2: Node.js only
- Kamal: Language-agnostic but requires Docker for everything

Tow has **first-class handlers** for 12 module types, each with sensible defaults for build commands, start/stop scripts, health checks, and artifacts.

### 3. Zero Installation on Servers

- No agent to install or update
- No Docker daemon to maintain
- No runtime to provision
- If you can SSH in, you can deploy

### 4. Born from Production Experience

Tow isn't a toy project. It's the Go rewrite of deployment systems that have been running in production across multiple companies and projects — managing Spring Boot microservices, Kafka clusters, Redis, MongoDB, monitoring stacks, and more.

Every feature in Tow is a battle scar turned into a safeguard:

| Feature | The real incident behind it |
|---------|---------------------------|
| **Deploy locking** | Two developers ran deploy at the same time. Server 1 got version A, server 2 got version B. API calls randomly failed depending on which server handled the request. |
| **Branch policies** | An engineer deployed a feature branch to production on Friday at 6pm. The branch had a migration that broke the database schema. Weekend incident. |
| **Health checks** | Deploy script reported "success" but the app was crash-looping due to a missing env variable. Nobody noticed for 30 minutes until customers reported errors. |
| **Hierarchical config** | Production server 1 (primary) needed 4GB heap, server 2 (secondary) needed 2GB. But they shared all other config. Copy-pasting config files led to drift. |
| **Instant rollback** | Bad deploy went out. Team had to rebuild the previous version from git, re-package, re-upload, re-install. Took 15 minutes. With symlink rollback, it takes 1 second. |
| **Parallel deploy** | Deploying 5 services to 3 servers sequentially took 25 minutes. Engineers context-switched away and forgot to verify the last few. |

These aren't theoretical features. They solve problems that cost real teams real hours.

### 5. Time to First Deploy

The ultimate differentiator is **how fast you go from zero to a working deployment pipeline**:

| Tool | Steps to first deploy |
|------|----------------------|
| **Ansible** | Install Python + Ansible → Learn YAML/Jinja → Write inventory → Write playbook → Write roles → Write templates → Debug → Deploy |
| **Capistrano** | Install Ruby + Bundler → Learn Rake DSL → Write deploy.rb → Write stage configs → Find plugins for your language → Debug → Deploy |
| **Kamal** | Install Ruby → Install Docker (every server) → Setup container registry → Write Dockerfile → Write deploy.yml → Debug → Deploy |
| **Tow** | `go install ...` → `tow init` → Edit 3 lines (server IPs) → `tow auto -e prod` |

Tow is designed so that **your first deploy happens in minutes, not days**.

---

## Migration Guides

### From Shell Scripts

If you're currently deploying with custom shell scripts:

1. Run `tow init` in your project
2. Map your scripts to Tow operations:
   - `build.sh` → `tow auto` (includes build step)
   - `deploy.sh` → `tow deploy`
   - `restart.sh` → `tow restart`
3. Move server IPs from scripts to `tow.yaml`
4. Use `--dry-run` to verify before first real deployment

### From Capistrano

1. Map `deploy.rb` stages to `tow.yaml` environments
2. Map `linked_dirs` to `data_dirs`
3. Map `linked_files` to `config_dir` hierarchy
4. Capistrano's `current` / `releases` pattern maps directly to Tow's symlink model

### From Ansible

1. Map inventory hosts to `tow.yaml` environments/servers
2. Map deployment playbook tasks to Tow's built-in pipeline
3. Keep Ansible for server provisioning; use Tow for application deployment
