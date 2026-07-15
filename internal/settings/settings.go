// SPDX-License-Identifier: AGPL-3.0-only

package settings

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Paths         Paths         `toml:"paths"`
	HTTP          HTTP          `toml:"http"`
	Collection    Collection    `toml:"collection"`
	Live          Live          `toml:"live"`
	Persistence   Persistence   `toml:"persistence"`
	Retention     Retention     `toml:"retention"`
	Database      Database      `toml:"database"`
	Charts        Charts        `toml:"charts"`
	Docker        Docker        `toml:"docker"`
	Checks        Checks        `toml:"checks"`
	Notifications Notifications `toml:"notifications"`
	Logs          Logs          `toml:"logs"`
	Coolify       Coolify       `toml:"coolify"`
	Auth          Auth          `toml:"auth"`
	Features      Features      `toml:"features"`
	Prometheus    Prometheus    `toml:"prometheus"`
	Sessions      Sessions      `toml:"sessions"`
	Demo          bool          `toml:"demo"`
}
type Paths struct {
	DataDir                 string `toml:"data_dir"`
	DatabasePath            string `toml:"database_path"`
	RuntimeDir              string `toml:"runtime_dir"`
	HostProc                string `toml:"host_proc"`
	HostSys                 string `toml:"host_sys"`
	HostPasswd              string `toml:"host_passwd"`
	MasterKey               string `toml:"master_key"`
	MasterKeyFile           string `toml:"master_key_file"`
	masterKeyLoadedFromFile bool
}
type HTTP struct {
	ListenAddress     string   `toml:"listen_address"`
	TrustedProxyCIDRs []string `toml:"trusted_proxy_cidrs"`
}
type Collection struct {
	HostInterval      time.Duration `toml:"host_interval"`
	ContainerInterval time.Duration `toml:"container_interval"`
	MinimumInterval   time.Duration `toml:"minimum_interval"`
}
type Live struct {
	SSEInterval time.Duration `toml:"sse_interval"`
}
type Persistence struct {
	RawInterval     time.Duration `toml:"raw_interval"`
	QueueBatchLimit int           `toml:"queue_batch_limit"`
}
type Retention struct {
	Preset        string        `toml:"preset"`
	Raw           time.Duration `toml:"raw"`
	OneMinute     time.Duration `toml:"one_minute"`
	FifteenMinute time.Duration `toml:"fifteen_minute"`
	OneHour       time.Duration `toml:"one_hour"`
}
type Database struct {
	TargetBudgetBytes   int64   `toml:"target_budget_bytes"`
	WarningRatio        float64 `toml:"warning_ratio"`
	CriticalRatio       float64 `toml:"critical_ratio"`
	EmergencyPauseRatio float64 `toml:"emergency_pause_ratio"`
}
type Charts struct {
	MaxPointsPerSeries int `toml:"max_points_per_series"`
}
type Docker struct {
	SocketPath     string `toml:"socket_path"`
	MaxConcurrency int    `toml:"max_concurrency"`
}
type Checks struct {
	MaxConcurrency int `toml:"max_concurrency"`
}
type Notifications struct {
	AllowPrivateTargets bool          `toml:"allow_private_targets"`
	MaxConcurrency      int           `toml:"max_concurrency"`
	QueueCapacity       int           `toml:"queue_capacity"`
	DeliveryTimeout     time.Duration `toml:"delivery_timeout"`
	ReminderInterval    time.Duration `toml:"reminder_interval"`
}
type Logs struct {
	MaxResponseBytes  int64    `toml:"max_response_bytes"`
	MaxLines          int      `toml:"max_lines"`
	RedactionPatterns []string `toml:"redaction_patterns"`
}
type Sessions struct {
	IdleTimeout      time.Duration `toml:"idle_timeout"`
	AbsoluteLifetime time.Duration `toml:"absolute_lifetime"`
}
type Coolify struct {
	URL               string `toml:"url"`
	APIToken          string `toml:"-"`
	APITokenFile      string `toml:"api_token_file"`
	AllowInsecureHTTP bool   `toml:"allow_insecure_http"`
}
type Auth struct {
	Mode           string   `toml:"mode"`
	ProxyCIDRs     []string `toml:"proxy_cidrs"`
	IdentityHeader string   `toml:"identity_header"`
	AllowedSubject string   `toml:"allowed_subject"`
}
type Features struct {
	AdvancedAuth bool `toml:"advanced_auth" json:"advancedAuth"`
	Portability  bool `toml:"portability" json:"portability"`
}
type Prometheus struct {
	Enabled bool `toml:"enabled"`
}

func Defaults() Config {
	return Config{
		Paths: Paths{DataDir: "/var/lib/binnacle", HostProc: "/proc", HostSys: "/sys", HostPasswd: "/host/etc/passwd"}, HTTP: HTTP{ListenAddress: ":8080"}, Auth: Auth{Mode: "local", IdentityHeader: "X-Forwarded-User"},
		Collection: Collection{HostInterval: 2 * time.Second, ContainerInterval: 2 * time.Second, MinimumInterval: time.Second}, Live: Live{SSEInterval: 2 * time.Second}, Persistence: Persistence{RawInterval: 10 * time.Second, QueueBatchLimit: 60},
		Retention: Retention{Preset: "balanced", Raw: 48 * time.Hour, OneMinute: 30 * 24 * time.Hour, FifteenMinute: 365 * 24 * time.Hour, OneHour: 0}, Database: Database{TargetBudgetBytes: 1073741824, WarningRatio: .80, CriticalRatio: .95, EmergencyPauseRatio: .98}, Charts: Charts{MaxPointsPerSeries: 1000}, Docker: Docker{SocketPath: "/var/run/docker.sock", MaxConcurrency: 4}, Checks: Checks{MaxConcurrency: 8}, Notifications: Notifications{MaxConcurrency: 4, QueueCapacity: 1000, DeliveryTimeout: 15 * time.Second, ReminderInterval: 2 * time.Hour}, Logs: Logs{MaxResponseBytes: 1048576, MaxLines: 5000}, Sessions: Sessions{IdleTimeout: 12 * time.Hour, AbsoluteLifetime: 720 * time.Hour},
	}
}
func RetentionPreset(name string) (Retention, bool) {
	switch name {
	case "minimal":
		return Retention{Preset: name, Raw: 12 * time.Hour, OneMinute: 7 * 24 * time.Hour, FifteenMinute: 90 * 24 * time.Hour, OneHour: 365 * 24 * time.Hour}, true
	case "balanced":
		return Defaults().Retention, true
	case "long-term":
		return Retention{Preset: name, Raw: 7 * 24 * time.Hour, OneMinute: 90 * 24 * time.Hour, FifteenMinute: 2 * 365 * 24 * time.Hour}, true
	}
	return Retention{}, false
}

func (c *Config) Normalize() {
	if c.Paths.RuntimeDir == "" {
		c.Paths.RuntimeDir = filepath.Join(c.Paths.DataDir, "runtime")
	}
	if c.Paths.DatabasePath == "" {
		c.Paths.DatabasePath = filepath.Join(c.Paths.DataDir, "binnacle.db")
	}
}
func (c Config) Validate() error {
	c.Normalize()
	for name, path := range map[string]string{"data_dir": c.Paths.DataDir, "database_path": c.Paths.DatabasePath, "runtime_dir": c.Paths.RuntimeDir, "host_proc": c.Paths.HostProc, "host_sys": c.Paths.HostSys, "host_passwd": c.Paths.HostPasswd, "docker.socket_path": c.Docker.SocketPath} {
		if path == "" || !filepath.IsAbs(path) {
			return fmt.Errorf("%s must be an absolute path", name)
		}
	}
	if _, _, err := net.SplitHostPort(c.HTTP.ListenAddress); err != nil {
		return fmt.Errorf("http.listen_address: %w", err)
	}
	for _, cidr := range c.HTTP.TrustedProxyCIDRs {
		if err := validateExactProxy(cidr); err != nil {
			return fmt.Errorf("http.trusted_proxy_cidrs: %w", err)
		}
	}
	if c.Auth.Mode != "local" && c.Auth.Mode != "proxy" && c.Auth.Mode != "local_and_proxy" {
		return fmt.Errorf("auth.mode must be local, proxy, or local_and_proxy")
	}
	for _, cidr := range c.Auth.ProxyCIDRs {
		if err := validateExactProxy(cidr); err != nil {
			return fmt.Errorf("auth.proxy_cidrs: %w", err)
		}
	}
	if c.Paths.MasterKeyFile != "" {
		if !filepath.IsAbs(c.Paths.MasterKeyFile) {
			return fmt.Errorf("paths.master_key_file must be an absolute path")
		}
		if c.Paths.MasterKey != "" && !c.Paths.masterKeyLoadedFromFile {
			return fmt.Errorf("paths.master_key and paths.master_key_file cannot both be configured")
		}
	}
	if c.Auth.Mode != "local" && (len(c.Auth.ProxyCIDRs) == 0 || strings.TrimSpace(c.Auth.IdentityHeader) == "" || strings.TrimSpace(c.Auth.AllowedSubject) == "") {
		return fmt.Errorf("proxy auth requires proxy_cidrs, identity_header, and allowed_subject")
	}
	if !c.Features.AdvancedAuth && c.Auth.Mode != "local" {
		return fmt.Errorf("proxy authentication requires features.advanced_auth=true")
	}
	if !c.Features.Portability && c.Prometheus.Enabled {
		return fmt.Errorf("prometheus requires features.portability=true")
	}
	if c.Collection.MinimumInterval <= 0 || c.Collection.HostInterval < c.Collection.MinimumInterval || c.Collection.ContainerInterval < c.Collection.MinimumInterval {
		return fmt.Errorf("collection intervals must be at least minimum_interval")
	}
	for name, d := range map[string]time.Duration{"live.sse_interval": c.Live.SSEInterval, "persistence.raw_interval": c.Persistence.RawInterval, "sessions.idle_timeout": c.Sessions.IdleTimeout, "sessions.absolute_lifetime": c.Sessions.AbsoluteLifetime} {
		if d <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if c.Sessions.AbsoluteLifetime < c.Sessions.IdleTimeout {
		return fmt.Errorf("sessions.absolute_lifetime must be at least idle_timeout")
	}
	if c.Persistence.QueueBatchLimit <= 0 || c.Charts.MaxPointsPerSeries <= 0 || c.Docker.MaxConcurrency <= 0 || c.Checks.MaxConcurrency <= 0 || c.Logs.MaxResponseBytes <= 0 || c.Logs.MaxLines <= 0 || c.Database.TargetBudgetBytes <= 0 {
		return fmt.Errorf("limits and budgets must be positive")
	}
	if c.Logs.MaxResponseBytes > 1<<20 || c.Logs.MaxLines > 5000 || len(c.Logs.RedactionPatterns) > 16 {
		return fmt.Errorf("log limits exceed the supported ceilings")
	}
	if c.Coolify.URL != "" {
		u, err := url.Parse(c.Coolify.URL)
		if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && !(c.Coolify.AllowInsecureHTTP && u.Scheme == "http")) {
			return fmt.Errorf("coolify.url must be a valid HTTPS URL")
		}
	}
	if c.Notifications.MaxConcurrency < 1 || c.Notifications.MaxConcurrency > 32 || c.Notifications.QueueCapacity < 1 || c.Notifications.QueueCapacity > 10000 || c.Notifications.DeliveryTimeout < time.Second || c.Notifications.DeliveryTimeout > time.Minute || c.Notifications.ReminderInterval < time.Minute || c.Notifications.ReminderInterval > 24*time.Hour {
		return fmt.Errorf("notification limits are outside supported bounds")
	}
	if !(0 < c.Database.WarningRatio && c.Database.WarningRatio < c.Database.CriticalRatio && c.Database.CriticalRatio < c.Database.EmergencyPauseRatio && c.Database.EmergencyPauseRatio <= 1) {
		return fmt.Errorf("database ratios must be ordered between zero and one")
	}
	if c.Retention.Preset != "minimal" && c.Retention.Preset != "balanced" && c.Retention.Preset != "long-term" && c.Retention.Preset != "advanced" {
		return fmt.Errorf("retention.preset must be minimal, balanced, long-term, or advanced")
	}
	if c.Retention.Preset == "advanced" && !(c.Retention.Raw > 0 && c.Retention.OneMinute > c.Retention.Raw && c.Retention.FifteenMinute > c.Retention.OneMinute && (c.Retention.OneHour == 0 || c.Retention.OneHour > c.Retention.FifteenMinute)) {
		return fmt.Errorf("advanced retention tiers must be ordered")
	}
	return nil
}

// UIOverridable reports whether a key can be changed without a deployment change.
func UIOverridable(key string) bool {
	switch key {
	case "paths.data_dir", "paths.database_path", "paths.runtime_dir", "paths.master_key", "paths.master_key_file", "http.listen_address", "docker.socket_path", "paths.host_proc", "paths.host_sys", "paths.host_passwd", "coolify.url", "coolify.api_token_file", "coolify.allow_insecure_http", "auth.mode", "auth.proxy_cidrs", "auth.identity_header", "auth.allowed_subject", "features.advanced_auth", "features.portability", "prometheus.enabled", "notifications.allow_private_targets", "notifications.max_concurrency", "notifications.queue_capacity", "notifications.delivery_timeout", "notifications.reminder_interval":
		return false
	}
	return true
}

func validateExactProxy(value string) error {
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return err
	}
	if prefix.Bits() != prefix.Addr().BitLen() {
		return fmt.Errorf("must identify one exact host (/32 for IPv4 or /128 for IPv6)")
	}
	return nil
}
