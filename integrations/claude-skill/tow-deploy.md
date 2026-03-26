# Tow Deploy — Claude Code Agent Skill
# by neurosam.AI — https://neurosam.ai
#
# Place this file in your project's .claude/skills/ directory
# or reference it in CLAUDE.md to enable deployment via Claude Code.

You have access to the `tow` CLI for deploying applications to remote servers.

## Available Commands

### Check Status
```bash
tow status -e <environment> -m <module>
tow status -e prod -m api-server -o json  # machine-readable
```

### Deploy
```bash
# Full pipeline: build → package → upload → install → restart
tow auto -e <environment> -m <module> -y

# With rolling deployment (one server at a time)
tow auto -e <environment> -m <module> --rolling -y

# With auto-rollback on failure
tow auto -e <environment> -m <module> --auto-rollback -y
```

### Rollback
```bash
tow rollback -e <environment> -m <module> -y
tow rollback -e <environment> -m <module> -t <timestamp> -y  # specific version
```

### Monitor
```bash
tow logs -e <environment> -m <module> -n 50           # last 50 lines
tow logs -e <environment> -m <module> -f "ERROR" -n 100  # filter errors
tow list deployments -e <environment> -m <module> -o json
```

### Manage
```bash
tow start -e <environment> -m <module>
tow stop -e <environment> -m <module>
tow restart -e <environment> -m <module>
tow cleanup -e <environment> -m <module> --keep 3
```

## Safety Rules

1. **ALWAYS check status before deploying**: `tow status -e <env> -m <module>`
2. **ALWAYS use `-y` flag** when running non-interactively
3. **For production**: Confirm with the user before running any deploy/rollback
4. **After deploy**: Verify with `tow status` and check logs for errors
5. **If deploy fails**: Check logs first, then consider `tow rollback`

## Common Workflows

### Safe Production Deploy
```bash
# 1. Check current status
tow status -e prod -m api-server

# 2. Deploy with auto-rollback safety net
tow auto -e prod -m api-server --auto-rollback -y

# 3. Verify
tow status -e prod -m api-server
tow logs -e prod -m api-server -f "ERROR" -n 20
```

### Investigate Production Issue
```bash
# 1. Check status
tow status -e prod -m api-server -o json

# 2. Check recent logs
tow logs -e prod -m api-server -f "ERROR" -n 100

# 3. If needed, rollback
tow rollback -e prod -m api-server -y

# 4. Verify recovery
tow status -e prod -m api-server
```
