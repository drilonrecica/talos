// SPDX-License-Identifier: AGPL-3.0-only

// Command talos starts the TALOS monitoring service.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/drilonrecica/talos/internal/api"
	"github.com/drilonrecica/talos/internal/app"
	"github.com/drilonrecica/talos/internal/settings"
	"github.com/drilonrecica/talos/internal/storage"
	"github.com/drilonrecica/talos/internal/webembed"
)

var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	config, _, err := settings.Load()
	if err != nil {
		log.Error("configuration is invalid", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application := app.New(log)
	application.Add(storage.New(config.Paths.DatabasePath, config.Paths.RuntimeDir))
	application.Add(app.NewHTTPServer(config.HTTP.ListenAddress, version, application, api.New().Handler(), webembed.Handler()))
	if err := application.Run(ctx); err != nil {
		log.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}
