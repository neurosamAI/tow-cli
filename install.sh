#!/bin/bash
# Tow CLI Installer
# by neurosam.AI — https://neurosam.ai
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/neurosamAI/tow-cli/main/install.sh | bash
#
#   With a specific version:
#   curl -fsSL https://raw.githubusercontent.com/neurosamAI/tow-cli/main/install.sh | bash -s -- v0.1.0
#
#   With AI integration (Claude Code + MCP):
#   curl -fsSL https://raw.githubusercontent.com/neurosamAI/tow-cli/main/install.sh | WITH_AI=true bash

set -e

# ─── Config ───

REPO="neurosamAI/tow-cli"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="tow"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ─── Functions ───

info() { echo -e "${BLUE}[info]${NC} $1"; }
success() { echo -e "${GREEN}[✓]${NC} $1"; }
error() { echo -e "${RED}[error]${NC} $1" >&2; exit 1; }

detect_platform() {
    local os arch

    case "$(uname -s)" in
        Darwin) os="darwin" ;;
        Linux)  os="linux" ;;
        *)      error "Unsupported OS: $(uname -s). Tow supports macOS and Linux." ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)             error "Unsupported architecture: $(uname -m). Tow supports amd64 and arm64." ;;
    esac

    echo "${os}-${arch}"
}

get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
        | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')

    if [ -z "$version" ]; then
        error "Failed to fetch latest version. Check your internet connection."
    fi
    echo "$version"
}

download_and_install() {
    local version="$1"
    local platform="$2"
    local url="https://github.com/${REPO}/releases/download/${version}/tow-${platform}"
    local tmp_file

    tmp_file=$(mktemp)

    info "Downloading Tow ${version} for ${platform}..."
    if ! curl -fsSL -o "$tmp_file" "$url"; then
        rm -f "$tmp_file"
        error "Download failed. URL: $url"
    fi

    chmod +x "$tmp_file"

    # Verify it's a valid binary
    if ! "$tmp_file" --version >/dev/null 2>&1; then
        rm -f "$tmp_file"
        error "Downloaded binary is invalid or corrupted."
    fi

    # Install
    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    success "Tow ${version} installed successfully!"
}

setup_ai_integration() {
    # Only if user opted in
    if [ "${WITH_AI:-false}" != "true" ]; then
        echo ""
        info "AI integration available! Run in your project:"
        echo -e "  ${CYAN}tow init --with-ai${NC}    # Generate Claude Code skill + MCP config"
        echo ""
        if command -v claude >/dev/null 2>&1; then
            info "Or add MCP server to Claude Code directly:"
            echo -e "  ${CYAN}claude mcp add tow tow mcp-server${NC}"
        fi
        return
    fi

    # Auto-setup Claude Code skill if .claude directory exists
    if [ -d ".claude" ] || [ -d "$HOME/.claude" ]; then
        local skill_dir
        if [ -d ".claude" ]; then
            skill_dir=".claude/skills"
        else
            skill_dir="$HOME/.claude/skills"
        fi

        if [ ! -f "$skill_dir/tow-deploy.md" ]; then
            info "Setting up Claude Code skill..."
            mkdir -p "$skill_dir"
            curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/integrations/claude-skill/tow-deploy.md" \
                -o "$skill_dir/tow-deploy.md" 2>/dev/null && \
                success "Claude Code skill installed at $skill_dir/tow-deploy.md" || true
        fi
    fi

    if command -v claude >/dev/null 2>&1; then
        info "Adding Tow MCP server to Claude Code..."
        claude mcp add tow tow mcp-server 2>/dev/null && \
            success "MCP server registered with Claude Code" || true
    fi
}

# ─── Main ───

main() {
    echo ""
    echo -e "${BOLD}${CYAN}  ⚓ Tow Installer${NC}"
    echo -e "  ${BLUE}by Murry Jeong (comchangs) — neurosam.AI${NC}"
    echo ""

    # Determine version
    local version="${1:-}"
    if [ -z "$version" ]; then
        info "Fetching latest version..."
        version=$(get_latest_version)
    fi

    # Detect platform
    local platform
    platform=$(detect_platform)

    info "Platform: ${platform}"
    info "Version:  ${version}"
    echo ""

    # Download and install
    download_and_install "$version" "$platform"

    # Verify installation
    local installed_version
    installed_version=$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>&1 | head -1)
    success "Installed: ${installed_version}"

    # AI integration setup
    echo ""
    setup_ai_integration

    # Print next steps
    echo ""
    echo -e "${BOLD}Next steps:${NC}"
    echo -e "  ${CYAN}cd your-project${NC}"
    echo -e "  ${CYAN}tow init${NC}              # Auto-detect and generate config"
    echo -e "  ${CYAN}tow auto -e dev -m app${NC} # Deploy!"
    echo ""
    echo -e "  Docs: ${BLUE}https://tow-cli.neurosam.ai${NC}"
    echo ""
}

main "$@"
