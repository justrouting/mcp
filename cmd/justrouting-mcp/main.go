package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/justrouting/mcp/internal/server"
)

func main() {
	logger := slog.New(
		slog.NewTextHandler(os.Stderr, nil),
	)

	srv, err := server.New(server.Config{
		APIKey: os.Getenv("JUSTROUTING_API_KEY"),
		Logger: logger,
	})
	if err != nil {
		logger.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	if err := srv.Run(context.Background()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
