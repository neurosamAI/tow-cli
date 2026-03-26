package initializer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/neurosamAI/tow-cli/internal/logger"
)

// GenerateScripts creates env.sh + server script for each deployable module
func GenerateScripts(det *DetectedProject) error {
	primary := det.Primary()
	modules := det.DeployableModules
	if len(modules) == 0 {
		modules = []string{det.ProjectName}
	}

	for _, modName := range modules {
		scriptDir := resolveScriptDir(det, modName)

		if err := os.MkdirAll(scriptDir, 0755); err != nil {
			return fmt.Errorf("creating script directory %s: %w", scriptDir, err)
		}

		// Generate env.sh (customizable settings)
		envPath := filepath.Join(scriptDir, "env.sh")
		if fileExists(envPath) {
			logger.Info("  [%s] script/env.sh already exists, skipping", modName)
		} else {
			envContent := generateEnvSh(modName, primary)
			if err := os.WriteFile(envPath, []byte(envContent), 0755); err != nil {
				return fmt.Errorf("writing %s: %w", envPath, err)
			}
			logger.Success("  [%s] Created %s", modName, envPath)
		}

		// Generate server script (common logic, sources env.sh)
		scriptPath := filepath.Join(scriptDir, "server")
		if fileExists(scriptPath) {
			logger.Info("  [%s] script/server already exists, skipping", modName)
		} else {
			script := generateServerScript(modName, primary)
			if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
				return fmt.Errorf("writing %s: %w", scriptPath, err)
			}
			logger.Success("  [%s] Created %s", modName, scriptPath)
		}
	}

	return nil
}

func resolveScriptDir(det *DetectedProject, modName string) string {
	if det.MultiModule {
		return filepath.Join(det.RootDir, modName, "script")
	}
	return filepath.Join(det.RootDir, "script")
}

// --- env.sh: all customizable settings per module type ---

func generateEnvSh(modName string, pt ProjectType) string {
	switch pt.Name {
	case "springboot":
		return generateSpringBootEnv(modName, pt)
	case "java":
		return generateJavaEnv(modName, pt)
	case "node":
		return generateNodeEnv(modName, pt)
	case "python":
		return generatePythonEnv(modName, pt)
	case "go":
		return generateGoEnv(modName)
	case "rust":
		return generateRustEnv(modName)
	default:
		return generateGenericEnv(modName)
	}
}

func generateSpringBootEnv(modName string, pt ProjectType) string {
	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Environment configuration for: %s
# Type: Spring Boot (%s)
#
# Edit this file to customize your module's runtime settings.
# This file is sourced by the server script — no need to touch server itself.
# =============================================================================

# --- Application ---
MODULE_NAME="%s"
APP_PORT=8080

# --- Start command ---
# How to launch the application. Runs inside SERVER_HOME (the deployment root).
START_CMD="java ${JAVA_OPTS} -jar lib/%s.jar --spring.config.additional-location=file:${SERVER_HOME}/conf/"

# --- JVM Options ---
# Customize heap, GC, and JVM flags here.
JAVA_OPTS="${JAVA_OPTS:--Xms256m -Xmx512m}"
JAVA_OPTS="${JAVA_OPTS} -XX:+UseG1GC"
JAVA_OPTS="${JAVA_OPTS} -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=${LOG_DIR}"
# JAVA_OPTS="${JAVA_OPTS} -Dspring.profiles.active=prod"
# JAVA_OPTS="${JAVA_OPTS} -Dcom.sun.management.jmxremote.port=19080"

# --- Startup verification ---
# How to confirm the app started successfully.
# Options:
#   "port"         — TCP port check (default, requires APP_PORT)
#   "pid"          — just verify process is alive after STARTUP_TIMEOUT
#   "log:PATTERN"  — grep log for regex pattern (e.g., "log:Started .* in .* seconds")
#   "http:URL"     — HTTP health check (e.g., "http:http://localhost:8080/actuator/health")
#   "cmd:COMMAND"  — run custom command (exit 0 = healthy)
#   "none"         — skip verification entirely
STARTUP_CHECK="port"
STARTUP_TIMEOUT=120      # seconds to wait before giving up
STARTUP_INTERVAL=2       # seconds between checks

# --- Shutdown ---
STOP_TIMEOUT=30          # seconds to wait for graceful shutdown before SIGKILL
`, modName, pt.BuildTool, modName, modName)
}

func generateJavaEnv(modName string, pt ProjectType) string {
	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Environment configuration for: %s
# Type: Java (%s)
# =============================================================================

MODULE_NAME="%s"
APP_PORT=8080

START_CMD="java ${JAVA_OPTS} -jar lib/%s.jar"

JAVA_OPTS="${JAVA_OPTS:--Xms256m -Xmx512m}"
JAVA_OPTS="${JAVA_OPTS} -XX:+UseG1GC"
JAVA_OPTS="${JAVA_OPTS} -XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=${LOG_DIR}"

STARTUP_CHECK="port"
STARTUP_TIMEOUT=120
STARTUP_INTERVAL=2

STOP_TIMEOUT=30
`, modName, pt.BuildTool, modName, modName)
}

func generateNodeEnv(modName string, pt ProjectType) string {
	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Environment configuration for: %s
# Type: Node.js (%s)
# =============================================================================

MODULE_NAME="%s"
APP_PORT=3000

# Start command — adjust entry point as needed
START_CMD="node dist/main.js"

# Node.js environment
export NODE_ENV=production
# export NODE_OPTIONS="--max-old-space-size=512"

STARTUP_CHECK="port"
STARTUP_TIMEOUT=30
STARTUP_INTERVAL=1

STOP_TIMEOUT=15
`, modName, pt.BuildTool, modName)
}

func generatePythonEnv(modName string, pt ProjectType) string {
	startCmd := "python -m app"
	if pt.BuildTool == "poetry" {
		startCmd = "poetry run python -m app"
	} else if pt.BuildTool == "uv" {
		startCmd = "uv run python -m app"
	}

	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Environment configuration for: %s
# Type: Python (%s)
# =============================================================================

MODULE_NAME="%s"
APP_PORT=8000

# Start command — adjust for your framework
# FastAPI:  "uvicorn app.main:app --host 0.0.0.0 --port ${APP_PORT}"
# Django:   "gunicorn project.wsgi:application --bind 0.0.0.0:${APP_PORT}"
# Flask:    "gunicorn app:app --bind 0.0.0.0:${APP_PORT}"
START_CMD="%s"

# Python environment
# export PYTHONPATH="${SERVER_HOME}"
# export VIRTUAL_ENV="${SERVER_HOME}/venv"

STARTUP_CHECK="port"
STARTUP_TIMEOUT=30
STARTUP_INTERVAL=1

STOP_TIMEOUT=15
`, modName, pt.BuildTool, modName, startCmd)
}

func generateGoEnv(modName string) string {
	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Environment configuration for: %s
# Type: Go
# =============================================================================

MODULE_NAME="%s"
APP_PORT=8080

START_CMD="bin/%s"

# Go runtime options
# export GOMAXPROCS=4
# export GOGC=100

STARTUP_CHECK="port"
STARTUP_TIMEOUT=15
STARTUP_INTERVAL=1

STOP_TIMEOUT=15
`, modName, modName, modName)
}

func generateRustEnv(modName string) string {
	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Environment configuration for: %s
# Type: Rust
# =============================================================================

MODULE_NAME="%s"
APP_PORT=8080

START_CMD="bin/%s"

# Rust runtime options
# export RUST_LOG=info

STARTUP_CHECK="port"
STARTUP_TIMEOUT=15
STARTUP_INTERVAL=1

STOP_TIMEOUT=15
`, modName, modName, modName)
}

func generateGenericEnv(modName string) string {
	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Environment configuration for: %s
# Type: Generic
#
# TODO: Set START_CMD and APP_PORT for your service.
# =============================================================================

MODULE_NAME="%s"
APP_PORT=8080

# TODO: Replace with your start command
START_CMD=""

STARTUP_CHECK="port"
STARTUP_TIMEOUT=30
STARTUP_INTERVAL=2

STOP_TIMEOUT=15
`, modName, modName)
}

// --- server script: universal, sources env.sh ---

func generateServerScript(modName string, pt ProjectType) string {
	extraCmds := ""
	if pt.Name == "springboot" || pt.Name == "java" {
		extraCmds = `
threaddump() {
    if ! is_alive; then
        echo "${MODULE_NAME} is not running"
        return 1
    fi
    local pid=$(cat "${PID_FILE}")
    echo "Generating thread dump for ${MODULE_NAME} (PID: ${pid})..."
    kill -3 ${pid}
    echo "Thread dump written to ${LOG_FILE}"
}
`
	}

	extraCase := "    *)"
	if pt.Name == "springboot" || pt.Name == "java" {
		extraCase = `    threaddump) threaddump ;;
    *)`
	}

	usage := "start|stop|restart|status"
	if pt.Name == "springboot" || pt.Name == "java" {
		usage = "start|stop|restart|status|threaddump"
	}

	return fmt.Sprintf(`#!/bin/bash
# =============================================================================
# Server control script (auto-generated by tow init)
# Module: %s
#
# All customizable settings are in env.sh — edit that file, not this one.
# =============================================================================

set -e

export LANG="en_US.UTF-8"
export LC_ALL="en_US.UTF-8"

SERVER_HOME=$(cd "$(dirname "$0")/.." && pwd)
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
LOG_DIR="${SERVER_HOME}/log"
LOG_FILE="${LOG_DIR}/std.log"
PID_FILE="${LOG_DIR}/pid"

# Load module configuration
if [ -f "${SCRIPT_DIR}/env.sh" ]; then
    source "${SCRIPT_DIR}/env.sh"
else
    echo "ERROR: env.sh not found in ${SCRIPT_DIR}"
    exit 1
fi

# --- Core functions ---

start() {
    if is_alive; then
        echo "${MODULE_NAME} is already running (PID: $(cat ${PID_FILE}))"
        return 1
    fi

    if [ -z "${START_CMD}" ]; then
        echo "ERROR: START_CMD is not set. Edit env.sh to configure."
        return 1
    fi

    # Pre-flight: validate that the main executable exists
    local first_cmd=$(echo "${START_CMD}" | awk '{print $1}')
    if [ "${first_cmd}" != "nohup" ] && ! command -v "${first_cmd}" >/dev/null 2>&1; then
        if [ ! -x "${first_cmd}" ] && [ ! -x "${SERVER_HOME}/${first_cmd}" ]; then
            echo "ERROR: Command not found: ${first_cmd}"
            echo "  Ensure it is installed or the path is correct in env.sh"
            return 1
        fi
    fi

    mkdir -p "${LOG_DIR}"
    echo "Starting ${MODULE_NAME}..."

    cd "${SERVER_HOME}"
    nohup ${START_CMD} >> "${LOG_FILE}" 2>&1 &

    local pid=$!
    # Atomic PID file write (write to temp, then move)
    echo ${pid} > "${PID_FILE}.tmp" && mv "${PID_FILE}.tmp" "${PID_FILE}"

    # Verify process is still alive after brief delay
    sleep 0.5
    if ! kill -0 ${pid} 2>/dev/null; then
        echo "ERROR: ${MODULE_NAME} process died immediately after start."
        echo "  Check ${LOG_FILE} for details."
        rm -f "${PID_FILE}"
        return 1
    fi

    wait_for_startup ${pid}
}

stop() {
    if ! is_alive; then
        echo "${MODULE_NAME} is not running"
        return 0
    fi

    local pid=$(cat "${PID_FILE}")
    echo "Stopping ${MODULE_NAME} (PID: ${pid})..."

    kill -15 ${pid} 2>/dev/null

    local timeout=${STOP_TIMEOUT:-30}
    local elapsed=0
    while [ ${elapsed} -lt ${timeout} ]; do
        if ! kill -0 ${pid} 2>/dev/null; then
            echo "${MODULE_NAME} stopped"
            rm -f "${PID_FILE}"
            return 0
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done

    echo "Force killing ${MODULE_NAME}..."
    kill -9 ${pid} 2>/dev/null
    rm -f "${PID_FILE}"
    echo "${MODULE_NAME} killed"
}

restart() {
    stop
    sleep 2
    start
}

status() {
    if is_alive; then
        local pid=$(cat "${PID_FILE}")
        local uptime=$(ps -o etime= -p ${pid} 2>/dev/null | tr -d ' ')
        local mem=$(ps -o rss= -p ${pid} 2>/dev/null | tr -d ' ')
        echo "${MODULE_NAME} is RUNNING (PID: ${pid}, uptime: ${uptime}, mem: ${mem}KB)"
    else
        echo "${MODULE_NAME} is STOPPED"
    fi
}
%s
# --- Helpers ---

is_alive() {
    [ -f "${PID_FILE}" ] && kill -0 $(cat "${PID_FILE}") 2>/dev/null
}

wait_for_startup() {
    local pid=$1
    local check="${STARTUP_CHECK:-port}"
    local timeout="${STARTUP_TIMEOUT:-60}"
    local interval="${STARTUP_INTERVAL:-2}"

    case "${check}" in
        port)
            # TCP port check — no application code changes needed
            local elapsed=0
            while [ ${elapsed} -lt ${timeout} ]; do
                if ! kill -0 ${pid} 2>/dev/null; then
                    echo "ERROR: ${MODULE_NAME} process died during startup"
                    return 1
                fi
                if command -v nc >/dev/null 2>&1; then
                    if nc -z localhost ${APP_PORT} 2>/dev/null; then
                        echo "${MODULE_NAME} started successfully (PID: ${pid}, port: ${APP_PORT})"
                        return 0
                    fi
                elif command -v bash >/dev/null 2>&1; then
                    if (echo >/dev/tcp/localhost/${APP_PORT}) 2>/dev/null; then
                        echo "${MODULE_NAME} started successfully (PID: ${pid}, port: ${APP_PORT})"
                        return 0
                    fi
                fi
                sleep ${interval}
                elapsed=$((elapsed + interval))
                printf "."
            done
            echo ""
            echo "WARNING: ${MODULE_NAME} started but port ${APP_PORT} not responding after ${timeout}s (PID: ${pid})"
            ;;
        log:*)
            # Log pattern check — e.g., STARTUP_CHECK="log:Started .* in .* seconds"
            local pattern="${check#log:}"
            local elapsed=0
            while [ ${elapsed} -lt ${timeout} ]; do
                if ! kill -0 ${pid} 2>/dev/null; then
                    echo "ERROR: ${MODULE_NAME} process died during startup"
                    return 1
                fi
                if tail -n 100 "${LOG_FILE}" 2>/dev/null | grep -qE "${pattern}"; then
                    echo "${MODULE_NAME} started successfully (PID: ${pid})"
                    return 0
                fi
                sleep ${interval}
                elapsed=$((elapsed + interval))
                printf "."
            done
            echo ""
            echo "WARNING: ${MODULE_NAME} started but pattern not found after ${timeout}s (PID: ${pid})"
            ;;
        pid)
            # Process-alive check — for workers, batch jobs, or services without a port
            local elapsed=0
            local check_after=5  # wait a few seconds then verify still alive
            sleep ${check_after}
            if kill -0 ${pid} 2>/dev/null; then
                echo "${MODULE_NAME} started and running (PID: ${pid})"
            else
                echo "ERROR: ${MODULE_NAME} process died shortly after startup"
                return 1
            fi
            ;;
        http:*)
            # HTTP health check — e.g., STARTUP_CHECK="http:http://localhost:8080/health"
            local url="${check#http:}"
            local elapsed=0
            while [ ${elapsed} -lt ${timeout} ]; do
                if ! kill -0 ${pid} 2>/dev/null; then
                    echo "ERROR: ${MODULE_NAME} process died during startup"
                    return 1
                fi
                if curl -sf "${url}" >/dev/null 2>&1; then
                    echo "${MODULE_NAME} started and healthy (PID: ${pid})"
                    return 0
                fi
                sleep ${interval}
                elapsed=$((elapsed + interval))
                printf "."
            done
            echo ""
            echo "WARNING: ${MODULE_NAME} started but health check failed after ${timeout}s (PID: ${pid})"
            ;;
        cmd:*)
            # Custom command check — e.g., STARTUP_CHECK="cmd:redis-cli ping"
            local custom_cmd="${check#cmd:}"
            local elapsed=0
            while [ ${elapsed} -lt ${timeout} ]; do
                if ! kill -0 ${pid} 2>/dev/null; then
                    echo "ERROR: ${MODULE_NAME} process died during startup"
                    return 1
                fi
                if eval "${custom_cmd}" >/dev/null 2>&1; then
                    echo "${MODULE_NAME} started and healthy (PID: ${pid})"
                    return 0
                fi
                sleep ${interval}
                elapsed=$((elapsed + interval))
                printf "."
            done
            echo ""
            echo "WARNING: ${MODULE_NAME} started but health check command failed after ${timeout}s (PID: ${pid})"
            ;;
        none)
            echo "${MODULE_NAME} started (PID: ${pid})"
            ;;
        *)
            echo "${MODULE_NAME} started (PID: ${pid})"
            ;;
    esac
}

# --- Dispatch ---

case "$1" in
    start)   start ;;
    stop)    stop ;;
    restart) restart ;;
    status)  status ;;
    %s
        echo "Usage: $0 {%s}"
        exit 1
        ;;
esac
`, modName, extraCmds, extraCase, usage)
}
