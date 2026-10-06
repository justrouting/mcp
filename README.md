# JustRouting MCP

[![CI](https://github.com/justrouting/mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/justrouting/mcp/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/justrouting/mcp)](https://goreportcard.com/report/github.com/justrouting/mcp)
[![Latest Release](https://img.shields.io/github/v/release/justrouting/mcp)](https://github.com/justrouting/mcp/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[![JustRouting MCP MCP server – quality and maintenance score on Glama](https://glama.ai/mcp/servers/justrouting/mcp/badges/card.svg)](https://glama.ai/mcp/servers/justrouting/mcp)


> Routing, geocoding, distance matrices, and vehicle routing for AI assistants — focused on Southeast Asia.

JustRouting MCP lets MCP-compatible AI assistants such as Claude and Cursor use [JustRouting](https://justrouting.tech/) for road routing, geocoding, distance and travel-time comparison, and multi-vehicle route optimization.

Designed for Southeast Asia.

## Quick Start

### 1. Install

#### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/justrouting/mcp/main/install.sh | sh
```

The installer downloads the latest prebuilt binary for your platform and installs `justrouting-mcp`.

No Go installation is required.

#### From source

```bash
go install github.com/justrouting/mcp/cmd/justrouting-mcp@latest
```

Make sure `justrouting-mcp` is available in your `PATH`.

### 2. Set your API key

Get an API key from [JustRouting](https://justrouting.tech/), then set:

```bash
export JUSTROUTING_API_KEY="your-api-key"
```

### 3. Connect to Claude or Cursor

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

The server communicates with MCP clients through `stdio`.

## Tools

| Tool       | Purpose                                                      |
| ---------- | ------------------------------------------------------------ |
| `geocode`  | Place or address → coordinates                               |
| `route`    | Calculate one route                                          |
| `table`    | Compare travel times or distances between multiple locations |
| `optimize` | Assign jobs to vehicles and order their stops                |

The tools are intentionally separated by task:

**route = calculate**
**table = compare**
**optimize = decide**

---

## `geocode`

Search for a place or address and convert it into coordinates.

Use it when the user provides:

* a place name
* a landmark
* a business
* a street address
* another location description

Do not use it when the user already provides coordinates.

### Input

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

The available fields are:

* `name` — place, landmark, or business name
* `housenumber` — house or building number
* `street` — street name
* `postcode` — postal or ZIP code
* `city` — city or locality
* `country` — country
* `limit` — maximum number of results, up to 10
* `filters` — optional search filters

Only provide fields that can be determined from the user's request. Do not invent missing address components.

Country and city can be useful for disambiguating common or abbreviated place names.

### Output

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

Results are ordered by relevance.

By default, the best matching result is returned. When multiple results are requested, choose the result that best matches the user's intended location.

The `coordinates` value is in `longitude,latitude` format and can be passed directly to `route`, `table`, or `optimize`.

If geocoding is used as part of a routing workflow, verify that the selected result matches the intended place before using its coordinates.

---

## `route`

Calculate a driving route between two locations.

Use it when the user needs to:

* calculate driving distance
* estimate travel time
* get a route between two locations
* get route geometry
* understand which roads a route uses
* avoid specific road types

### Input

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

For example:

```text
103.8198,1.3521
```

Optional motorcycle routing:

```json
{
  "origin": "103.8198,1.3521",
  "destination": "103.9915,1.3644",
  "profile": "motorcycle"
}
```

Use `profile: "motorcycle"` when the user explicitly asks for a motorcycle or motorbike route.

Otherwise, omit `profile` to use the default driving profile.

### Avoid roads

Use `exclude` when the user explicitly asks to avoid a type of road.

Supported values:

* `toll`
* `motorway`
* `ferry`

Example:

```json
{
  "origin": "103.8198,1.3521",
  "destination": "103.9915,1.3644",
  "exclude": ["toll"]
}
```

Road avoidance is a preference, not a guarantee. The routing engine avoids the specified road type when a reasonable alternative exists.

### Output

```json
{
  "distance_meters": 18500,
  "duration_seconds": 1500,
  "profile": "driving",
  "origin": {
    "input": "103.8198,1.3521",
    "snapped": [103.8197, 1.3520],
    "name": "Example Road",
    "distance": 4.2
  },
  "destination": {
    "input": "103.9915,1.3644",
    "snapped": [103.9915, 1.3644],
    "name": "Airport Boulevard",
    "distance": 8.1
  },
  "geometry": "ka|`@_ceeEnAqB...",
  "summary": {
    "roads": ["East Coast Parkway"],
    "tolls": false,
    "ferry": false
  }
}
```

`distance_meters` is the route distance in meters.

`duration_seconds` is the estimated travel duration in seconds.

`geometry` is a simplified encoded polyline that can be used to render the route on a map.

`summary.roads` lists the main roads used by the route.

`summary.tolls` indicates whether the route uses toll roads.

`summary.ferry` indicates whether the route includes a ferry crossing.

If the user provides place names or addresses instead of coordinates, call `geocode` first.

---

## `table`

Calculate a distance and/or travel-time matrix between multiple locations.

Use it when the user needs to:

* find the nearest driver, vehicle, store, or facility
* compare several destinations
* compare multiple origin-destination pairs
* build a distance or travel-time matrix

Do not use it for a single route between two locations. Use `route` instead.

### Input

```json
{
  "coordinates": [
    "103.8391,1.2771",
    "103.8590,1.2834",
    "103.7986,1.2885"
  ],
  "annotations": ["distance"]
}
```

Each coordinate has a stable zero-based index based on its position in the `coordinates` list.

Matrix rows correspond to **sources**.

Matrix columns correspond to **destinations**.

A matrix value at:

```text
[row][column]
```

represents the route from that source to that destination.

For example:

```text
durations[0][2]
```

is the travel time from source index `0` to destination index `2`.

The `sources` and `destinations` returned by the tool contain the original coordinate indices so the matrix can be mapped back to the user's locations.

### Nearest-driver example

For a question such as:

> Which driver is closest to this customer?

Put the customer first:

```text
coordinates[0] = customer
coordinates[1] = driver A
coordinates[2] = driver B
coordinates[3] = driver C
```

Then use the customer as the source and the drivers as destinations.

Read the corresponding matrix row and choose the smallest non-null distance or duration.

### Selecting sources and destinations

`source` and `destinations` can restrict the matrix to specific coordinate indices.

For example:

```json
{
  "coordinates": [
    "103.8391,1.2771",
    "103.8590,1.2834",
    "103.7986,1.2885"
  ],
  "sources": [0],
  "destinations": [1, 2]
}
```

This calculates routes from coordinate `0` to coordinates `1` and `2`.

### Annotations

Use:

```json
"annotations": ["duration"]
```

for travel times.

Use:

```json
"annotations": ["distance"]
```

for distances.

Use:

```json
"annotations": ["duration", "distance"]
```

for both.

If omitted, both are returned.

### Output

```json
{
  "code": "Ok",
  "durations": [
    [0, 1860, null],
    [1850, 0, 2100],
    [null, 2110, 0]
  ],
  "distances": [
    [0, 14200, null],
    [14100, 0, 18400],
    [null, 18200, 0]
  ],
  "sources": [
    {
      "index": 0,
      "name": "Duxton Road",
      "location": [103.8391, 1.2771],
      "distance": 12.3
    }
  ],
  "destinations": [
    {
      "index": 1,
      "name": "Bayfront Avenue",
      "location": [103.8590, 1.2834],
      "distance": 8.4
    }
  ]
}
```

`durations` are in seconds.

`distances` are in meters.

A matrix value of `null` means that the origin-destination pair is unreachable. Never interpret `null` as zero.

---

## `optimize`

Assign jobs to vehicles and determine the order in which each vehicle should visit its assigned jobs.

Use it when the user needs to:

* assign multiple deliveries or pickups to vehicles
* plan routes for a fleet
* determine the order of multiple stops
* optimize a multi-vehicle routing solution

Do not use it for a single route. Use `route`.

Do not use it only to compare distances between locations. Use `table`.

### Input

```json
{
  "vehicles": [
    {
      "id": 1,
      "start": "103.7923,1.3246",
      "end": "103.7923,1.3246"
    },
    {
      "id": 2,
      "start": "103.8232,1.3241",
      "end": "103.8232,1.3241"
    }
  ],
  "jobs": [
    {
      "id": 1,
      "location": "103.7975,1.3104"
    },
    {
      "id": 2,
      "location": "103.7843,1.3149"
    },
    {
      "id": 3,
      "location": "103.7976,1.3198"
    }
  ]
}
```

Each vehicle must have at least a `start` or an `end`.

For a round trip, set both to the same location:

```json
{
  "id": 1,
  "start": "103.7923,1.3246",
  "end": "103.7923,1.3246"
}
```

If `start` or `end` is omitted, the vehicle may start or end anywhere.

Each vehicle and job must have a unique `id`.

Locations must use:

```text
longitude,latitude
```

### Vehicle profiles

Each vehicle can use its own routing profile.

For example:

```json
{
  "id": 1,
  "profile": "motorcycle",
  "start": "103.7923,1.3246",
  "end": "103.7923,1.3246"
}
```

Use `"motorcycle"` when the user explicitly asks for a motorcycle or motorbike route.

Otherwise, omit `profile` to use the default driving profile.

### Output

A simplified result looks like:

```json
{
  "summary": {
    "cost": 2733,
    "routes": 2,
    "unassigned": 0,
    "duration": 2733
  },
  "routes": [
    {
      "vehicle": 1,
      "cost": 1144,
      "duration": 1144,
      "steps": [
        {
          "type": "start",
          "location": [103.7923, 1.3246],
          "arrival": 0
        },
        {
          "type": "job",
          "job": 2,
          "location": [103.7843, 1.3149],
          "arrival": 269,
          "duration": 269
        },
        {
          "type": "job",
          "job": 1,
          "location": [103.7975, 1.3104],
          "arrival": 673,
          "duration": 673
        },
        {
          "type": "end",
          "location": [103.7923, 1.3246],
          "arrival": 1144,
          "duration": 1144
        }
      ]
    }
  ],
  "unassigned": []
}
```

Read each route's `steps` in order to determine:

* which vehicle serves each job
* the order in which jobs are visited
* when each job is reached

Always check `unassigned`.

Do not assume that every job can be assigned to a vehicle.

If the user provides place names or addresses instead of coordinates, call `geocode` first for every location and use the selected coordinates in the optimization request.

---

## Agent Workflows

JustRouting MCP is designed so that AI assistants can combine the tools into larger workflows.

### Route between two places

For:

> How long does it take to drive from Marina Bay Sands to Changi Airport?

The workflow is:

```text
geocode → geocode → route
```

1. Geocode the origin.
2. Geocode the destination.
3. Pass the resulting coordinates to `route`.

If the user already provides coordinates, skip geocoding.

### Find the nearest driver

For:

> Which driver is closest to the customer?

The workflow is:

```text
geocode × N → table
```

1. Geocode the customer.
2. Geocode each driver.
3. Keep the locations in a fixed order.
4. Use the customer as the source.
5. Use the drivers as destinations.
6. Compare the returned distances or durations.

### Plan multiple vehicles

For:

> I have two vans and six customers. Assign the customers to the vans and determine the stop order.

The workflow is:

```text
geocode × N → optimize
```

1. Geocode each vehicle's start/end location.
2. Geocode every job location.
3. Create one vehicle entry per vehicle.
4. Create one job entry per customer.
5. Call `optimize`.
6. Read `routes[].steps` to determine each vehicle's itinerary.
7. Check `unassigned` for jobs that could not be served.

---

## Coordinate Format

All routing tools use:

```text
longitude,latitude
```

For example:

```text
103.8198,1.3521
```

This is:

```text
longitude = 103.8198
latitude  = 1.3521
```

Do not reverse the order.

---

## Claude

Add JustRouting MCP to your Claude MCP configuration:

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

Once connected, Claude can use the JustRouting tools directly.

---

## Cursor

Add JustRouting MCP to your Cursor MCP configuration:

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

Once connected, Cursor can use the JustRouting tools directly.

---

## Requirements

For prebuilt binaries:

* A JustRouting API key

For installation from source:

* Go 1.25+

The MCP server currently uses `stdio` transport.

---

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

---

## Architecture

```text
┌─────────────────────────────┐
│      MCP Client             │
│   Claude / Cursor / AI      │
└──────────────┬──────────────┘
               │
               │ MCP / stdio
               ▼
┌─────────────────────────────┐
│       justrouting-mcp       │
│                             │
│  geocode   route            │
│  table     optimize         │
└──────────────┬──────────────┘
               │
               │ Official Go Client
               ▼
┌─────────────────────────────┐
│    api.justrouting.tech     │
└─────────────────────────────┘
```

The MCP server is intentionally thin.

It exposes JustRouting capabilities through MCP and uses the official [JustRouting Go client](https://github.com/justrouting/go-client) for API communication.

---

## Releases

Prebuilt binaries are published through [GitHub Releases](https://github.com/justrouting/mcp/releases).

To create a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions builds the binaries and publishes the release.

The one-line installer always installs the latest release.

---

## Roadmap

The core routing capabilities are already available:

* Route calculation
* Geocoding
* Distance and travel-time matrix
* Vehicle routing optimization

Future improvements may include:

* Alternative routes
* Waypoints
* Streamable HTTP transport
* Remote MCP deployment

---

## License

[MIT](./LICENSE)
