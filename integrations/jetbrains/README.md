# Tow Deploy — JetBrains Plugin

> by [neurosam.AI](https://neurosam.ai)

JetBrains IDE plugin for managing deployments with Tow.
Supports IntelliJ IDEA, WebStorm, PyCharm, GoLand, and all JetBrains IDEs.

## Features

- **Tool Window**: Sidebar with environments, modules, and deployment history
- **Run Configuration**: "Tow Deploy" run configuration type
- **Gutter Icons**: Deploy buttons next to `tow.yaml` module definitions
- **Status Bar Widget**: Current deployment status
- **Terminal Integration**: All commands run in the built-in terminal

## Planned Actions

| Action | Shortcut | Description |
|--------|----------|-------------|
| Deploy | `Ctrl+Shift+D` | Deploy selected module |
| Auto Deploy | `Ctrl+Shift+A` | Full pipeline (build → deploy) |
| Rollback | `Ctrl+Shift+R` | Rollback to previous |
| Status | `Ctrl+Shift+S` | Check module status |
| Logs | `Ctrl+Shift+L` | Stream remote logs |

## Build

```bash
cd integrations/jetbrains
./gradlew buildPlugin
```

## Development Status

This plugin is in planning phase. The core `tow` CLI provides all functionality;
this plugin adds IDE convenience.

Contributions welcome! See [CONTRIBUTING.md](../../CONTRIBUTING.md).
