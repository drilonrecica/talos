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
	"github.com/drilonrecica/talos/internal/auth"
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
	store := storage.New(config.Paths.DatabasePath, config.Paths.RuntimeDir)
	application.Add(store)
	var setup *auth.SetupService
	var credentials *auth.Credentials
	var sessions *auth.Sessions
	if !*demoMode && !config.Demo {
		setup = auth.NewSetupService(nil)
		credentials = auth.NewCredentials(nil)
		sessions = auth.NewSessions(nil, auth.SessionConfig{IdleTimeout: config.Sessions.IdleTimeout, AbsoluteLifetime: config.Sessions.AbsoluteLifetime})
		application.Add(app.ComponentFuncs{StartFunc: func(ctx context.Context) error {
			setup.SetDB(store.DB())
			credentials.SetDB(store.DB())
			sessions.SetDB(store.DB())
			generated, err := setup.Initialize(ctx, config.HTTP.ListenAddress, os.Getenv("TALOS_SETUP_TOKEN"))
			if generated != "" {
				log.Warn("local setup token generated", "setup_token", generated)
			}
			return err
		}})
		application.Add(sessions)
	}
	apiServer := api.New()
	proxies, _ := auth.ParseTrustedProxies(config.HTTP.TrustedProxyCIDRs)
	protection := auth.NewProtection(4096, proxies)
	if sessions != nil {
		sessions.SetTrustedProxies(proxies)
	}
	var authorizer api.Authorizer = api.DemoAuthorizer(*demoMode || config.Demo)
	if sessions != nil {
		authorizer = sessions
	}
	apiServer.EnableLive(engine, authorizer)
	apiServer.EnableCurrent(engine, authorizer)
	apiServer.EnableResources(engine, authorizer)
	apiServer.EnableMetrics(store, authorizer, protection)
	apiServer.EnableEvents(store, authorizer)
	apiServer.EnableHistoryDeletion(store, authorizer, sessions)
	if setup != nil {
		apiServer.EnableSetup(setup, protection)
		apiServer.EnableAuth(credentials, sessions, protection)
	}
	application.Add(app.NewHTTPServer(config.HTTP.ListenAddress, version, application, apiServer.Handler(), webembed.Handler()))
	if err := application.Run(ctx); err != nil {
		log.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}
