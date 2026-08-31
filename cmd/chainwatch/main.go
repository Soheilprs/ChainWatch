package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/soheilprs/chainwatch/internal/app"
	"github.com/soheilprs/chainwatch/internal/config"
)

func main() {
	logger := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelInfo},
		),
	)
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("ChainWatch stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	runtimeConfig, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	return app.Run(ctx, runtimeConfig, logger)
}
