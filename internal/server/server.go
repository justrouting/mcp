package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/justrouting/mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	name    = "justrouting"
	version = "0.2.0"
)

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
