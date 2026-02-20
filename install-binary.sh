#!/usr/bin/env bash
set -euo pipefail

PROJECT_NAME="helm-values-checker"
GITHUB_REPO="chrishham/helm-values-checker"
PLUGIN_DIR="$(cd "$(dirname "$0")" && pwd)"

# If a pre-built binary already exists (local dev), skip download
if [ -x "${PLUGIN_DIR}/bin/${PROJECT_NAME}" ]; then
  echo "${PROJECT_NAME} binary already exists, skipping download"
  exit 0
fi

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64)   ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)               echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest version or use HELM_VALUES_CHECKER_VERSION if set
if [ -n "${HELM_VALUES_CHECKER_VERSION:-}" ]; then
  VERSION="$HELM_VALUES_CHECKER_VERSION"
else
  VERSION=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  if [ -z "$VERSION" ]; then
    echo "Error: Could not determine latest version"
    exit 1
  fi
fi

BINARY="${PROJECT_NAME}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY}"

echo "Downloading ${PROJECT_NAME} ${VERSION} for ${OS}/${ARCH}..."

mkdir -p "${PLUGIN_DIR}/bin"

curl -sSL "${DOWNLOAD_URL}" | tar xz -C "${PLUGIN_DIR}/bin" "${PROJECT_NAME}"
chmod +x "${PLUGIN_DIR}/bin/${PROJECT_NAME}"

echo "${PROJECT_NAME} ${VERSION} installed successfully"
