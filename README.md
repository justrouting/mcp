# JustRouting MCP

MCP server for [JustRouting](https://justrouting.tech/), a Southeast Asia-focused routing API.

Use JustRouting's road-routing capabilities from MCP-compatible AI assistants such as Claude and Cursor.

## Features

* Driving route calculation
* Motorcycle routing profile
* Driving distance
* Estimated travel duration
* Distance and duration matrix (table)
* Address and place search (geocoding)
* Southeast Asia-focused road coverage
* MCP stdio transport
* Built on the official JustRouting Go client

## Requirements

* A JustRouting API key
* Go 1.25+ (only needed when installing from source)

## Installation

### One-line installer (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/justrouting/mcp/main/install.sh | sh
```

Downloads the latest prebuilt binary for your OS/architecture from [GitHub Releases](https://github.com/justrouting/mcp/releases), verifies its checksum, and installs it to `/usr/local/bin` (or `~/.local/bin`). No Go required.

Pin a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/justrouting/mcp/main/install.sh | JUSTROUTING_MCP_VERSION=v0.1.1 sh
```

Uninstall: remove the `justrouting-mcp` binary from the directory it was installed to.

### Manual download

Prebuilt binaries for macOS, Linux, and Windows are attached to every [release](https://github.com/justrouting/mcp/releases). Download the archive for your platform, extract it, and put the binary on your `PATH`.

### From source (Go users)

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

Search for places and convert a place name or address into coordinates. Use this before `route` when the user refers to places by name.

The search is structured: the assistant parses the user's place reference into address components and passes only the ones it can determine — `name`, `housenumber`, `street`, `postcode`, `city`, `country`. Components with no data are omitted. Prefer including `country` (and `city`) when the context implies them — they are the strongest disambiguators for common, abbreviated, or misspelled names.

Input:

```json
{
  "name": "Marina Bay Sands",
  "housenumber": "10",
  "street": "Bayfront Avenue",
  "postcode": "018956",
  "city": "Singapore",
  "country": "Singapore"
}
```

At least one component is required. `limit` is optional and defaults to 1 (best match only); it must not exceed 10. `filters` is also optional, for example `"filters": ["countrycode:sg"]` to restrict results to Singapore.

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

Results are ordered best first. The `coordinates` field is ready to pass to `route` or `table`. If nothing matches, the tool returns an error.

### `table`

Calculate a matrix of driving durations and distances between many locations at once. Use it to compare several places — for example, to find which of several drivers is nearest to a customer.

Input:

```json
{
  "coordinates": [
    "103.8391,1.2771",
    "103.859,1.2834",
    "103.7986,1.2885"
  ],
  "annotations": ["distance"]
}
```

`coordinates` is required and must hold at least two entries in `longitude,latitude` format (as returned by `geocode`). The position of each entry is its index: matrix rows and columns, and the `index` field of every source/destination in the output, refer back to it. Pass the places in a fixed order — for a nearest-driver question, put the customer first and the drivers after, then read the first row.

`sources` and `destinations` optionally restrict the matrix to subsets of the coordinates by index (empty or omitted means all of them). `annotations` optionally selects `"duration"`, `"distance"`, or both (default both). `profile` works exactly as in `route` — set `"motorcycle"` when the user mentions a motorcycle or motorbike, otherwise omit it (or use `"car"`) for driving.

Output:

```json
{
  "code": "Ok",
  "durations": [[0, 1860, null], [1850, 0, 2100], [null, 2110, 0]],
  "distances": [[0, 14200, null], [14100, 0, 18400], [null, 18200, 0]],
  "sources": [
    {"index": 0, "name": "Duxton Road", "location": [103.8391, 1.2771], "distance": 12.3},
    {"index": 1, "name": "Bayfront Avenue", "location": [103.859, 1.2834], "distance": 8.4},
    {"index": 2, "name": "Alexandra Road", "location": [103.7986, 1.2885], "distance": 6.1}
  ],
  "destinations": [
    {"index": 0, "name": "Duxton Road", "location": [103.8391, 1.2771], "distance": 12.3},
    {"index": 1, "name": "Bayfront Avenue", "location": [103.859, 1.2834], "distance": 8.4},
    {"index": 2, "name": "Alexandra Road", "location": [103.7986, 1.2885], "distance": 6.1}
  ]
}
```

`durations` is in seconds and `distances` in meters, each indexed `[source][destination]`. A `null` entry means the engine could not connect that pair — it is not zero. Every source and destination object carries an `index` back to the input `coordinates` list.

### Asking about places by name

For a prompt such as "how long from 'marina bay singapore' driving to 'changqi airport'?", the assistant geocodes each place and then routes:

1. `geocode` with `"name": "marina bay", "country": "singapore"` → take `coordinates`
2. `geocode` with `"name": "changqi airport"` → take `coordinates`
3. `route` with the two `coordinates` values as `origin` and `destination` (omit `profile` for driving)

For a comparison such as "which of these 4 drivers is nearest to the customer?", geocode the customer and every driver, pass all `coordinates` values to `table` in a fixed order — customer first — and read the first row of the returned matrices.

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

## Releasing

Tag and push — GitHub Actions builds the binaries and publishes the release:

```bash
git tag v0.1.1
git push origin v0.1.1
```

The `install.sh` script always installs the latest release, so it never needs to be updated when a new version ships.

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
│  table tool          │
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
* [x] Distance matrix
* [ ] Route optimization
* [ ] Streamable HTTP
* [ ] Remote MCP deployment

## License

[MIT](./LICENSE)
