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
JustRouting provides road routing, geocoding, distance matrices, and fleet optimization across Southeast Asia.

Choose the simplest tool that matches the user's task:
- geocode: find coordinates for a place or address
- route: calculate one route between two locations
- table: compare distances or travel times between multiple locations
- optimize: assign multiple jobs to vehicles and determine their stop order

Use geocode first when the user provides place names, landmarks, businesses, or addresses instead of coordinates. Do not guess coordinates. Verify that the selected geocoding result matches the user's intended location before using it for routing.

All routing tools use coordinates in longitude,latitude order.

Use route for a single origin-destination route. Use table when comparing multiple origin-destination pairs, such as finding the nearest driver, vehicle, store, or facility. Use optimize when the user needs multiple jobs assigned to vehicles and the order of stops determined.

For motorcycle or motorbike requests, set the routing profile to "motorcycle". Otherwise, use the default driving profile.

When combining tools, preserve the user's location order when constructing table or optimize requests, and map results back using the original coordinate or job indices.

Always check for unreachable results in table and unassigned jobs in optimize. Do not assume that every requested route or job is successfully served.
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
