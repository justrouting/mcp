go build ./cmd/justrouting-mcp
cp justrouting-mcp ~/.local/bin/justrouting-mcp
codesign --force --sign - ~/.local/bin/justrouting-mcp
