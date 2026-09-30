#!/bin/sh
set -e

# RevKeen CLI installer
# Usage: curl -fsSL https://cli.revkeen.com/install.sh | sh

REPO="revkeen/cli"
INSTALL_DIR="${REVKEEN_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="revkeen"

# Detect OS
detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux)  echo "linux" ;;
    *)
      echo "Unsupported OS: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

# Detect architecture
detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "Unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

# Get latest version from GitHub API
get_latest_version() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
      grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" |
      grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
  else
    echo "Neither curl nor wget found. Please install one of them." >&2
    exit 1
  fi
}

# Download file
download() {
  local url="$1"
  local dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$dest" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  fi
}

main() {
  local os arch version archive_name url tmpdir

  os="$(detect_os)"
  arch="$(detect_arch)"

  echo "Detecting system: ${os}/${arch}"

  if [ -n "${REVKEEN_VERSION:-}" ]; then
    version="$REVKEEN_VERSION"
  else
    echo "Fetching latest version..."
    version="$(get_latest_version)"
  fi

  if [ -z "$version" ]; then
    echo "Failed to determine version. Set REVKEEN_VERSION manually." >&2
    exit 1
  fi

  echo "Installing revkeen v${version}..."

  archive_name="revkeen_${os}_${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/download/v${version}/${archive_name}"

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading ${url}..."
  download "$url" "${tmpdir}/${archive_name}"

  echo "Extracting..."
  tar -xzf "${tmpdir}/${archive_name}" -C "$tmpdir"

  # Install binary
  if [ -w "$INSTALL_DIR" ]; then
    mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "${tmpdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
  fi

  chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

  echo ""
  echo "revkeen v${version} installed to ${INSTALL_DIR}/${BINARY_NAME}"
  echo ""
  echo "Run 'revkeen --help' to get started."
}

main
