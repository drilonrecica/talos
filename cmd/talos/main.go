// SPDX-License-Identifier: AGPL-3.0-only

// Command talos starts the TALOS monitoring service.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/drilonrecica/talos/internal/api"
	"github.com/drilonrecica/talos/internal/app"
	"github.com/drilonrecica/talos/internal/auth"
	"github.com/drilonrecica/talos/internal/demo"
	"github.com/drilonrecica/talos/internal/diagnostics"
	"github.com/drilonrecica/talos/internal/metrics"
	"github.com/drilonrecica/talos/internal/onboarding"
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
	config, effectiveSettings, err := settings.Load()
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
	application.Add(storage.NewPersistence(engine, store, config.Persistence.RawInterval, config.Persistence.QueueBatchLimit))
	var setup *auth.SetupService
	var credentials *auth.Credentials
	var sessions *auth.Sessions
	var onboardingService *onboarding.Service
	var settingsService *settings.Service
	if !*demoMode && !config.Demo {
		setup = auth.NewSetupService(nil)
		credentials = auth.NewCredentials(nil)
		sessions = auth.NewSessions(nil, auth.SessionConfig{IdleTimeout: config.Sessions.IdleTimeout, AbsoluteLifetime: config.Sessions.AbsoluteLifetime})
		checker := diagnostics.OnboardingChecker{HostProc: config.Paths.HostProc, HostSys: config.Paths.HostSys, DataDir: config.Paths.DataDir}
		onboardingService = onboarding.New(nil, checker)
		settingsService = settings.NewService(settings.NewStore(nil), config, effectiveSettings, func(updated settings.Config) {
			sessions.SetConfig(auth.SessionConfig{IdleTimeout: updated.Sessions.IdleTimeout, AbsoluteLifetime: updated.Sessions.AbsoluteLifetime})
		})
		application.Add(app.ComponentFuncs{StartFunc: func(ctx context.Context) error {
			setup.SetDB(store.DB())
			credentials.SetDB(store.DB())
			sessions.SetDB(store.DB())
			onboardingService.SetDB(store.DB())
			settingsService.SetDB(store.DB())
			if err := settingsService.Initialize(ctx); err != nil {
				return err
			}
			if _, err := auth.BootstrapAdmin(ctx, credentials, setup); err != nil {
				return err
			}
			setupToken, err := auth.SetupTokenFromEnvironment()
			if err != nil {
				return err
			}
			generated, err := setup.Initialize(ctx, config.HTTP.ListenAddress, setupToken)
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
	bundleService := diagnostics.NewBundleService(func(ctx context.Context) diagnostics.BundleData {
		schema, schemaErr := store.SchemaVersion(ctx)
		var databaseBytes int64
		stat, sizeErr := os.Stat(config.Paths.DatabasePath)
		if sizeErr == nil {
			databaseBytes = stat.Size()
		}
		fields := map[string]any{
			"version": version, "os": runtime.GOOS, "architecture": runtime.GOARCH,
			"schemaVersion": schema, "collectorHealth": engine.Snapshot().Collectors,
			"configuration": effectiveSettings, "resourceCount": len(engine.Snapshot().Resources),
			"databaseBytes": databaseBytes, "recentInternalErrors": []string{},
			"dockerVersion": nil, "selfMetrics": map[string]int64{},
		}
		failures := []string{}
		if schemaErr != nil {
			failures = append(failures, "database schema version unavailable")
		}
		if sizeErr != nil {
			failures = append(failures, "database size unavailable")
		}
		failures = append(failures, "Docker version unavailable")
		return diagnostics.BundleData{Fields: fields, PartialFailures: failures}
	})
	apiServer.EnableDiagnostics(bundleService, authorizer, protection)
	if setup != nil {
		apiServer.EnableSetup(setup, protection, sessions)
		apiServer.EnableAuth(credentials, sessions, protection)
		apiServer.EnableOnboarding(onboardingService, sessions, sessions)
		apiServer.EnableSettings(settingsService, sessions, sessions)
	}
	application.Add(app.NewHTTPServer(config.HTTP.ListenAddress, version, application, apiServer.Handler(), webembed.Handler()))
	if err := application.Run(ctx); err != nil {
		log.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}
