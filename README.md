# JustRouting MCP

MCP server for [JustRouting](https://justrouting.tech/), a Southeast Asia-focused routing API.

Use JustRouting's road-routing capabilities from MCP-compatible AI assistants such as Claude and Cursor.

## Features

* Driving route calculation
* Motorcycle routing profile
* Driving distance
* Estimated travel duration
* Address and place search (geocoding)
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

## Tools

### `route`

Calculate a route between two locations, using the driving profile by default or the motorcycle profile when requested.

Input:

```json
{
  "origin": "103.8198,1.3521",
  "destination": "103.9915,1.3644",
  "profile": "motorcycle"
}
```

`profile` is optional. Set it to `"motorcycle"` for a motorcycle route; omit it (or use `"car"`) for the default driving route. When the user mentions a motorcycle or motorbike, the assistant sets `profile` to `"motorcycle"`.

Coordinates must use:

```text
longitude,latitude
```

If the user asks about places by name or address instead of coordinates, call `geocode` first (see below) and pass its `coordinates` values here.

Output:

```json
{
  "distance_meters": 18500,
  "duration_seconds": 1500
}
```

`distance_meters` is the driving distance in meters.

`duration_seconds` is the estimated driving duration in seconds.

### `geocode`

Search for places and convert an address or place name into coordinates. Use this before `route` when the user refers to places by name.

Input:

```json
{
  "text": "marina bay singapore",
  "limit": 3
}
```

`limit` is optional and defaults to 1 (best match only); it must not exceed 10. `filters` is also optional, for example `"filters": ["countrycode:sg"]` to restrict results to Singapore.

Output:

```json
{
  "results": [
    {
      "longitude": 103.859,
      "latitude": 1.2834,
      "coordinates": "103.859,1.2834",
      "formatted": "Marina Bay Sands, 10 Bayfront Avenue, 018956, Singapore",
      "place_id": "51667b3e...",
      "country_code": "sg",
      "result_type": "building"
    }
  ]
}
```

Results are ordered best first. The `coordinates` field is ready to pass to `route`. If nothing matches, the tool returns an error.

### Asking about places by name

For a prompt such as "how long from 'marina bay singapore' driving to 'changqi airport'?", the assistant geocodes each place and then routes:

1. `geocode` with `"text": "marina bay singapore"` → take `coordinates`
2. `geocode` with `"text": "changqi airport"` → take `coordinates`
3. `route` with the two `coordinates` values as `origin` and `destination` (omit `profile` for driving)

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
│  geocode tool        │
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
