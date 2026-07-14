// SPDX-License-Identifier: AGPL-3.0-only
package coolify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"

	"github.com/drilonrecica/binnacle/internal/auth"
	"github.com/drilonrecica/binnacle/internal/metrics"
)

const coolifyTokenSecret = "coolify.api_token"

type Integration struct {
	db                       *sql.DB
	secrets                  *auth.SecretStore
	env                      ClientConfig
	environmentAuthoritative bool
	mu                       sync.RWMutex
	ctx                      context.Context
	cancel                   context.CancelFunc
	enricher                 *Enricher
	url                      string
	tokenConfigured          bool
}
type IntegrationStatus struct {
	Enabled                  bool       `json:"enabled"`
	URL                      string     `json:"url,omitempty"`
	TokenConfigured          bool       `json:"tokenConfigured"`
	EnvironmentAuthoritative bool       `json:"environmentAuthoritative"`
	Collector                SyncStatus `json:"collector"`
}

func NewIntegration(secrets *auth.SecretStore, env ClientConfig) *Integration {
	return &Integration{secrets: secrets, env: env, environmentAuthoritative: env.BaseURL != "" || env.Token != ""}
}
func (i *Integration) SetDB(db *sql.DB) { i.db = db }
func (i *Integration) Start(ctx context.Context) error {
	i.mu.Lock()
	i.ctx = ctx
	i.mu.Unlock()
	config, configured, err := i.effectiveConfig(ctx)
	if err != nil {
		return err
	}
	if configured {
		return i.activate(config)
	}
	return nil
}
func (i *Integration) Stop(context.Context) error {
	i.mu.Lock()
	if i.cancel != nil {
		i.cancel()
	}
	i.cancel = nil
	i.mu.Unlock()
	return nil
}
func (i *Integration) effectiveConfig(ctx context.Context) (ClientConfig, bool, error) {
	config := i.env
	if i.environmentAuthoritative {
		return config, config.BaseURL != "" && config.Token != "", nil
	}
	var raw string
	err := i.db.QueryRowContext(ctx, "SELECT value_json FROM settings WHERE key='coolify.url'").Scan(&raw)
	if err == nil {
		_ = json.Unmarshal([]byte(raw), &config.BaseURL)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return config, false, err
	}
	token, err := i.secrets.Get(ctx, coolifyTokenSecret)
	if err == nil {
		config.Token = string(token)
	} else if !errors.Is(err, auth.ErrSecretNotFound) && !errors.Is(err, auth.ErrMasterKeyMissing) {
		return config, false, err
	}
	return config, config.BaseURL != "" && config.Token != "", nil
}
func (i *Integration) activate(config ClientConfig) error {
	client, err := NewAPIClient(config)
	if err != nil {
		return err
	}
	enricher := NewEnricher(client)
	enricher.SetDB(i.db)
	i.mu.Lock()
	if i.cancel != nil {
		i.cancel()
	}
	ctx, cancel := context.WithCancel(i.ctx)
	i.cancel = cancel
	i.enricher = enricher
	i.url = config.BaseURL
	i.tokenConfigured = config.Token != ""
	i.mu.Unlock()
	return enricher.Start(ctx)
}
func (i *Integration) Configure(ctx context.Context, url, token string) error {
	if i.environmentAuthoritative {
		return errors.New("environment configuration is authoritative")
	}
	config := ClientConfig{BaseURL: url, Token: token, AllowInsecureHTTP: i.env.AllowInsecureHTTP}
	if token == "" {
		existing, err := i.secrets.Get(ctx, coolifyTokenSecret)
		if err != nil {
			return errors.New("Coolify API token is required")
		}
		config.Token = string(existing)
	}
	if _, err := NewAPIClient(config); err != nil {
		return err
	}
	raw, _ := json.Marshal(url)
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at,updated_by) VALUES('coolify.url',?,unixepoch('subsec')*1000,'admin') ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at,updated_by=excluded.updated_by`, string(raw))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if token != "" {
		if err = i.secrets.Put(ctx, coolifyTokenSecret, []byte(token)); err != nil {
			return err
		}
	}
	return i.activate(config)
}
func (i *Integration) Test(ctx context.Context, url, token string) error {
	config := ClientConfig{BaseURL: url, Token: token, AllowInsecureHTTP: i.env.AllowInsecureHTTP}
	if i.environmentAuthoritative {
		config = i.env
	} else {
		if config.BaseURL == "" {
			i.mu.RLock()
			config.BaseURL = i.url
			i.mu.RUnlock()
		}
		if config.Token == "" {
			value, err := i.secrets.Get(ctx, coolifyTokenSecret)
			if err != nil {
				return err
			}
			config.Token = string(value)
		}
	}
	client, err := NewAPIClient(config)
	if err != nil {
		return err
	}
	_, err = client.Metadata(ctx)
	return err
}
func (i *Integration) Status() IntegrationStatus {
	i.mu.RLock()
	defer i.mu.RUnlock()
	status := IntegrationStatus{Enabled: i.enricher != nil, URL: i.url, TokenConfigured: i.tokenConfigured, EnvironmentAuthoritative: i.environmentAuthoritative, Collector: SyncStatus{State: "unknown"}}
	if i.enricher != nil {
		status.Collector = i.enricher.Status()
	}
	return status
}
func (i *Integration) Decorate(ctx context.Context, snapshot metrics.Snapshot) metrics.Snapshot {
	i.mu.RLock()
	enricher := i.enricher
	i.mu.RUnlock()
	if enricher == nil {
		return snapshot
	}
	return enricher.Decorate(ctx, snapshot)
}
