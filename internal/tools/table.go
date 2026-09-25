package tools

import (
	"context"
	"fmt"
	"strings"

	justrouting "github.com/justrouting/go-client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TableConfig struct {
	APIKey string
}

type TableInput struct {
	Coordinates  []string `json:"coordinates" jsonschema:"list of coordinates in longitude,latitude format, for example [\"103.8198,1.3521\", \"103.9915,1.3644\"]; at least 2 required; the position of each coordinate in this list is its index in the returned matrices"`
	Profile      string   `json:"profile,omitempty" jsonschema:"routing profile: set to \"motorcycle\" when the user's request mentions a motorcycle or motorbike; otherwise omit it or set it to \"car\" for the default driving profile"`
	Sources      []int    `json:"sources,omitempty" jsonschema:"optional subset of coordinates to use as matrix rows (sources), by index into the coordinates list; empty or omitted means all of them"`
	Destinations []int    `json:"destinations,omitempty" jsonschema:"optional subset of coordinates to use as matrix columns (destinations), by index into the coordinates list; empty or omitted means all of them"`
	Annotations  []string `json:"annotations,omitempty" jsonschema:"which matrices to compute: \"duration\", \"distance\", or both; omit to get both"`
}

// TableWaypointOutput describes one source or destination coordinate as the
// routing engine used it. Index ties it back to a position in the request's
// coordinates list, so matrix rows and columns can be mapped to places.
type TableWaypointOutput struct {
	Index    int               `json:"index"`
	Name     string            `json:"name,omitempty"`
	Location justrouting.Point `json:"location"`
	Distance float64           `json:"distance"`
}

type TableOutput struct {
	Code         string                `json:"code"`
	Message      string                `json:"message,omitempty"`
	Durations    [][]*float64          `json:"durations,omitempty"`
	Distances    [][]*float64          `json:"distances,omitempty"`
	Sources      []TableWaypointOutput `json:"sources"`
	Destinations []TableWaypointOutput `json:"destinations"`
}

func RegisterTableTool(
	server *mcp.Server,
	cfg TableConfig,
) {
	client := justrouting.NewClient(cfg.APIKey)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name: "table",
			Description: `
Calculate a matrix of travel durations and distances between many locations using JustRouting.

Coordinates must use longitude,latitude format, one string per place.
For example: ["103.8198,1.3521", "103.9915,1.3644"]

If the user gives place names or addresses instead of coordinates, do not
guess coordinates. Call the geocode tool first to look up every place, then
pass the returned "coordinates" values to this tool. Keep the places in a
fixed order: each position in your coordinates list is an index, the rows
and columns of the returned matrices are numbered by these indices, and
every source/destination object in the result carries an "index" field
pointing back to that position.

Use this tool when comparing several places at once, for example picking
the nearest of several drivers: put the customer first and the drivers
after, then read the first row (or column) of the returned matrices.

Returns "durations" in seconds and "distances" in meters, each indexed
[source][destination]. A pair the engine cannot connect is reported as
null and must not be read as zero.

Optional "sources" and "destinations" inputs restrict the matrix to
subsets of the coordinates, by index into the coordinates list. Omit
them to compute the full matrix between all coordinates.

Optional "annotations" input selects which matrices to compute:
"duration", "distance", or both. Omit it to get both.

Optional "profile" input selects the routing profile:
- If the user's request mentions a motorcycle or motorbike, set profile to "motorcycle".
- Otherwise (the user asks to drive, or no vehicle is mentioned), omit profile or set it to "car" to get the default driving route.
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
			Annotations:  annotations,
			Profile:      profile,
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

// buildWaypoints maps engine waypoints onto TableWaypointOutput, attaching
// the index each one had in the request's coordinates list. The mapping is
// bounded defensively: a nil waypoint or a response longer than the
// selection is truncated instead of causing a panic.
func buildWaypoints(waypoints []*justrouting.Waypoint, indices []int) []TableWaypointOutput {
	out := make([]TableWaypointOutput, 0, len(waypoints))
	for i, w := range waypoints {
		if w == nil || i >= len(indices) {
			break
		}
		out = append(out, TableWaypointOutput{
			Index:    indices[i],
			Name:     w.Name,
			Location: w.Location,
			Distance: w.Distance,
		})
	}
	return out
}

// normalizeAnnotations validates and deduplicates matrix annotations.
// Empty input returns nil so the client applies its default (both).
func normalizeAnnotations(annotations []string) ([]string, error) {
	if len(annotations) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(annotations))
	seen := make(map[string]bool, len(annotations))
	for _, a := range annotations {
		a = strings.ToLower(strings.TrimSpace(a))
		switch a {
		case "duration", "distance":
		default:
			return nil, fmt.Errorf(
				"invalid annotation %q: must be \"duration\" or \"distance\"",
				a,
			)
		}
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out, nil
}
