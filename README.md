# JustRouting MCP

MCP server for [JustRouting](https://justrouting.tech/), a Southeast Asia-focused routing API.

Use JustRouting's road-routing capabilities from MCP-compatible AI assistants such as Claude and Cursor.

## Features

* Driving route calculation
* Driving distance
* Estimated travel duration
* Southeast Asia-focused road coverage
* MCP stdio transport
* Built on the official JustRouting Go client

## Requirements

* Go 1.25+
* A JustRouting API key

## Installation

```bash
go install github.com/justrouting/mcp/cmd/justrouting-mcp@latest
```

Make sure `justrouting-mcp` is available in your `PATH`.

## Configuration

Set your JustRouting API key:

```bash
export JUSTROUTING_API_KEY="YOUR-API-KEY"
```

## Tool

### `route`

Calculate a driving route between two locations.

Input:

```json
{
  "origin": "103.8198,1.3521",
  "destination": "103.9915,1.3644"
}
```

Coordinates must use:

```text
longitude,latitude
```

Output:

```json
{
  "distance_meters": 18500,
  "duration_seconds": 1500
}
```

`distance_meters` is the driving distance in meters.

`duration_seconds` is the estimated driving duration in seconds.

## Claude

Configure the MCP server:

```json
{
  "mcpServers": {
    "justrouting": {
      "command": "justrouting-mcp",
      "env": {
        "JUSTROUTING_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Cursor

Add the following MCP server configuration:

```json
{
  "mcpServers": {
    "justrouting": {
      "command": "justrouting-mcp",
      "env": {
        "JUSTROUTING_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Development

Clone the repository:

```bash
git clone https://github.com/justrouting/mcp.git
cd mcp
```

Install dependencies:

```bash
go mod download
```

Run tests:

```bash
go test ./...
```

Build:

```bash
go build -o justrouting-mcp ./cmd/justrouting-mcp
```

Run:

```bash
JUSTROUTING_API_KEY="YOUR-API-KEY" ./justrouting-mcp
```

The server communicates with MCP clients through stdin/stdout.

## Architecture

```text
┌──────────────────────┐
│   MCP Client         │
│ Claude / Cursor / AI │
└──────────┬───────────┘
           │
           │ MCP / stdio
           ▼
┌──────────────────────┐
│  justrouting-mcp     │
│                      │
│  route tool          │
└──────────┬───────────┘
           │
           │ Go Client
           ▼
┌──────────────────────┐
│ api.justrouting.tech │
└──────────────────────┘
```

The MCP server is intentionally thin.

Routing logic, API authentication, HTTP transport, retries, and API error handling are provided by the official JustRouting Go client.

## Roadmap

* [ ] Route geometry
* [ ] Alternative routes
* [ ] Waypoints
* [ ] Distance matrix
* [ ] Route optimization
* [ ] Streamable HTTP
* [ ] Remote MCP deployment

## License

[MIT](./LICENSE)
