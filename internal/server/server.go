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
JustRouting provides route calculations across Southeast Asia.

Use the route tool to calculate distance and estimated travel duration
between two locations. The default routing profile is driving; when the
user's request mentions a motorcycle or motorbike, pass "motorcycle" as
the route tool's profile input.

The route tool takes coordinates as longitude,latitude. When the user
asks about places by name or address, call the geocode tool first to
look up each place, then pass the returned coordinates to the route tool.

The table tool calculates a matrix of driving distances and durations
between many coordinates at once. When comparing several places (for
example, finding the nearest of several drivers), call the geocode tool
first for every place, pass the coordinates to the table tool as a single
ordered list, and read the returned matrix by list position.
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
