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
Calculate a driving route between two locations using JustRouting.

Returns the driving distance in meters and estimated travel duration
in seconds.

Coordinates must use longitude,latitude format.
For example: 103.8198,1.3521
			`,
		},
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input RouteInput,
		) (*mcp.CallToolResult, RouteOutput, error) {
			return calculateRoute(ctx, client, input)
		},
	)
}

func calculateRoute(
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

	route, err := client.Routes.Get(
		ctx,
		&justrouting.RouteRequest{
			Origin:      origin,
			Destination: destination,
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
