---
title: Contributing
---

# Contributing

Tow is open source under the MIT License. We welcome contributions of all kinds — code, documentation, bug reports, and feature requests.

## Getting Started

### 1. Fork and Clone

```bash
git clone https://github.com/your-username/tow-cli.git
cd tow-cli
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Build

```bash
make build       # Build binary to ./bin/tow
make install     # Install to $GOPATH/bin
```

### 4. Test

```bash
make test        # Run all tests
make lint        # Run golangci-lint
```

## Project Structure

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

## Development Guidelines

### Code Style

- Follow standard Go conventions (`gofmt`, `golangci-lint`)
- Keep packages focused — each package has a single responsibility
- Prefer explicit over clever code
- Error messages should be actionable: "SSH key not found at ~/.ssh/deploy.pem" over "key error"

### Adding a New Module Type

1. Add the handler in `internal/module/handler.go`:

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

2. Add detection logic in `internal/initializer/initializer.go`
3. Add tests in `internal/initializer/initializer_test.go`
4. Update documentation

### Adding a New Command

1. Add the Cobra command in `cmd/tow/main.go`
2. Implement the logic in the appropriate `internal/` package
3. Add tests
4. Update the [Commands](/docs/commands/) documentation

### Writing Tests

- Config tests go in `internal/config/config_test.go`
- Initializer tests go in `internal/initializer/initializer_test.go`
- Use table-driven tests where appropriate
- Mock SSH connections for deploy tests

## Contribution Types

### Bug Reports

Open an issue with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Tow version (`tow --version`)
- OS and Go version

### Feature Requests

Open an issue describing:
- The use case (what are you trying to accomplish?)
- Proposed solution
- Alternatives you've considered

### Pull Requests

1. Create a feature branch from `main`
2. Make your changes with tests
3. Ensure `make test` and `make lint` pass
4. Write a clear PR description
5. Reference any related issues

## Roadmap

We're building Tow into the go-to deployment tool for teams on bare metal and VMs. Here are areas where contributions are especially welcome:

- **Rolling deployment strategy** — Deploy to servers one at a time with health check gates
- **Blue-green deployment** — Switch traffic between two sets of servers
- **Plugin system** — Allow community-built module handlers
- **Web dashboard** — Real-time deployment status and history
- **Notification integrations** — Slack, Discord, Teams, PagerDuty
- **Deployment metrics** — Track deployment frequency, duration, and failure rate
- **Multi-region support** — Coordinate deployments across regions
- **Config encryption** — Encrypt sensitive values in `tow.yaml`

## Community

- **GitHub Issues**: [github.com/neurosamAI/tow-cli/issues](https://github.com/neurosamAI/tow-cli/issues)
- **Discussions**: [github.com/neurosamAI/tow-cli/discussions](https://github.com/neurosamAI/tow-cli/discussions)

## License

Tow is released under the [MIT License](https://github.com/neurosamAI/tow-cli/blob/main/LICENSE).
