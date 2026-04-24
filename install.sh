#!/usr/bin/env bash
set -euo pipefail

REPO="yearsyan/llmagent"
BIN="llmagent"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.llmagent/bin}"
VERSION="${VERSION:-latest}"

main() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)

  case "$OS" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
  esac

  case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported arch: $ARCH"; exit 1 ;;
  esac

  TAR="llmagent-${OS}-${ARCH}.tar.gz"
  URL="https://github.com/${REPO}/releases/${VERSION}/download/${TAR}"

  echo "Downloading ${URL}..."
  TMPDIR=$(mktemp -d)
  trap "rm -rf ${TMPDIR}" EXIT

  if command -v curl &>/dev/null; then
    curl -fsSL "$URL" -o "${TMPDIR}/${TAR}"
  elif command -v wget &>/dev/null; then
    wget -q "$URL" -O "${TMPDIR}/${TAR}"
  else
    echo "Error: curl or wget required"
    exit 1
  fi

  tar xzf "${TMPDIR}/${TAR}" -C "$TMPDIR"
  mkdir -p "$INSTALL_DIR"
  mv "${TMPDIR}/${BIN}-${OS}-${ARCH}" "${INSTALL_DIR}/${BIN}"
  chmod +x "${INSTALL_DIR}/${BIN}"

  echo "Installed ${BIN} to ${INSTALL_DIR}/${BIN}"
  echo ""
  echo "Add to your shell profile:"
  echo "  export PATH=\"\${HOME}/.llmagent/bin:\${PATH}\""

  CONFIG="${HOME}/.llmagent/config.yaml"
  if [ ! -f "$CONFIG" ]; then
    echo ""
    echo "No config found. Create ~/.llmagent/config.yaml:"
    echo ""
    echo "models:"
    echo "  claude-official:"
    echo "    backend: claude-code"
    echo "    official: true"
    echo "    description: \"Official Claude Code using your local Claude login\""
    echo ""
    echo "  deepseek-reasoner:"
    echo "    backend: claude-code"
    echo "    description: \"DeepSeek reasoning model, good for complex logic\""
    echo "    base_url: \"https://api.deepseek.com/anthropic\""
    echo "    auth_token: \"sk-xxx\""
    echo "    model: \"deepseek-reasoner\""
    echo "    small_fast_model: \"deepseek-chat\""
    echo ""
    echo "daemon:"
    echo "  socket: \"/tmp/llmagent.sock\""
  fi
}

main
