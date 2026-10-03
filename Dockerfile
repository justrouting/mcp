# Runtime image for the JustRouting MCP server. Built by GoReleaser
# (see .goreleaser.yaml) which places the static binary in the build
# context; the resulting images are published to ghcr.io/justrouting.
FROM gcr.io/distroless/static-debian12:nonroot
COPY justrouting-mcp /usr/local/bin/justrouting-mcp
ENTRYPOINT ["/usr/local/bin/justrouting-mcp"]
