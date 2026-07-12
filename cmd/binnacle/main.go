// SPDX-License-Identifier: AGPL-3.0-only

// Command binnacle starts the Binnacle monitoring service.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/drilonrecica/binnacle/internal/api"
	"github.com/drilonrecica/binnacle/internal/app"
	"github.com/drilonrecica/binnacle/internal/auth"
	dockercollector "github.com/drilonrecica/binnacle/internal/collector/docker"
	"github.com/drilonrecica/binnacle/internal/collector/production"
	"github.com/drilonrecica/binnacle/internal/demo"
	"github.com/drilonrecica/binnacle/internal/diagnostics"
	"github.com/drilonrecica/binnacle/internal/dockerapi"
	"github.com/drilonrecica/binnacle/internal/metrics"
	"github.com/drilonrecica/binnacle/internal/onboarding"
	"github.com/drilonrecica/binnacle/internal/settings"
	"github.com/drilonrecica/binnacle/internal/storage"
	"github.com/drilonrecica/binnacle/internal/webembed"
)

var version = "dev"

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func main() {
	demoMode := flag.Bool("demo", false, "run with deterministic synthetic monitoring data")
	demoSeed := flag.Uint64("demo-seed", 1, "seed for synthetic demo data")
	demoContainers := flag.Int("demo-containers", 30, "number of synthetic containers to generate in demo mode")
	healthcheck := flag.Bool("healthcheck", false, "perform a one-shot local health check and exit")
	flag.Parse()
	if *healthcheck {
		if err := runHealthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
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

	store := storage.New(config.Paths.DatabasePath, config.Paths.RuntimeDir)
	store.SetBudgetTarget(config.Database.TargetBudgetBytes)
	application.Add(store)
	persistence := storage.NewPersistence(engine, store, config.Persistence.RawInterval, config.Persistence.QueueBatchLimit)
	application.Add(persistence)

	// Auth, onboarding, and settings are always available, even in demo mode.
	// Demo mode only replaces real host/Docker collection with synthetic data;
	// it does not disable authentication.
	setup := auth.NewSetupService(nil)
	credentials := auth.NewCredentials(nil)
	sessions := auth.NewSessions(nil, auth.SessionConfig{IdleTimeout: config.Sessions.IdleTimeout, AbsoluteLifetime: config.Sessions.AbsoluteLifetime})
	checker := diagnostics.OnboardingChecker{HostProc: config.Paths.HostProc, HostSys: config.Paths.HostSys, DataDir: config.Paths.DataDir}
	onboardingService := onboarding.New(nil, checker)
	settingsService := settings.NewService(settings.NewStore(nil), config, effectiveSettings, func(updated settings.Config) {
		sessions.SetConfig(auth.SessionConfig{IdleTimeout: updated.Sessions.IdleTimeout, AbsoluteLifetime: updated.Sessions.AbsoluteLifetime})
	})

	var dockerEngine dockerapi.Client
	var productionSampler *production.Sampler
	if *demoMode || config.Demo {
		generator := demo.New(*demoSeed, realClock{})
		generator.Containers = *demoContainers
		application.Add(&demo.Component{Generator: generator, Engine: engine})
		log.Warn("demo mode enabled", "seed", *demoSeed, "containers", *demoContainers)
	} else {
		rawEngine, err := dockerapi.NewEngine(config.Docker.SocketPath)
		if err != nil {
			log.Error("Docker client configuration is invalid", "error", err)
			os.Exit(1)
		}
		dockerEngine = dockerapi.New(rawEngine, config.Docker.MaxConcurrency)
		onboardingService.SetDocker(dockerEngine)
		cache := dockercollector.NewCache()
		productionSampler = &production.Sampler{Engine: engine, Docker: dockerEngine, Cache: cache, Store: store, HostProc: config.Paths.HostProc, DataDir: config.Paths.DataDir, MaxDockerConcurrency: config.Docker.MaxConcurrency, Interval: func() time.Duration {
			return settingsService.Current().Collection.HostInterval
		}}
		application.Add(productionSampler)
	}

	application.Add(app.ComponentFuncs{StartFunc: func(ctx context.Context) error {
		setup.SetDB(store.DB())
		credentials.SetDB(store.DB())
		sessions.SetDB(store.DB())
		onboardingService.SetDB(store.DB())
		settingsService.SetDB(store.DB())
		if err := settingsService.Initialize(ctx); err != nil {
			return err
		}
		cfg := settingsService.Current()
		store.SetRetention(storage.RetentionCutoffs{Raw: cfg.Retention.Raw, OneMinute: cfg.Retention.OneMinute, FifteenMinute: cfg.Retention.FifteenMinute, OneHour: cfg.Retention.OneHour})
		if _, err := auth.BootstrapAdmin(ctx, credentials, setup); err != nil {
			return err
		}
		setupToken, err := auth.SetupTokenFromEnvironment()
		if err != nil {
			return err
		}
		generated, err := setup.Initialize(ctx, config.HTTP.ListenAddress, setupToken)
		if generated != "" {
			log.Warn("local setup token generated for loopback installation; retrieve it from the setup UI")
		}
		return err
	}})
	application.Add(sessions)

	apiServer := api.New()
	proxies, _ := auth.ParseTrustedProxies(config.HTTP.TrustedProxyCIDRs)
	protection := auth.NewProtection(4096, proxies)
	sessions.SetTrustedProxies(proxies)

	authorizer := sessions
	apiServer.EnableLive(engine, authorizer, protection)
	apiServer.EnableCurrent(engine, authorizer)
	apiServer.EnableResources(engine, authorizer, store, protection)
	apiServer.EnableMetrics(store, authorizer, protection)
	apiServer.EnableEvents(store, authorizer, protection)
	apiServer.EnableHistoryDeletion(store, authorizer, sessions)
	monitor := &diagnostics.Monitor{DatabasePath: config.Paths.DatabasePath, DatabaseTarget: config.Database.TargetBudgetBytes, QueueCapacity: config.Persistence.QueueBatchLimit, Engine: engine, Persistence: persistence, Collector: productionSampler}
	apiServer.EnableMonitorHealth(monitor, authorizer)
	bundleService := diagnostics.NewBundleService(func(ctx context.Context) diagnostics.BundleData {
		schema, schemaErr := store.SchemaVersion(ctx)
		var databaseBytes int64
		stat, sizeErr := os.Stat(config.Paths.DatabasePath)
		if sizeErr == nil {
			databaseBytes = stat.Size()
		}
		var dockerVersion any
		failures := []string{}
		if dockerEngine != nil {
			value, versionErr := dockerEngine.Version(ctx)
			if versionErr == nil {
				dockerVersion = value.APIVersion
			} else {
				failures = append(failures, "Docker version unavailable")
			}
		} else {
			failures = append(failures, "Docker version unavailable")
		}
		fields := map[string]any{
			"version": version, "os": runtime.GOOS, "architecture": runtime.GOARCH,
			"schemaVersion": schema, "collectorHealth": engine.Snapshot().Collectors,
			"configuration": effectiveSettings, "resourceCount": len(engine.Snapshot().Resources),
			"databaseBytes": databaseBytes, "recentInternalErrors": []string{},
			"dockerVersion": dockerVersion, "selfMetrics": monitor.Snapshot(),
		}
		if schemaErr != nil {
			failures = append(failures, "database schema version unavailable")
		}
		if sizeErr != nil {
			failures = append(failures, "database size unavailable")
		}
		return diagnostics.BundleData{Fields: fields, PartialFailures: failures}
	})
	apiServer.EnableDiagnostics(bundleService, authorizer, protection)
	apiServer.EnableSetup(setup, protection, sessions)
	apiServer.EnableAuth(credentials, sessions, protection)
	apiServer.EnableOnboarding(onboardingService, sessions, sessions)
	apiServer.EnableSettings(settingsService, sessions, sessions)
	application.Add(app.NewHTTPServer(config.HTTP.ListenAddress, version, application, apiServer.Handler(), webembed.Handler()))
	if err := application.Run(ctx); err != nil {
		log.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func runHealthcheck() error {
	resp, err := http.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %s", resp.Status)
	}
	return nil
}
