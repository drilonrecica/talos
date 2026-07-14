// SPDX-License-Identifier: AGPL-3.0-only

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Source string

const (
	SourceDefault     Source = "Default"
	SourceFile        Source = "Config file"
	SourceEnvironment Source = "Environment"
	SourceAdmin       Source = "Admin override"
)

type Effective struct {
	Value           string `json:"value"`
	Source          Source `json:"source"`
	Secret          bool   `json:"secret"`
	RestartRequired bool   `json:"restart_required"`
}
type OverrideProvider interface {
	Overrides() (map[string]string, error)
}
type NoopOverrideProvider struct{}

func (NoopOverrideProvider) Overrides() (map[string]string, error) { return nil, nil }

// Discover returns the selected optional configuration file.
func Discover(getenv func(string) string, exists func(string) bool) string {
	if p := getenv("BINNACLE_CONFIG_FILE"); p != "" {
		return p
	}
	for _, p := range []string{"/etc/binnacle/binnacle.toml", "/var/lib/binnacle/binnacle.toml"} {
		if exists(p) {
			return p
		}
	}
	return ""
}
func Load() (Config, map[string]Effective, error) {
	return LoadWith(os.Getenv, func(p string) bool { _, e := os.Stat(p); return e == nil }, NoopOverrideProvider{})
}
func LoadWith(getenv func(string) string, exists func(string) bool, provider OverrideProvider) (Config, map[string]Effective, error) {
	c := Defaults()
	sources := map[string]Source{}
	path := Discover(getenv, exists)
	if path != "" {
		values, err := readTOML(path)
		if err != nil {
			return Config{}, nil, err
		}
		if err = apply(&c, values); err != nil {
			return Config{}, nil, fmt.Errorf("config file %s: %w", path, err)
		}
		for k := range values {
			sources[k] = SourceFile
		}
	}
	for env, key := range environment {
		if value := getenv(env); value != "" {
			if err := apply(&c, map[string]string{key: value}); err != nil {
				return Config{}, nil, fmt.Errorf("%s: %w", env, err)
			}
			sources[key] = SourceEnvironment
		}
	}
	if value := getenv("BINNACLE_HOST_PASSWD"); value != "" {
		if err := apply(&c, map[string]string{"paths.host_passwd": value}); err != nil {
			return Config{}, nil, fmt.Errorf("BINNACLE_HOST_PASSWD: %w", err)
		}
		sources["paths.host_passwd"] = SourceEnvironment
	}
	for env, key := range map[string]string{"BINNACLE_COOLIFY_URL": "coolify.url", "BINNACLE_COOLIFY_API_TOKEN_FILE": "coolify.api_token_file", "BINNACLE_COOLIFY_ALLOW_INSECURE_HTTP": "coolify.allow_insecure_http"} {
		if value := getenv(env); value != "" {
			if err := apply(&c, map[string]string{key: value}); err != nil {
				return Config{}, nil, fmt.Errorf("%s: %w", env, err)
			}
			sources[key] = SourceEnvironment
		}
	}
	if token := strings.TrimSpace(getenv("BINNACLE_COOLIFY_API_TOKEN")); token != "" {
		c.Coolify.APIToken = token
		sources["coolify.api_token"] = SourceEnvironment
	}
	if c.Coolify.APIToken == "" && c.Coolify.APITokenFile != "" {
		raw, err := os.ReadFile(c.Coolify.APITokenFile)
		if err != nil {
			return Config{}, nil, fmt.Errorf("read Coolify token file: %w", err)
		}
		if len(raw) > 4096 {
			return Config{}, nil, fmt.Errorf("Coolify token file is too large")
		}
		c.Coolify.APIToken = strings.TrimSpace(string(raw))
	}
	if provider != nil {
		overrides, err := provider.Overrides()
		if err != nil {
			return Config{}, nil, err
		}
		for key, value := range overrides {
			if !UIOverridable(key) {
				return Config{}, nil, fmt.Errorf("UI override for %s is not allowed", key)
			}
			if err := apply(&c, map[string]string{key: value}); err != nil {
				return Config{}, nil, err
			}
			sources[key] = SourceAdmin
		}
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		return Config{}, nil, err
	}
	return c, effective(c, sources), nil
}

// ResolveOverrides validates persisted UI values against an already resolved
// deployment configuration. It is also used by the runtime settings service.
func ResolveOverrides(base Config, values map[string]string) (Config, error) {
	resolved := base
	if presetName, ok := values["retention.preset"]; ok && presetName != "advanced" {
		preset, found := RetentionPreset(strings.ToLower(presetName))
		if !found {
			return Config{}, fmt.Errorf("unknown retention preset %s", presetName)
		}
		resolved.Retention = preset
	}
	if err := apply(&resolved, values); err != nil {
		return Config{}, err
	}
	resolved.Normalize()
	if err := resolved.Validate(); err != nil {
		return Config{}, err
	}
	return resolved, nil
}
func readTOML(path string) (map[string]string, error) {
	var raw map[string]any
	meta, err := toml.DecodeFile(path, &raw)
	if err != nil {
		return nil, err
	}
	if len(meta.Undecoded()) > 0 {
		return nil, fmt.Errorf("unknown key %s in %s", meta.Undecoded()[0], path)
	}
	out := map[string]string{}
	var flatten func(string, map[string]any)
	flatten = func(prefix string, m map[string]any) {
		for k, v := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			if child, ok := v.(map[string]any); ok {
				flatten(key, child)
			} else {
				out[key] = fmt.Sprint(v)
			}
		}
	}
	flatten("", raw)
	for k := range out {
		if _, ok := supported[k]; !ok {
			return nil, fmt.Errorf("unknown key %s in %s", k, path)
		}
	}
	return out, nil
}

var environment = map[string]string{
	"BINNACLE_DATA_DIR": "paths.data_dir", "BINNACLE_DATABASE_PATH": "paths.database_path", "BINNACLE_RUNTIME_DIR": "paths.runtime_dir", "BINNACLE_HOST_PROC": "paths.host_proc", "BINNACLE_HOST_SYS": "paths.host_sys", "BINNACLE_MASTER_KEY": "paths.master_key", "BINNACLE_LISTEN_ADDRESS": "http.listen_address", "BINNACLE_TRUSTED_PROXY_CIDRS": "http.trusted_proxy_cidrs", "BINNACLE_HOST_INTERVAL": "collection.host_interval", "BINNACLE_CONTAINER_INTERVAL": "collection.container_interval", "BINNACLE_MINIMUM_INTERVAL": "collection.minimum_interval", "BINNACLE_SSE_INTERVAL": "live.sse_interval", "BINNACLE_RAW_INTERVAL": "persistence.raw_interval", "BINNACLE_QUEUE_BATCH_LIMIT": "persistence.queue_batch_limit", "BINNACLE_RETENTION_PRESET": "retention.preset", "BINNACLE_RETENTION_RAW": "retention.raw", "BINNACLE_RETENTION_ONE_MINUTE": "retention.one_minute", "BINNACLE_RETENTION_FIFTEEN_MINUTE": "retention.fifteen_minute", "BINNACLE_RETENTION_ONE_HOUR": "retention.one_hour", "BINNACLE_DATABASE_TARGET_BUDGET_BYTES": "database.target_budget_bytes", "BINNACLE_DATABASE_WARNING_RATIO": "database.warning_ratio", "BINNACLE_DATABASE_CRITICAL_RATIO": "database.critical_ratio", "BINNACLE_DATABASE_EMERGENCY_PAUSE_RATIO": "database.emergency_pause_ratio", "BINNACLE_CHARTS_MAX_POINTS": "charts.max_points_per_series", "BINNACLE_DOCKER_SOCKET": "docker.socket_path", "BINNACLE_DOCKER_MAX_CONCURRENCY": "docker.max_concurrency", "BINNACLE_CHECKS_MAX_CONCURRENCY": "checks.max_concurrency", "BINNACLE_NOTIFICATIONS_ALLOW_PRIVATE_TARGETS": "notifications.allow_private_targets", "BINNACLE_NOTIFICATIONS_MAX_CONCURRENCY": "notifications.max_concurrency", "BINNACLE_NOTIFICATIONS_QUEUE_CAPACITY": "notifications.queue_capacity", "BINNACLE_NOTIFICATIONS_DELIVERY_TIMEOUT": "notifications.delivery_timeout", "BINNACLE_NOTIFICATIONS_REMINDER_INTERVAL": "notifications.reminder_interval", "BINNACLE_LOGS_MAX_RESPONSE_BYTES": "logs.max_response_bytes", "BINNACLE_LOGS_MAX_LINES": "logs.max_lines", "BINNACLE_LOGS_REDACTION_PATTERNS": "logs.redaction_patterns", "BINNACLE_SESSION_IDLE_TIMEOUT": "sessions.idle_timeout", "BINNACLE_SESSION_ABSOLUTE_LIFETIME": "sessions.absolute_lifetime", "BINNACLE_DEMO": "demo"}
var supported = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range environment {
		m[k] = true
	}
	m["paths.host_passwd"] = true
	for _, key := range []string{"coolify.url", "coolify.api_token", "coolify.api_token_file", "coolify.allow_insecure_http"} {
		m[key] = true
	}
	return m
}()

func apply(c *Config, values map[string]string) error {
	if value, ok := values["retention.preset"]; ok {
		c.Retention.Preset = strings.ToLower(value)
	}
	for key, value := range values {
		var err error
		d := func(dst *time.Duration) {
			var v time.Duration
			v, err = time.ParseDuration(value)
			if err == nil {
				*dst = v
			}
		}
		i := func(dst *int) {
			var v int
			v, err = strconv.Atoi(value)
			if err == nil {
				*dst = v
			}
		}
		i64 := func(dst *int64) {
			var v int64
			v, err = strconv.ParseInt(value, 10, 64)
			if err == nil {
				*dst = v
			}
		}
		f := func(dst *float64) {
			var v float64
			v, err = strconv.ParseFloat(value, 64)
			if err == nil {
				*dst = v
			}
		}
		switch key {
		case "paths.data_dir":
			c.Paths.DataDir = value
		case "paths.database_path":
			c.Paths.DatabasePath = value
		case "paths.runtime_dir":
			c.Paths.RuntimeDir = value
		case "paths.host_proc":
			c.Paths.HostProc = value
		case "paths.host_sys":
			c.Paths.HostSys = value
		case "paths.host_passwd":
			c.Paths.HostPasswd = value
		case "paths.master_key":
			c.Paths.MasterKey = value
		case "coolify.url":
			c.Coolify.URL = strings.TrimRight(value, "/")
		case "coolify.api_token_file":
			c.Coolify.APITokenFile = value
		case "coolify.allow_insecure_http":
			c.Coolify.AllowInsecureHTTP, err = strconv.ParseBool(value)
		case "http.listen_address":
			c.HTTP.ListenAddress = value
		case "http.trusted_proxy_cidrs":
			c.HTTP.TrustedProxyCIDRs = nil
			for _, cidr := range strings.Split(value, ",") {
				if cidr = strings.TrimSpace(cidr); cidr != "" {
					c.HTTP.TrustedProxyCIDRs = append(c.HTTP.TrustedProxyCIDRs, cidr)
				}
			}
		case "collection.host_interval":
			d(&c.Collection.HostInterval)
		case "collection.container_interval":
			d(&c.Collection.ContainerInterval)
		case "collection.minimum_interval":
			d(&c.Collection.MinimumInterval)
		case "live.sse_interval":
			d(&c.Live.SSEInterval)
		case "persistence.raw_interval":
			d(&c.Persistence.RawInterval)
		case "persistence.queue_batch_limit":
			i(&c.Persistence.QueueBatchLimit)
		case "retention.preset":
			// Applied first so tier overrides are deterministic.
		case "retention.raw":
			d(&c.Retention.Raw)
		case "retention.one_minute":
			d(&c.Retention.OneMinute)
		case "retention.fifteen_minute":
			d(&c.Retention.FifteenMinute)
		case "retention.one_hour":
			d(&c.Retention.OneHour)
		case "database.target_budget_bytes":
			i64(&c.Database.TargetBudgetBytes)
		case "database.warning_ratio":
			f(&c.Database.WarningRatio)
		case "database.critical_ratio":
			f(&c.Database.CriticalRatio)
		case "database.emergency_pause_ratio":
			f(&c.Database.EmergencyPauseRatio)
		case "charts.max_points_per_series":
			i(&c.Charts.MaxPointsPerSeries)
		case "docker.socket_path":
			c.Docker.SocketPath = value
		case "docker.max_concurrency":
			i(&c.Docker.MaxConcurrency)
		case "checks.max_concurrency":
			i(&c.Checks.MaxConcurrency)
		case "notifications.allow_private_targets":
			var v bool
			v, err = strconv.ParseBool(value)
			if err == nil {
				c.Notifications.AllowPrivateTargets = v
			}
		case "notifications.max_concurrency":
			i(&c.Notifications.MaxConcurrency)
		case "notifications.queue_capacity":
			i(&c.Notifications.QueueCapacity)
		case "notifications.delivery_timeout":
			d(&c.Notifications.DeliveryTimeout)
		case "notifications.reminder_interval":
			d(&c.Notifications.ReminderInterval)
		case "logs.max_response_bytes":
			i64(&c.Logs.MaxResponseBytes)
		case "logs.max_lines":
			i(&c.Logs.MaxLines)
		case "logs.redaction_patterns":
			c.Logs.RedactionPatterns = nil
			for _, pattern := range strings.Split(value, "||") {
				if pattern = strings.TrimSpace(pattern); pattern != "" {
					c.Logs.RedactionPatterns = append(c.Logs.RedactionPatterns, pattern)
				}
			}
		case "sessions.idle_timeout":
			d(&c.Sessions.IdleTimeout)
		case "sessions.absolute_lifetime":
			d(&c.Sessions.AbsoluteLifetime)
		case "demo":
			var v bool
			v, err = strconv.ParseBool(value)
			if err == nil {
				c.Demo = v
			}
		default:
			return fmt.Errorf("unknown key %s", key)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}
func effective(c Config, sources map[string]Source) map[string]Effective {
	result := map[string]Effective{}
	for key := range supported {
		source := sources[key]
		if source == "" {
			source = SourceDefault
		}
		value := "configured"
		secret := key == "paths.master_key" || key == "coolify.api_token"
		if !secret {
			value = lookup(c, key)
		}
		result[key] = Effective{Value: value, Source: source, Secret: secret, RestartRequired: !UIOverridable(key)}
	}
	return result
}
func lookup(c Config, key string) string {
	values := map[string]string{
		"paths.data_dir": c.Paths.DataDir, "paths.database_path": c.Paths.DatabasePath, "paths.runtime_dir": c.Paths.RuntimeDir,
		"paths.host_proc": c.Paths.HostProc, "paths.host_sys": c.Paths.HostSys, "http.listen_address": c.HTTP.ListenAddress,
		"paths.host_passwd": c.Paths.HostPasswd, "coolify.url": c.Coolify.URL, "coolify.api_token_file": c.Coolify.APITokenFile, "coolify.allow_insecure_http": strconv.FormatBool(c.Coolify.AllowInsecureHTTP),
		"http.trusted_proxy_cidrs": strings.Join(c.HTTP.TrustedProxyCIDRs, ","), "docker.socket_path": c.Docker.SocketPath,
		"collection.host_interval": c.Collection.HostInterval.String(), "collection.container_interval": c.Collection.ContainerInterval.String(),
		"collection.minimum_interval": c.Collection.MinimumInterval.String(), "live.sse_interval": c.Live.SSEInterval.String(),
		"persistence.raw_interval": c.Persistence.RawInterval.String(), "persistence.queue_batch_limit": strconv.Itoa(c.Persistence.QueueBatchLimit),
		"retention.preset": c.Retention.Preset, "retention.raw": c.Retention.Raw.String(), "retention.one_minute": c.Retention.OneMinute.String(),
		"retention.fifteen_minute": c.Retention.FifteenMinute.String(), "retention.one_hour": c.Retention.OneHour.String(),
		"database.target_budget_bytes": strconv.FormatInt(c.Database.TargetBudgetBytes, 10), "database.warning_ratio": strconv.FormatFloat(c.Database.WarningRatio, 'f', -1, 64),
		"database.critical_ratio": strconv.FormatFloat(c.Database.CriticalRatio, 'f', -1, 64), "database.emergency_pause_ratio": strconv.FormatFloat(c.Database.EmergencyPauseRatio, 'f', -1, 64),
		"charts.max_points_per_series": strconv.Itoa(c.Charts.MaxPointsPerSeries), "docker.max_concurrency": strconv.Itoa(c.Docker.MaxConcurrency),
		"checks.max_concurrency": strconv.Itoa(c.Checks.MaxConcurrency), "logs.max_response_bytes": strconv.FormatInt(c.Logs.MaxResponseBytes, 10),
		"notifications.allow_private_targets": strconv.FormatBool(c.Notifications.AllowPrivateTargets), "notifications.max_concurrency": strconv.Itoa(c.Notifications.MaxConcurrency), "notifications.queue_capacity": strconv.Itoa(c.Notifications.QueueCapacity), "notifications.delivery_timeout": c.Notifications.DeliveryTimeout.String(), "notifications.reminder_interval": c.Notifications.ReminderInterval.String(),
		"logs.max_lines": strconv.Itoa(c.Logs.MaxLines), "sessions.idle_timeout": c.Sessions.IdleTimeout.String(),
		"sessions.absolute_lifetime": c.Sessions.AbsoluteLifetime.String(), "demo": strconv.FormatBool(c.Demo),
	}
	if v, ok := values[key]; ok {
		return v
	}
	return "configured"
}
func ConfigPath(path string) string { return filepath.Clean(path) }
