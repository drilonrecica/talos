// SPDX-License-Identifier: AGPL-3.0-only

// Command talos starts the TALOS monitoring service.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/drilonrecica/talos/internal/api"
	"github.com/drilonrecica/talos/internal/app"
	"github.com/drilonrecica/talos/internal/demo"
	"github.com/drilonrecica/talos/internal/metrics"
	"github.com/drilonrecica/talos/internal/settings"
	"github.com/drilonrecica/talos/internal/storage"
	"github.com/drilonrecica/talos/internal/webembed"
)

var version = "dev"

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func main() {
	demoMode := flag.Bool("demo", false, "run with deterministic synthetic monitoring data")
	demoSeed := flag.Uint64("demo-seed", 1, "seed for synthetic demo data")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	config, _, err := settings.Load()
	if err != nil {
		log.Error("configuration is invalid", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application := app.New(log)
	engine := metrics.NewEngine(128)
	application.Add(engine)
	if *demoMode || config.Demo {
		application.Add(&demo.Component{Generator: demo.New(*demoSeed, realClock{}), Engine: engine})
	}
	application.Add(storage.New(config.Paths.DatabasePath, config.Paths.RuntimeDir))
	application.Add(app.NewHTTPServer(config.HTTP.ListenAddress, version, application, api.New().Handler(), webembed.Handler()))
	if err := application.Run(ctx); err != nil {
		log.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}
