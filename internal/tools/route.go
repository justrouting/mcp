package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	justrouting "github.com/justrouting/go-client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RouteConfig struct {
	APIKey string
}

type RouteInput struct {
	Origin      string   `json:"origin" jsonschema:"origin coordinates in longitude,latitude format"`
	Destination string   `json:"destination" jsonschema:"destination coordinates in longitude,latitude format"`
	Profile     Profile  `json:"profile,omitempty" jsonschema:"routing profile: set to \"motorcycle\" when the user's request mentions a motorcycle or motorbike; otherwise omit it for the default driving profile"`
	Exclude     []string `json:"exclude,omitempty" jsonschema:"road classes to avoid, for example [\"toll\"] when the user asks to avoid toll roads; supported values are \"toll\", \"motorway\", \"ferry\""`
}

type RouteOutput struct {
	DistanceMeters  float64            `json:"distance_meters" jsonschema:"route length in meters"`
	DurationSeconds float64            `json:"duration_seconds" jsonschema:"estimated travel time in seconds"`
	Profile         Profile            `json:"profile" jsonschema:"routing profile used: \"driving\" or \"motorcycle\""`
	Origin          RoutePointOutput   `json:"origin" jsonschema:"the origin as the routing engine used it: the requested coordinate plus where it snapped to the road network"`
	Destination     RoutePointOutput   `json:"destination" jsonschema:"the destination as the routing engine used it: the requested coordinate plus where it snapped to the road network"`
	Geometry        string             `json:"geometry,omitempty" jsonschema:"the route's shape as an encoded polyline (simplified overview); omitted when the engine returned none"`
	Exclude         []string           `json:"exclude,omitempty" jsonschema:"the road classes the route avoids; omitted when none were requested"`
	Summary         RouteSummaryOutput `json:"summary" jsonschema:"high-level description of the route"`
}

// RoutePointOutput reports one endpoint of the route. When the engine did
// not report snapping data for the endpoint, only Input is set.
type RoutePointOutput struct {
	Input    string            `json:"input" jsonschema:"the coordinate as requested, in longitude,latitude format"`
	Snapped  justrouting.Point `json:"snapped,omitempty" jsonschema:"the coordinate snapped to the nearest road, as [longitude, latitude]"`
	Name     string            `json:"name,omitempty" jsonschema:"the street this endpoint snapped to, if known"`
	Distance float64           `json:"distance,omitempty" jsonschema:"meters between the requested coordinate and the snapped position"`
}

type RouteSummaryOutput struct {
	MajorRoads []string `json:"major_roads" jsonschema:"main roads the route travels, in order of travel; consecutive repeats are collapsed"`
	Tolls      bool     `json:"tolls" jsonschema:"true when the route passes through toll roads"`
	Ferry      bool     `json:"ferry" jsonschema:"true when the route includes a ferry crossing"`
}

func RegisterRouteTool(
	server *mcp.Server,
	cfg RouteConfig,
) {
	client := justrouting.NewClient(cfg.APIKey)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:         "route",
			InputSchema:  mustSchema[RouteInput]("route input schema"),
			OutputSchema: mustSchema[RouteOutput]("route output schema"),
			Description: `
Calculate a route between two locations using JustRouting: distance in meters, estimated travel duration in seconds, the route's polyline geometry, the snapped origin and destination, the main roads traveled, and whether the route uses toll roads or a ferry.

Use this tool when the user asks for the distance, travel time or a route between two places. For comparing many places at once (for example, which of several drivers is nearest) use the table tool. For assigning jobs to vehicles and ordering their stops use the optimize tool.

Coordinates must use longitude,latitude format.
For example: 103.8198,1.3521

If the user gives place names or addresses instead of coordinates, do not
guess coordinates. Call the geocode tool first to look up each place, then
pass the "coordinates" value of its first (best) result to this tool.

Optional "profile" input selects the routing profile:
- If the user's request mentions a motorcycle or motorbike, set profile to "motorcycle".
- Otherwise (the user asks to drive, or no vehicle is mentioned), omit profile to get the default driving route.

Optional "exclude" input lists road classes to avoid, for example ["toll"] when the user asks to avoid toll roads. Supported values are "toll", "motorway" and "ferry". The engine routes around them where a reasonable alternative exists.
			`,
		},
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input RouteInput,
		) (*mcp.CallToolResult, RouteOutput, error) {
			return getRoute(ctx, client, input)
		},
	)
}

func getRoute(
	ctx context.Context,
	client *justrouting.Client,
	input RouteInput,
) (*mcp.CallToolResult, RouteOutput, error) {
	origin, err := parsePoint(input.Origin)
	if err != nil {
		return nil, RouteOutput{}, fmt.Errorf("invalid origin: %w", err)
	}

	destination, err := parsePoint(input.Destination)
	if err != nil {
		return nil, RouteOutput{}, fmt.Errorf("invalid destination: %w", err)
	}

	profile, err := normalizeProfile(input.Profile)
	if err != nil {
		return nil, RouteOutput{}, fmt.Errorf("invalid profile: %w", err)
	}

	exclude, err := normalizeExclude(input.Exclude)
	if err != nil {
		return nil, RouteOutput{}, fmt.Errorf("invalid exclude: %w", err)
	}

	resp, err := client.Routes.GetAll(
		ctx,
		&justrouting.RouteRequest{
			Origin:      origin,
			Destination: destination,
			Profile:     string(profile),
			Exclude:     exclude,
			// Steps is the only way the engine reports intersection road
			// classes and leg summaries: the classes feed the summary
			// tolls/ferry booleans and the summaries fill major_roads
			// (both are empty without them). The steps themselves never
			// enter the tool output, so the MCP response shape is
			// unchanged.
			Steps: true,
			// Explicitly request the compact polyline overview so the
			// response stays small and deterministic for LLM context.
			Geometries: "polyline",
			Overview:   "simplified",
		},
	)
	if err != nil {
		return nil, RouteOutput{}, fmt.Errorf("route calculation failed: %w", err)
	}

	// GetAll does not convert an empty routes array into an error (Routes.Get
	// does), so mirror that check here: an Ok response with no routes means
	// the points cannot be connected. Non-Ok engine codes already arrive as
	// errors from the client's response decoding.
	if len(resp.Routes) == 0 || resp.Routes[0] == nil {
		return nil, RouteOutput{}, fmt.Errorf(
			"route calculation failed: no route found between the given coordinates",
		)
	}
	route := resp.Routes[0]

	tolls, ferry := roadClasses(route.Legs)

	out := RouteOutput{
		DistanceMeters:  route.Distance,
		DurationSeconds: route.Duration,
		Profile:         profile,
		Origin:          buildRoutePoint(origin, waypointAt(resp.Waypoints, 0)),
		Destination:     buildRoutePoint(destination, waypointAt(resp.Waypoints, 1)),
		Exclude:         exclude,
		Summary: RouteSummaryOutput{
			MajorRoads: majorRoads(route.Legs),
			Tolls:      tolls,
			Ferry:      ferry,
		},
	}

	// Geometry is an enrichment: a missing or undecodable geometry must not
	// fail the whole call.
	if geometry, err := route.Geometry.Polyline(); err == nil {
		out.Geometry = geometry
	}

	return nil, out, nil
}

// waypointAt returns the i-th snapped waypoint, or nil when the engine
// returned fewer. The route request always has exactly origin and
// destination, so only indices 0 and 1 are ever used.
func waypointAt(waypoints []*justrouting.Waypoint, i int) *justrouting.Waypoint {
	if i < len(waypoints) {
		return waypoints[i]
	}
	return nil
}

// buildRoutePoint maps an input coordinate and its optional snapped waypoint
// onto RoutePointOutput. A nil waypoint leaves only the echoed input
// coordinate, so endpoint objects stay present even when the engine reports
// no snapping data.
func buildRoutePoint(input justrouting.Point, snapped *justrouting.Waypoint) RoutePointOutput {
	out := RoutePointOutput{Input: input.String()}
	if snapped == nil {
		return out
	}
	out.Snapped = snapped.Location
	out.Name = snapped.Name
	out.Distance = snapped.Distance
	return out
}

// majorRoads collapses the per-leg road summaries into the ordered list of
// main roads the route travels. Consecutive duplicates (adjacent legs on the
// same road) are collapsed, but a road that recurs after leaving it is kept;
// empty summaries and nil legs are dropped.
func majorRoads(legs []*justrouting.Leg) []string {
	out := make([]string, 0, len(legs))
	var prev string
	for _, leg := range legs {
		if leg == nil {
			continue
		}
		name := strings.TrimSpace(leg.Summary)
		if name == "" || name == prev {
			continue
		}
		out = append(out, name)
		prev = name
	}
	return out
}

// roadClasses reports whether the route touches toll roads or a ferry,
// from the per-intersection road classes the engine attaches to step
// intersections. The classes are only present when steps are requested;
// nil legs, steps and intersections contribute nothing. If a real route
// ever shows a missed ferry, step.Mode == "ferry" is the fallback signal
// to add here.
func roadClasses(legs []*justrouting.Leg) (tolls, ferry bool) {
	for _, leg := range legs {
		if leg == nil {
			continue
		}
		for _, step := range leg.Steps {
			if step == nil {
				continue
			}
			for _, intersection := range step.Intersections {
				if intersection == nil {
					continue
				}
				for _, class := range intersection.Classes {
					switch strings.ToLower(class) {
					case "toll":
						tolls = true
					case "ferry":
						ferry = true
					}
				}
			}
		}
	}
	return tolls, ferry
}

func parsePoint(value string) (justrouting.Point, error) {
	parts := strings.Split(strings.TrimSpace(value), ",")

	if len(parts) != 2 {
		return nil, fmt.Errorf(
			"expected longitude,latitude, got %q",
			value,
		)
	}

	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid longitude: %w", err)
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid latitude: %w", err)
	}

	point := justrouting.Point{lon, lat}

	if err := point.Validate(); err != nil {
		return nil, err
	}

	return point, nil
}

// normalizeExclude validates and normalizes the road classes to avoid.
// Supported values are the standard OSRM car-profile classes: "toll",
// "motorway" and "ferry". Each value is trimmed and lowercased; duplicates
// are dropped. Unknown values are rejected so unsupported classes fail fast
// with a clear error instead of reaching the engine. An empty input returns
// nil, which the client omits from the request.
func normalizeExclude(exclude []string) ([]string, error) {
	var out []string
	seen := make(map[string]bool, len(exclude))
	for _, class := range exclude {
		class = strings.ToLower(strings.TrimSpace(class))
		switch class {
		case "toll", "motorway", "ferry":
		default:
			return nil, fmt.Errorf(
				"unsupported road class %q: must be one of \"toll\", \"motorway\", \"ferry\"",
				class,
			)
		}
		if !seen[class] {
			seen[class] = true
			out = append(out, class)
		}
	}
	return out, nil
}
