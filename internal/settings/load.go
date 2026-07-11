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
	if p := getenv("TALOS_CONFIG_FILE"); p != "" {
		return p
	}
	for _, p := range []string{"/etc/talos/talos.toml", "/var/lib/talos/talos.toml"} {
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
	"TALOS_DATA_DIR": "paths.data_dir", "TALOS_DATABASE_PATH": "paths.database_path", "TALOS_RUNTIME_DIR": "paths.runtime_dir", "TALOS_HOST_PROC": "paths.host_proc", "TALOS_HOST_SYS": "paths.host_sys", "TALOS_MASTER_KEY": "paths.master_key", "TALOS_LISTEN_ADDRESS": "http.listen_address", "TALOS_HOST_INTERVAL": "collection.host_interval", "TALOS_CONTAINER_INTERVAL": "collection.container_interval", "TALOS_MINIMUM_INTERVAL": "collection.minimum_interval", "TALOS_SSE_INTERVAL": "live.sse_interval", "TALOS_RAW_INTERVAL": "persistence.raw_interval", "TALOS_QUEUE_BATCH_LIMIT": "persistence.queue_batch_limit", "TALOS_RETENTION_PRESET": "retention.preset", "TALOS_RETENTION_RAW": "retention.raw", "TALOS_RETENTION_ONE_MINUTE": "retention.one_minute", "TALOS_RETENTION_FIFTEEN_MINUTE": "retention.fifteen_minute", "TALOS_RETENTION_ONE_HOUR": "retention.one_hour", "TALOS_DATABASE_TARGET_BUDGET_BYTES": "database.target_budget_bytes", "TALOS_DATABASE_WARNING_RATIO": "database.warning_ratio", "TALOS_DATABASE_CRITICAL_RATIO": "database.critical_ratio", "TALOS_DATABASE_EMERGENCY_PAUSE_RATIO": "database.emergency_pause_ratio", "TALOS_CHARTS_MAX_POINTS": "charts.max_points_per_series", "TALOS_DOCKER_SOCKET": "docker.socket_path", "TALOS_DOCKER_MAX_CONCURRENCY": "docker.max_concurrency", "TALOS_CHECKS_MAX_CONCURRENCY": "checks.max_concurrency", "TALOS_LOGS_MAX_RESPONSE_BYTES": "logs.max_response_bytes", "TALOS_LOGS_MAX_LINES": "logs.max_lines", "TALOS_SESSION_IDLE_TIMEOUT": "sessions.idle_timeout", "TALOS_SESSION_ABSOLUTE_LIFETIME": "sessions.absolute_lifetime", "TALOS_DEMO": "demo"}
var supported = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range environment {
		m[k] = true
	}
	return m
}()

func apply(c *Config, values map[string]string) error {
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
		case "paths.master_key":
			c.Paths.MasterKey = value
		case "http.listen_address":
			c.HTTP.ListenAddress = value
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
			c.Retention.Preset = strings.ToLower(value)
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
		case "logs.max_response_bytes":
			i64(&c.Logs.MaxResponseBytes)
		case "logs.max_lines":
			i(&c.Logs.MaxLines)
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
		secret := key == "paths.master_key"
		if !secret {
			value = lookup(c, key)
		}
		result[key] = Effective{Value: value, Source: source, Secret: secret, RestartRequired: !UIOverridable(key)}
	}
	return result
}
func lookup(c Config, key string) string {
	values := map[string]string{"paths.data_dir": c.Paths.DataDir, "paths.database_path": c.Paths.DatabasePath, "paths.runtime_dir": c.Paths.RuntimeDir, "paths.host_proc": c.Paths.HostProc, "paths.host_sys": c.Paths.HostSys, "http.listen_address": c.HTTP.ListenAddress, "docker.socket_path": c.Docker.SocketPath}
	if v, ok := values[key]; ok {
		return v
	}
	return "configured"
}
func ConfigPath(path string) string { return filepath.Clean(path) }
