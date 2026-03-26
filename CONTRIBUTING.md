# Contributing to Tow

First off, thank you for considering contributing to Tow! It's people like you that make Tow such a great tool.

> **Tow** is created by [Murry Jeong (comchangs)](https://github.com/comchangs) and supported by [neurosam.AI](https://neurosam.ai).

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating a bug report, please check if the issue already exists. When creating a report, include as many details as possible:

- **Tow version** (`tow --version`)
- **OS and architecture** (e.g., macOS arm64, Ubuntu 22.04 amd64)
- **Go version** (if building from source)
- **Steps to reproduce**
- **Expected behavior**
- **Actual behavior**
- **Relevant logs** (run with `-v` for verbose output)

### Suggesting Features

Feature requests are welcome! Please open an issue and describe:

- The use case — what problem are you trying to solve?
- Proposed solution
- Alternatives you've considered

### Pull Requests

1. Fork the repo and create your branch from `main`
2. If you've added code, add tests
3. Ensure `make test` and `make lint` pass
4. Write a clear PR description referencing any related issues
5. Sign your commits (see below)

## Development Setup

```bash
# Clone your fork
git clone https://github.com/your-username/tow-cli.git
cd tow-cli

# Install dependencies
go mod download

# Build
make build

# Run tests
make test

# Run linter
make lint
```

## Project Structure

```
tow-cli/
├── cmd/tow/            # CLI entry point (Cobra commands)
├── internal/
│   ├── config/         # Configuration loading and validation
│   ├── deploy/         # Deployment operations
│   ├── initializer/    # Project auto-detection
│   ├── logger/         # Colored logging
│   ├── module/         # Module type handlers
│   ├── pipeline/       # Deployment pipelines
│   └── ssh/            # SSH connection management
├── docs/               # GitHub Pages documentation
├── examples/           # Example configurations
└── Makefile            # Build automation
```

## Coding Guidelines

- Follow standard Go conventions (`gofmt`, `golangci-lint`)
- Keep packages focused with single responsibility
- Write error messages that are actionable
- Add tests for new functionality
- Keep commits atomic and well-described

## Commit Messages

Use clear, descriptive commit messages:

```
feat: add rolling deployment strategy
fix: resolve SSH connection timeout on slow networks
docs: update configuration reference for health checks
test: add integration tests for branch policy
```

Prefixes: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `ci`

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

---

<p align="center">
  <sub>Created by <a href="https://github.com/comchangs">comchangs</a> &middot; Supported by <a href="https://neurosam.ai">neurosam.AI</a></sub>
</p>
