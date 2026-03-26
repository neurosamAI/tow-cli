# Tow Deploy — VS Code Extension

> by [neurosam.AI](https://neurosam.ai)

Deploy to bare-metal servers and VMs directly from VS Code — without the complexity of Kubernetes.

## Features

- **Sidebar Panel**: View environments, modules, and deployment history at a glance
- **Command Palette**: All Tow commands available via `Ctrl+Shift+P`
- **Status Bar**: Quick deploy button
- **Terminal Integration**: Commands run in VS Code's integrated terminal

## Requirements

- [Tow CLI](https://github.com/neurosamAI/tow-cli) installed (`brew install neurosamAI/tap/tow`)
- A `tow.yaml` in your workspace root

## Commands

| Command | Description |
|---------|-------------|
| `Tow: Auto Deploy` | Full pipeline (build → package → upload → deploy) |
| `Tow: Deploy Module` | Deploy only (package → upload → install → restart) |
| `Tow: Rollback` | Rollback to previous deployment |
| `Tow: Check Status` | View module status in output panel |
| `Tow: Stream Logs` | Tail remote logs in terminal |
| `Tow: Start Module` | Start a stopped module |
| `Tow: Stop Module` | Stop a running module |
| `Tow: SSH Login` | Open SSH session in terminal |

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `tow.configPath` | `tow.yaml` | Path to configuration file |
| `tow.defaultEnvironment` | | Default environment for commands |
| `tow.showStatusBar` | `true` | Show deploy button in status bar |

## Development

```bash
cd integrations/vscode
npm install
npm run compile
# Press F5 in VS Code to launch Extension Development Host
```

## Publishing

```bash
npm install -g @vscode/vsce
vsce package
vsce publish
```
