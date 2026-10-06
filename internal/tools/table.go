package tools

import (
	"context"
	"fmt"

	justrouting "github.com/justrouting/go-client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TableConfig struct {
	APIKey string
}

type TableInput struct {
	Coordinates  []string `json:"coordinates" jsonschema:"list of coordinates in longitude,latitude format, for example [\"103.8198,1.3521\", \"103.9915,1.3644\"]; at least 2 required; the position of each coordinate in this list is its index in the returned matrices"`
	Profile      Profile  `json:"profile,omitempty" jsonschema:"routing profile: set to \"motorcycle\" when the user's request mentions a motorcycle or motorbike; otherwise omit it for the default driving profile"`
	Sources      []int    `json:"sources,omitempty" jsonschema:"optional subset of coordinates to use as matrix rows (sources), by index into the coordinates list; empty or omitted means all of them"`
	Destinations []int    `json:"destinations,omitempty" jsonschema:"optional subset of coordinates to use as matrix columns (destinations), by index into the coordinates list; empty or omitted means all of them"`
	Annotations  []Annotation `json:"annotations,omitempty" jsonschema:"which matrices to compute: \"duration\", \"distance\", or both; omit to get both"`
}

// TableWaypoint describes one source or destination coordinate as the
// routing engine used it. Index ties it back to a position in the request's
// coordinates list, so matrix rows and columns can be mapped to places.
type TableWaypoint struct {
	Index    int               `json:"index"`
	Name     string            `json:"name,omitempty"`
	Location justrouting.Point `json:"location"`
	Distance float64           `json:"distance"`
}

type TableOutput struct {
	Code         string          `json:"code"`
	Message      string          `json:"message,omitempty"`
	Durations    [][]*float64    `json:"durations,omitempty"`
	Distances    [][]*float64    `json:"distances,omitempty"`
	Sources      []TableWaypoint `json:"sources"`
	Destinations []TableWaypoint `json:"destinations"`
}

func RegisterTableTool(
	server *mcp.Server,
	cfg TableConfig,
) {
	client := justrouting.NewClient(cfg.APIKey)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "table",
			InputSchema: mustSchema[TableInput]("table input schema"),
			Description: `
Calculate a travel-time and/or distance matrix between multiple locations using JustRouting.

Use this tool when the user needs to compare travel times or distances between
multiple origins and destinations, such as:
- finding the nearest driver, vehicle, store, or facility
- comparing which destination is closest to an origin
- comparing multiple origin-destination pairs
- building a distance or travel-time matrix for several locations

Do not use this tool for:
- a single route between two locations; use the route tool
- assigning jobs to vehicles or determining the order of multiple stops; use the optimize tool

Coordinates:
- Each coordinate must be in longitude,latitude format.
- For example: ["103.8198,1.3521", "103.9915,1.3644"]
- Each coordinate has a stable zero-based index based on its position in the
  coordinates list.
- Matrix rows correspond to sources and columns correspond to destinations.
- A matrix value at [row][column] represents the route from that source to
  that destination.
- The sources and destinations in the result include their original
  coordinate indices, so use these indices to map matrix rows and columns
  back to the user's locations.

If the user provides place names or addresses instead of coordinates, do not
guess their coordinates. Call the geocode tool first for every place, then
pass each place's best geocoded coordinates to this tool. Keep the same
order as the user's locations so the matrix can be mapped back correctly.

For example, if the user asks which driver is closest to a customer:
- put the customer in the coordinates list
- put the drivers after it
- use the customer as the source and the drivers as destinations
- read the corresponding row of the matrix and choose the smallest
  non-null distance or duration

Unreachable origin-destination pairs are returned as null. Never interpret
null as zero or as a valid route.

Optional "sources" and "destinations" select subsets of the coordinates by
zero-based index. Omit them to calculate the full matrix.

Optional "annotations" controls which matrices are returned:
- "duration" for travel times in seconds
- "distance" for distances in meters
- ["duration", "distance"] for both
- omit it to return both

Optional "profile" selects the routing profile:
- Set profile to "motorcycle" when the user explicitly asks for a
  motorcycle or motorbike route.
- Otherwise, omit profile to use the default driving profile.
			`,
		},
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input TableInput,
		) (*mcp.CallToolResult, TableOutput, error) {
			return getTable(ctx, client, input)
		},
	)
}

func getTable(
	ctx context.Context,
	client *justrouting.Client,
	input TableInput,
) (*mcp.CallToolResult, TableOutput, error) {
	if len(input.Coordinates) < 2 {
		return nil, TableOutput{}, fmt.Errorf(
			"at least 2 coordinates are required, got %d",
			len(input.Coordinates),
		)
	}

	profile, err := normalizeProfile(input.Profile)
	if err != nil {
		return nil, TableOutput{}, fmt.Errorf("invalid profile: %w", err)
	}

	points := make([]justrouting.Point, len(input.Coordinates))
	for i, raw := range input.Coordinates {
		points[i], err = parsePoint(raw)
		if err != nil {
			return nil, TableOutput{}, fmt.Errorf("invalid coordinates[%d]: %w", i, err)
		}
	}

	// Validate index selections here so the LLM gets a clear message
	// instead of the client's validation error, and so no request is spent
	// on a selection the API would reject.
	if err := validateIndices("sources", input.Sources, len(points)); err != nil {
		return nil, TableOutput{}, err
	}
	if err := validateIndices("destinations", input.Destinations, len(points)); err != nil {
		return nil, TableOutput{}, err
	}

	// An omitted annotations list stays nil so the client applies its
	// default of computing both matrices.
	annotations, err := normalizeAnnotations(input.Annotations)
	if err != nil {
		return nil, TableOutput{}, err
	}

	resp, err := client.Matrix.Get(
		ctx,
		&justrouting.MatrixRequest{
			Coordinates:  points,
			Sources:      input.Sources,
			Destinations: input.Destinations,
			Annotations:  annotationStrings(annotations),
			Profile:      string(profile),
		},
	)
	if err != nil {
		return nil, TableOutput{}, fmt.Errorf("table calculation failed: %w", err)
	}

	// The response lists sources and destinations in the order of the
	// request's effective selection, so indices are computed from the
	// selection lists (or 0..n-1 when omitted), never from response
	// position.
	sourceIndices := effectiveIndices(input.Sources, len(points))
	destinationIndices := effectiveIndices(input.Destinations, len(points))

	return nil, TableOutput{
		Code:         resp.Code,
		Message:      resp.Message,
		Durations:    resp.Durations,
		Distances:    resp.Distances,
		Sources:      buildWaypoints(resp.Sources, sourceIndices),
		Destinations: buildWaypoints(resp.Destinations, destinationIndices),
	}, nil
}

// validateIndices rejects selection indices outside the coordinate slice
// before a request is spent.
func validateIndices(field string, sel []int, total int) error {
	for i, idx := range sel {
		if idx < 0 || idx >= total {
			return fmt.Errorf(
				"invalid %s[%d] = %d: index must be between 0 and %d for %d coordinates",
				field, i, idx, total-1, total,
			)
		}
	}
	return nil
}

// effectiveIndices returns the selection itself, or 0..n-1 when the
// selection is empty, matching the API default of using all coordinates.
func effectiveIndices(sel []int, n int) []int {
	if len(sel) > 0 {
		return sel
	}
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// buildWaypoints maps engine waypoints onto TableWaypoint, attaching
// the index each one had in the request's coordinates list. The mapping is
// bounded defensively: a nil waypoint or a response longer than the
// selection is truncated instead of causing a panic.
func buildWaypoints(waypoints []*justrouting.Waypoint, indices []int) []TableWaypoint {
	out := make([]TableWaypoint, 0, len(waypoints))
	for i, w := range waypoints {
		if w == nil || i >= len(indices) {
			break
		}
		out = append(out, TableWaypoint{
			Index:    indices[i],
			Name:     w.Name,
			Location: w.Location,
			Distance: w.Distance,
		})
	}
	return out
}

