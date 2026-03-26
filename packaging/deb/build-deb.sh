#!/bin/bash
# Build .deb package for Tow
# by neurosam.AI — https://neurosam.ai
#
# Usage: ./build-deb.sh <version> <arch>
# Example: ./build-deb.sh 0.1.0 amd64

set -e

VERSION="${1:-0.1.0}"
ARCH="${2:-amd64}"
PKG_NAME="tow"
PKG_DIR="${PKG_NAME}_${VERSION}_${ARCH}"

echo "Building ${PKG_DIR}.deb..."

# Create package structure
mkdir -p "${PKG_DIR}/DEBIAN"
mkdir -p "${PKG_DIR}/usr/local/bin"
mkdir -p "${PKG_DIR}/usr/share/doc/tow"

# Copy binary
cp "../../dist/tow-linux-${ARCH}" "${PKG_DIR}/usr/local/bin/tow"
chmod 755 "${PKG_DIR}/usr/local/bin/tow"

# Copy docs
cp "../../LICENSE" "${PKG_DIR}/usr/share/doc/tow/"
cp "../../README.md" "${PKG_DIR}/usr/share/doc/tow/"

# Create control file
cat > "${PKG_DIR}/DEBIAN/control" << EOF
Package: tow
Version: ${VERSION}
Section: devel
Priority: optional
Architecture: ${ARCH}
Maintainer: Murry Jeong / neurosam.AI <oss@neurosam.ai>
Homepage: https://tow-cli.neurosam.ai
Description: Lightweight, agentless deployment orchestrator
 Tow deploys applications to bare-metal servers or cloud VMs via SSH.
 No agents, no containers, no Kubernetes required.
 .
 Features: auto-detection, symlink-based atomic deployments,
 instant rollback, health checks, parallel execution.
 .
 by neurosam.AI — https://neurosam.ai
EOF

# Build .deb
dpkg-deb --build "${PKG_DIR}"

echo "✓ Built: ${PKG_DIR}.deb"

# Cleanup
rm -rf "${PKG_DIR}"
