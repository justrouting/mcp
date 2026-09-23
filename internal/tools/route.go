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
	Origin      string `json:"origin" jsonschema:"origin coordinates in longitude,latitude format"`
	Destination string `json:"destination" jsonschema:"destination coordinates in longitude,latitude format"`
	Profile     string `json:"profile,omitempty" jsonschema:"routing profile: set to \"motorcycle\" when the user's request mentions a motorcycle or motorbike; otherwise omit it or set it to \"car\" for the default driving profile"`
}

type RouteOutput struct {
	DistanceMeters  float64 `json:"distance_meters"`
	DurationSeconds float64 `json:"duration_seconds"`
}

func RegisterRouteTool(
	server *mcp.Server,
	cfg RouteConfig,
) {
	client := justrouting.NewClient(cfg.APIKey)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name: "route",
			Description: `
Calculate a route between two locations using JustRouting.

Returns the distance in meters and estimated travel duration in seconds.

Coordinates must use longitude,latitude format.
For example: 103.8198,1.3521

Optional "profile" input selects the routing profile:
- If the user's request mentions a motorcycle or motorbike, set profile to "motorcycle".
- Otherwise (the user asks to drive, or no vehicle is mentioned), omit profile or set it to "car" to get the default driving route.
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

	route, err := client.Routes.Get(
		ctx,
		&justrouting.RouteRequest{
			Origin:      origin,
			Destination: destination,
			Profile:     profile,
		},
	)
	if err != nil {
		return nil, RouteOutput{}, fmt.Errorf("route calculation failed: %w", err)
	}

	return nil, RouteOutput{
		DistanceMeters:  route.Distance,
		DurationSeconds: route.Duration,
	}, nil
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

// normalizeProfile maps user-facing profile names to JustRouting API
// profiles. An empty profile (field omitted) and car synonyms map to
// "driving", the API default. Motorcycle synonyms map to "motorcycle".
// Any other value is rejected so unsupported profiles fail fast with a
// clear error instead of silently returning a driving route.
func normalizeProfile(profile string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "car", "driving":
		return "driving", nil
	case "motorcycle", "motorbike":
		return "motorcycle", nil
	default:
		return "", fmt.Errorf(
			"unsupported profile %q: must be one of \"car\", \"driving\", \"motorcycle\", \"motorbike\"",
			profile,
		)
	}
}
