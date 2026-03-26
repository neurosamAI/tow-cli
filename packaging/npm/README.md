# @neurosamai/tow

> Created by [Murry Jeong (comchangs)](https://github.com/comchangs) &middot; Supported by [neurosam.AI](https://neurosam.ai)

Lightweight, agentless deployment orchestrator for bare-metal servers and VMs.

## Install

```bash
npm install -g @neurosamai/tow
```

This downloads the appropriate binary for your platform (macOS/Linux, amd64/arm64).

## Usage

```bash
tow init                              # Auto-detect project → generate config
tow auto -e prod -m api-server        # Build → package → upload → deploy
tow rollback -e prod -m api-server    # Instant rollback
tow status -e prod -m api-server      # Check status
```

## Documentation

- [Getting Started](https://tow-cli.neurosam.ai/getting-started)
- [Full Documentation](https://tow-cli.neurosam.ai)
- [GitHub](https://github.com/neurosamAI/tow-cli)

## Other Installation Methods

```bash
# Homebrew
brew install neurosamAI/tap/tow

# Bash installer
curl -fsSL https://raw.githubusercontent.com/neurosamAI/tow-cli/main/install.sh | bash

# Go
go install github.com/neurosamAI/tow-cli/cmd/tow@latest
```

## License

MIT — [comchangs](https://github.com/comchangs) / [neurosam.AI](https://neurosam.ai)
