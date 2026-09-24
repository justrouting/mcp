#!/usr/bin/env bash
# Install the JustRouting MCP server from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/justrouting/mcp/main/install.sh | sh
#
# Options (environment variables):
#   JUSTROUTING_MCP_VERSION   Release tag to install (default: latest release).
#   JUSTROUTING_MCP_BASE_URL  Base URL for the downloads (default: GitHub Releases).
#   INSTALL_DIR               Where to install the binary (default: auto).

set -euo pipefail

REPO="justrouting/mcp"
PROJECT="justrouting-mcp"
VERSION="${JUSTROUTING_MCP_VERSION:-latest}"
BASE_URL="${JUSTROUTING_MCP_BASE_URL:-https://github.com/$REPO/releases/$VERSION/download}"

# --- Map OS and architecture to GoReleaser asset naming ---
case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  MINGW*|MSYS*|CYGWIN*)
    echo "Error: this installer does not support Windows." >&2
    echo "Download justrouting-mcp_windows_amd64.zip from" >&2
    echo "  https://github.com/$REPO/releases/latest" >&2
    echo "and put the binary on your PATH." >&2
    exit 1
    ;;
  *)
    echo "Error: unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64)   arch="amd64" ;;
  arm64|aarch64)  arch="arm64" ;;
  *)
    echo "Error: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

ARCHIVE="${PROJECT}_${os}_${arch}.tar.gz"
CHECKSUMS="checksums.txt"

# --- Download archive and checksums ---
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "==> Downloading ${ARCHIVE} (${VERSION})"
curl -fsSL --retry 3 -o "$TMP_DIR/$ARCHIVE" "$BASE_URL/$ARCHIVE"
curl -fsSL --retry 3 -o "$TMP_DIR/$CHECKSUMS" "$BASE_URL/$CHECKSUMS"

# --- Verify the checksum from the release's checksums.txt ---
cd "$TMP_DIR"
grep "$ARCHIVE" "$CHECKSUMS" > checksums.filtered
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c checksums.filtered
else
  shasum -a 256 -c checksums.filtered
fi

# --- Extract and install ---
tar -xzf "$ARCHIVE"
chmod +x "$PROJECT"

if [ -n "${INSTALL_DIR:-}" ]; then
  DEST="$INSTALL_DIR"
elif [ -w /usr/local/bin ]; then
  DEST="/usr/local/bin"
else
  DEST="$HOME/.local/bin"
fi
mkdir -p "$DEST"
install -m 755 "$PROJECT" "$DEST/$PROJECT"

if ! command -v "$PROJECT" >/dev/null 2>&1; then
  echo "Warning: $DEST is not on your PATH." >&2
  echo "  Add it, or run the server with its full path: $DEST/$PROJECT" >&2
fi

echo "==> Installed $PROJECT to $DEST/$PROJECT"
echo
echo "Next, set your API key and add the server to your MCP client:"
echo "  https://github.com/$REPO#configuration"
echo
echo "Uninstall: rm \"$DEST/$PROJECT\""
