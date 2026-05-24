package main

import (
	"context"
	"log/slog"
	"os"

	"kids-checkin/internal/cmd"
	"kids-checkin/internal/logger"
)

func main() {
	logger.Init(logger.LevelFromEnv())

	slog.Info("starting kids-checkin", slog.String("args", os.Args[0]))

	err := (cmd.NewCommand()).Run(context.Background(), os.Args)
	if err != nil {
		slog.Error("execution failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.Info("shutdown complete")
}
