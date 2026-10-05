package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/justrouting/mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	name = "justrouting"
)

// version is injected at build time via
// -ldflags "-X github.com/justrouting/mcp/internal/server.version=vX.Y.Z"
// (see .goreleaser.yaml). Local builds without ldflags report "dev".
var version = "dev"

type Config struct {
	APIKey string
	Logger *slog.Logger
}

type Server struct {
	mcpServer *mcp.Server
}

func New(cfg Config) (*Server, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("JUSTROUTING_API_KEY is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    name,
			Version: version,
		},
		&mcp.ServerOptions{
			Instructions: `
JustRouting provides road routing, geocoding and fleet optimization across Southeast Asia.

The route tool calculates a route between two locations: distance in meters, estimated travel duration in seconds, the route's polyline geometry, and the main roads traveled. Use it when the user asks for the distance, travel time or a route between two places. The default routing profile is driving; when the user's request mentions a motorcycle or motorbike, pass "motorcycle" as the route tool's profile input.

The table tool computes a matrix of driving distances and durations between many coordinates at once. Use it when comparing several places (for example, finding the nearest of several drivers), and read the returned matrix by list position.

The optimize tool solves vehicle routing problems: it assigns jobs to vehicles and orders each vehicle's stops. Use it when the user asks to plan deliveries or visits with one or more vehicles. Read the response's "routes" array: each route's "steps", in order, show which jobs that vehicle serves and when it arrives; "unassigned" lists the jobs no vehicle could serve.

All three routing tools take coordinates as longitude,latitude. When the user asks about places by name or address, call the geocode tool first for every place and pass each call's first (best) result "coordinates" value to the routing tool: as "origin" and "destination" for route, as one ordered list for table, and as one vehicle entry per vehicle plus one job entry per task for optimize.
			`,
			Logger: cfg.Logger,
		},
	)

	tools.RegisterRouteTool(
		mcpServer,
		tools.RouteConfig{
			APIKey: cfg.APIKey,
		},
	)

	tools.RegisterGeocodeTool(
		mcpServer,
		tools.GeocodeConfig{
			APIKey: cfg.APIKey,
		},
	)

	tools.RegisterTableTool(
		mcpServer,
		tools.TableConfig{
			APIKey: cfg.APIKey,
		},
	)

	tools.RegisterOptimizeTool(
		mcpServer,
		tools.OptimizeConfig{
			APIKey: cfg.APIKey,
		},
	)

	return &Server{
		mcpServer: mcpServer,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(
		ctx,
		&mcp.StdioTransport{},
	)
}
