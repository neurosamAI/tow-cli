You have access to the `tow` CLI for deploying applications to remote servers.

## Available Commands

### Deployment
```bash
tow status -e <env> -m <module>                    # Check status
tow status -e <env>                                 # All modules status
tow auto -e <env> -m <module> -y                    # Full deploy pipeline
tow auto -e <env> -m <module> --rolling -y          # Rolling deploy
tow auto -e <env> -m <module> --auto-rollback -y    # Deploy with auto-rollback
tow deploy -e <env> -m <module> -y                  # Package + deploy only
tow build -e <env> -m <module>                      # Build only
tow package -e <env> -m <module>                    # Package only
tow rollback -e <env> -m <module> -y                # Rollback
tow start -e <env> -m <module>                      # Start
tow stop -e <env> -m <module>                       # Stop
tow restart -e <env> -m <module>                    # Restart
```

### Monitoring & Diagnostics
```bash
tow logs -e <env> -m <module> -n 50                 # Last 50 lines
tow logs -e <env> -m <module> -f "ERROR"            # Filter errors
tow logs -e <env> -m <module> --all -F              # Stream all servers
tow logs -e <env> -m <module> -s srv-1,srv-2        # Multiple servers
tow ssh -e <env> -m <module> --all -- "free -h"     # Run command on servers
tow ssh -e <env> -m <module> -- "df -h"             # Disk space
tow doctor -e <env> -m <module>                     # Pre-flight diagnostics
tow diff -e <env> -m <module>                       # What will be deployed
```

### History & Information
```bash
tow list deployments -e <env> -m <module>           # Deployment history
tow list deployments -e <env> -m <module> -o json   # JSON format
tow status -e <env> -m <module> -o json             # Status as JSON
tow list modules                                     # All modules
tow list envs                                        # All environments
```

### Configuration Management
```bash
tow config server list -e <env>                     # List servers
tow config server add -e <env> --name srv --host IP --modules mod1,mod2
tow config server remove -e <env> --name srv
tow config module add my-api --type springboot --port 8080
tow config module remove my-api
tow config assign -e <env> --server srv --modules mod1,mod2
tow config unassign -e <env> --server srv --modules mod1
```

### Maintenance
```bash
tow cleanup -e <env> -m <module> --keep 3           # Clean old deploys
tow threaddump -e <env> -m <module>                 # Java thread dump
tow provision -e <env> -m <module> --jre --tools    # Server setup
tow plugin list                                      # Available plugins
```

## Safety Rules

1. ALWAYS check status before deploying: `tow status -e <env> -m <module>`
2. ALWAYS use `-y` flag when running non-interactively
3. For production: confirm with the user before running any deploy/rollback
4. After deploy: verify with `tow status` and check logs for errors
5. If deploy fails: check logs first, then consider `tow rollback`

## Common Workflows

### Safe Production Deploy
```bash
tow doctor -e prod -m api-server                    # Pre-flight check
tow diff -e prod -m api-server                      # Review changes
tow auto -e prod -m api-server --auto-rollback -y   # Deploy with safety net
tow status -e prod -m api-server                    # Verify
tow logs -e prod -m api-server -f "ERROR" -n 20     # Check for errors
```

### Investigate Production Issue
```bash
tow status -e prod -m api-server -o json            # Quick status
tow logs -e prod -m api-server -f "ERROR" -n 100    # Recent errors
tow ssh -e prod -m api-server -- "free -h"          # Memory check
tow ssh -e prod -m api-server -- "df -h"            # Disk check
tow rollback -e prod -m api-server -y               # Rollback if needed
```

### Multi-Server Monitoring
```bash
tow logs -e prod -m kafka --all -n 10               # All Kafka servers
tow ssh -e prod -m kafka --all -- "df -h"           # Disk on all nodes
tow status -e prod                                   # All modules overview
```
