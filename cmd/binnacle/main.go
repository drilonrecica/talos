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
	"strconv"
	"syscall"
	"time"

	"github.com/drilonrecica/binnacle/internal/alerts"
	"github.com/drilonrecica/binnacle/internal/api"
	"github.com/drilonrecica/binnacle/internal/app"
	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/checks"
	dockercollector "github.com/drilonrecica/binnacle/internal/collector/docker"
	"github.com/drilonrecica/binnacle/internal/collector/production"
	"github.com/drilonrecica/binnacle/internal/coolify"
	"github.com/drilonrecica/binnacle/internal/demo"
	"github.com/drilonrecica/binnacle/internal/diagnostics"
	"github.com/drilonrecica/binnacle/internal/dockerapi"
	"github.com/drilonrecica/binnacle/internal/metrics"
	"github.com/drilonrecica/binnacle/internal/notifications"
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
	demoChecks := flag.Int("demo-checks", 10, "number of synthetic checks to generate in demo mode")
	healthcheck := flag.Bool("healthcheck", false, "perform a one-shot local health check and exit")
	showVersion := flag.Bool("version", false, "print the Binnacle version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
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
	tokenRepository := auth.NewAPITokenRepository(nil)
	checker := diagnostics.OnboardingChecker{HostProc: config.Paths.HostProc, HostSys: config.Paths.HostSys, DataDir: config.Paths.DataDir}
	onboardingService := onboarding.New(nil, checker)
	settingsService := settings.NewService(settings.NewStore(nil), config, effectiveSettings, func(updated settings.Config) {
		sessions.SetConfig(auth.SessionConfig{IdleTimeout: updated.Sessions.IdleTimeout, AbsoluteLifetime: updated.Sessions.AbsoluteLifetime})
		store.SetRetention(storage.RetentionCutoffs{Raw: updated.Retention.Raw, OneMinute: updated.Retention.OneMinute, FifteenMinute: updated.Retention.FifteenMinute, OneHour: updated.Retention.OneHour})
	})
	onboardingService.SetRetentionSettings(settingsService)
	checkRepository := checks.NewRepository(nil)
	alertRepository := alerts.NewRepository(nil)
	secretStore, err := auth.NewSecretStore(nil, config.Paths.MasterKey)
	if err != nil {
		log.Error("master encryption key is invalid", "error", err)
		os.Exit(1)
	}
	mfaService := auth.NewMFA(nil, credentials, secretStore, sessions)
	notificationRepository := notifications.NewRepository(nil, secretStore)
	coolifyIntegration := coolify.NewIntegration(secretStore, coolify.ClientConfig{BaseURL: config.Coolify.URL, Token: config.Coolify.APIToken, AllowInsecureHTTP: config.Coolify.AllowInsecureHTTP})
	notificationWorker := notifications.NewWorker(notificationRepository, notifications.Config{
		MaxConcurrency:   config.Notifications.MaxConcurrency,
		QueueCapacity:    config.Notifications.QueueCapacity,
		DeliveryTimeout:  config.Notifications.DeliveryTimeout,
		ReminderInterval: config.Notifications.ReminderInterval,
		AllowPrivate:     config.Notifications.AllowPrivateTargets,
	})
	alertRepository.SetIncidentSink(notificationRepository)
	allowPrivate, _ := strconv.ParseBool(os.Getenv("BINNACLE_CHECKS_ALLOW_PRIVATE_TARGETS"))
	var checkRunner interface {
		Run(context.Context, checks.Check) checks.Result
	} = &checks.Runner{AllowPrivate: allowPrivate}
	if *demoMode || config.Demo {
		checkRunner = demo.CheckRunner{}
	}
	checkScheduler := checks.NewScheduler(checkRepository, checkRunner, config.Checks.MaxConcurrency)
	alertEvaluator := alerts.NewEvaluator(alertRepository, engine)

	var dockerEngine dockerapi.Client
	var dockerLogs dockerapi.LogClient
	var productionSampler *production.Sampler
	if *demoMode || config.Demo {
		generator := demo.New(*demoSeed, realClock{})
		generator.Containers = *demoContainers
		application.Add(app.ComponentFuncs{StartFunc: func(ctx context.Context) error {
			if err := demo.SeedHistory(ctx, store, generator, time.Now()); err != nil {
				return fmt.Errorf("seed demo history: %w", err)
			}
			return nil
		}})
		application.Add(&demo.Component{Generator: generator, Engine: engine})
		log.Warn("demo mode enabled", "seed", *demoSeed, "containers", *demoContainers)
	} else {
		rawEngine, err := dockerapi.NewEngine(config.Docker.SocketPath)
		if err != nil {
			log.Error("Docker client configuration is invalid", "error", err)
			os.Exit(1)
		}
		limitedDocker := dockerapi.New(rawEngine, config.Docker.MaxConcurrency)
		dockerEngine, dockerLogs = limitedDocker, limitedDocker
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
		tokenRepository.SetDB(store.DB())
		onboardingService.SetDB(store.DB())
		settingsService.SetDB(store.DB())
		checkRepository.SetDB(store.DB())
		alertRepository.SetDB(store.DB())
		secretStore.SetDB(store.DB())
		mfaService.SetDB(store.DB())
		notificationRepository.SetDB(store.DB())
		coolifyIntegration.SetDB(store.DB())
		if err := alertRepository.SeedDefaults(ctx); err != nil {
			return err
		}
		if *demoMode || config.Demo {
			if err := demo.SeedChecksAlerts(ctx, store.DB(), *demoChecks, *demoContainers, time.Now().UTC()); err != nil {
				return err
			}
		}
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
	application.Add(coolifyIntegration)
	application.Add(sessions)
	if !*demoMode && !config.Demo {
		application.Add(checkScheduler)
	}
	application.Add(alertEvaluator)
	application.Add(notificationWorker)

	apiServer := api.New()
	proxies, _ := auth.ParseTrustedProxies(config.HTTP.TrustedProxyCIDRs)
	proxyAuth, err := auth.NewProxyAuthenticator(auth.ProxyAuthConfig{Mode: auth.AuthMode(config.Auth.Mode), IdentityHeader: config.Auth.IdentityHeader, AllowedSubject: config.Auth.AllowedSubject, ProxyCIDRs: config.Auth.ProxyCIDRs}, proxies)
	if err != nil {
		log.Error("proxy authentication configuration is invalid", "error", err)
		os.Exit(1)
	}
	protection := auth.NewProtection(4096, proxies)
	sessions.SetTrustedProxies(proxies)

	authorizer := sessions
	serverAuthorizer := api.ScopedAuthorizer{Sessions: sessions, Tokens: tokenRepository, Scope: auth.ScopeServerRead}
	resourceAuthorizer := api.ScopedAuthorizer{Sessions: sessions, Tokens: tokenRepository, Scope: auth.ScopeResourcesRead}
	metricsAuthorizer := api.ScopedAuthorizer{Sessions: sessions, Tokens: tokenRepository, Scope: auth.ScopeMetricsRead}
	eventsAuthorizer := api.ScopedAuthorizer{Sessions: sessions, Tokens: tokenRepository, Scope: auth.ScopeEventsRead}
	incidentsAuthorizer := api.ScopedAuthorizer{Sessions: sessions, Tokens: tokenRepository, Scope: auth.ScopeIncidentsRead, TokenPathPrefixes: []string{"/api/v1/incidents"}}
	apiServer.EnableLive(engine, authorizer, protection, alertRepository, coolifyIntegration)
	apiServer.EnableCurrent(engine, serverAuthorizer)
	apiServer.EnableResources(engine, resourceAuthorizer, store, protection, alertRepository, coolifyIntegration)
	apiServer.EnableMetrics(store, metricsAuthorizer, protection)
	apiServer.EnableEvents(store, eventsAuthorizer, protection)
	apiServer.EnableHistoryDeletion(store, authorizer, sessions)
	monitor := &diagnostics.Monitor{DatabasePath: config.Paths.DatabasePath, DatabaseTarget: config.Database.TargetBudgetBytes, QueueCapacity: config.Persistence.QueueBatchLimit, Engine: engine, Persistence: persistence, Collector: productionSampler, Notifications: notificationWorker, NotificationQueueCapacity: config.Notifications.QueueCapacity}
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
	apiServer.EnableAuth(credentials, sessions, protection, mfaService, proxyAuth)
	apiServer.EnableProxyAuth(proxyAuth, credentials, sessions)
	apiServer.EnableMFA(mfaService, credentials, sessions, protection)
	apiServer.EnableOnboarding(onboardingService, sessions, sessions)
	apiServer.EnableSettings(settingsService, sessions, sessions)
	apiServer.EnableChecks(checkRepository, checkScheduler, sessions, sessions, protection)
	apiServer.EnableAlerts(alertRepository, sessions, sessions, protection)
	apiServer.EnableIncidentsNotifications(notificationRepository, notificationWorker, incidentsAuthorizer, sessions, protection)
	apiServer.EnableAPITokens(tokenRepository, sessions)
	apiServer.EnableExports(store, notificationRepository, engine, metricsAuthorizer, eventsAuthorizer, incidentsAuthorizer, resourceAuthorizer, coolifyIntegration)
	apiServer.EnableCoolify(coolifyIntegration, sessions, sessions)
	logService, err := diagnostics.NewLogService(dockerLogs, config.Logs.MaxLines, config.Logs.MaxResponseBytes, config.Logs.RedactionPatterns)
	if err != nil {
		log.Error("log diagnostics configuration is invalid", "error", err)
		os.Exit(1)
	}
	apiServer.EnableLogs(logService, engine, sessions)
	apiServer.EnableProcesses(diagnostics.NewProcessScanner(config.Paths.HostProc, config.Paths.HostPasswd), sessions)
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
