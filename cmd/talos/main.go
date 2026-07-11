// SPDX-License-Identifier: AGPL-3.0-only

// Command talos starts the TALOS monitoring service.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/drilonrecica/talos/internal/app"
)

var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application := app.New(log)
	application.Add(app.NewHTTPServer(":8080", version, application))
	if err := application.Run(ctx); err != nil {
		log.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}
