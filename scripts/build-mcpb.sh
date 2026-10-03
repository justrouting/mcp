#!/bin/sh
# Rebuild justrouting.mcpb from the release binaries.
# Usage: scripts/build-mcpb.sh [version]   (defaults to the latest git tag)
# Requires: mcpb CLI (npm install -g @anthropic-ai/mcpb), curl, tar, unzip
set -eu

DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION="${1:-$(git -C "$DIR" describe --tags --abbrev=0 | sed 's/^v//')}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

BUNDLE="$TMP/bundle"
mkdir -p "$BUNDLE/server" "$TMP/assets"

# Download the per-platform binaries from the GitHub release
for asset in \
  justrouting-mcp_darwin_amd64.tar.gz \
  justrouting-mcp_darwin_arm64.tar.gz \
  justrouting-mcp_linux_amd64.tar.gz \
  justrouting-mcp_linux_arm64.tar.gz \
  justrouting-mcp_windows_amd64.zip; do
  curl -fsSL --retry 5 --retry-all-errors --retry-delay 2 "https://github.com/justrouting/mcp/releases/download/v$VERSION/$asset" -o "$TMP/assets/$asset"
done

# Unix binaries: keep arch-suffixed names for the dispatch wrapper
for asset in "$TMP"/assets/*.tar.gz; do
  tar -xzf "$asset" -C "$BUNDLE/server"
  mv "$BUNDLE/server/justrouting-mcp" \
    "$BUNDLE/server/justrouting-mcp-$(basename "$asset" | sed 's/justrouting-mcp_//; s/\.tar\.gz//')"
done

# Windows binary: the MCPB host app appends .exe to the base command
unzip -qo "$TMP/assets/justrouting-mcp_windows_amd64.zip" -d "$BUNDLE/server"

# Platform/arch dispatch wrapper (entry point for macOS/Linux)
cat > "$BUNDLE/server/justrouting-mcp" <<'EOF'
#!/bin/sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) exec "$DIR/justrouting-mcp-darwin_arm64" "$@" ;;
  Darwin-x86_64) exec "$DIR/justrouting-mcp-darwin_amd64" "$@" ;;
  Linux-aarch64|Linux-arm64) exec "$DIR/justrouting-mcp-linux_arm64" "$@" ;;
  Linux-x86_64) exec "$DIR/justrouting-mcp-linux_amd64" "$@" ;;
  *) echo "JustRouting MCP: unsupported platform $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac
EOF
chmod +x "$BUNDLE/server/justrouting-mcp"

# Icon + manifest
curl -fsSL "https://justrouting.tech/logo-mark.png" -o "$BUNDLE/icon.png"
sed "s/\"version\": \"[^\"]*\"/\"version\": \"$VERSION\"/" "$DIR/mcpb-manifest.json" > "$BUNDLE/manifest.json"

mcpb validate "$BUNDLE/manifest.json"
mcpb pack "$BUNDLE" "$DIR/justrouting.mcpb"

echo "built $DIR/justrouting.mcpb (v$VERSION)"
echo "sha256: $(shasum -a 256 "$DIR/justrouting.mcpb" | awk '{print $1}')"
echo "remember to update fileSha256 in server.json and upload the asset to the v$VERSION release"
