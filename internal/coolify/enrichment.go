// SPDX-License-Identifier: AGPL-3.0-only
package coolify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
)

type Enricher struct {
	Client                               *APIClient
	db                                   *sql.DB
	MetadataInterval, DeploymentInterval time.Duration
	mu                                   sync.RWMutex
	byUUID                               map[string]ResourceMetadata
	state                                SyncStatus
}
type SyncStatus struct {
	State         string     `json:"state"`
	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	ErrorCode     string     `json:"errorCode,omitempty"`
	Resources     int        `json:"resources"`
}

func NewEnricher(client *APIClient) *Enricher {
	return &Enricher{Client: client, MetadataInterval: 5 * time.Minute, DeploymentInterval: 30 * time.Second, byUUID: map[string]ResourceMetadata{}, state: SyncStatus{State: "unknown"}}
}
func (e *Enricher) SetDB(db *sql.DB) { e.db = db }
func (e *Enricher) Start(ctx context.Context) error {
	if e == nil || e.Client == nil {
		return nil
	}
	if e.db == nil {
		return errors.New("Coolify enrichment storage unavailable")
	}
	_ = e.loadCache(ctx)
	go e.run(ctx)
	return nil
}
func (e *Enricher) Stop(context.Context) error { return nil }
func (e *Enricher) run(ctx context.Context) {
	e.syncMetadata(ctx)
	e.syncDeployments(ctx)
	metadataTicker, deploymentTicker := time.NewTicker(e.MetadataInterval), time.NewTicker(e.DeploymentInterval)
	defer metadataTicker.Stop()
	defer deploymentTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-metadataTicker.C:
			e.syncMetadata(ctx)
		case <-deploymentTicker.C:
			e.syncDeployments(ctx)
		}
	}
}
func (e *Enricher) syncMetadata(ctx context.Context) {
	now := time.Now().UTC()
	values, err := e.Client.Metadata(ctx)
	if err != nil {
		e.setFailure(ctx, now, classifyError(err))
		return
	}
	payload, err := json.Marshal(values)
	if err != nil || len(payload) > 16<<20 {
		e.setFailure(ctx, now, "invalid_response")
		return
	}
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		e.setFailure(ctx, now, "storage")
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO coolify_enrichment_cache(cache_key,payload_json,resource_count,fetched_at) VALUES('metadata',?,?,?) ON CONFLICT(cache_key) DO UPDATE SET payload_json=excluded.payload_json,resource_count=excluded.resource_count,fetched_at=excluded.fetched_at`, string(payload), len(values), now.UnixMilli())
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE coolify_sync_state SET state='healthy',last_attempt_at=?,last_success_at=?,error_code=NULL WHERE id=1`, now.UnixMilli(), now.UnixMilli())
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		e.setFailure(ctx, now, "storage")
		return
	}
	e.replace(values, SyncStatus{State: "healthy", LastAttemptAt: &now, LastSuccessAt: &now, Resources: len(values)})
}
func (e *Enricher) syncDeployments(ctx context.Context) {
	values, err := e.Client.Deployments(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, value := range values {
		tx, err := e.db.BeginTx(ctx, nil)
		if err != nil {
			return
		}
		var previous string
		scanErr := tx.QueryRowContext(ctx, "SELECT last_status FROM coolify_deployments WHERE deployment_uuid=?", value.UUID).Scan(&previous)
		if scanErr != nil && !errors.Is(scanErr, sql.ErrNoRows) {
			tx.Rollback()
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO coolify_deployments(deployment_uuid,resource_uuid,last_status,commit_sha,commit_message,first_seen_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(deployment_uuid) DO UPDATE SET last_status=excluded.last_status,commit_sha=excluded.commit_sha,commit_message=excluded.commit_message,updated_at=excluded.updated_at`, value.UUID, nullString(value.ResourceUUID), value.Status, nullString(value.Commit), nullString(value.CommitMessage), now.UnixMilli(), now.UnixMilli())
		if err == nil && previous != "" && previous != value.Status && terminalDeployment(value.Status) {
			resourceID := localResourceID(ctx, tx, value.ResourceUUID)
			details, _ := json.Marshal(map[string]string{"deploymentUuid": value.UUID, "status": value.Status, "commit": value.Commit, "commitMessage": value.CommitMessage})
			eventID := "coolify_deployment_" + safeID(value.UUID) + "_" + safeID(value.Status)
			_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO events(id,ts,resource_id,type,severity,summary,details_json,correlation_key,source,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, eventID, now.UnixMilli(), nullString(resourceID), "deployment", "info", "Coolify deployment "+value.Status, string(details), value.UUID, "coolify", now.UnixMilli())
			if err == nil {
				_, err = tx.ExecContext(ctx, "UPDATE coolify_deployments SET event_emitted_at=? WHERE deployment_uuid=?", now.UnixMilli(), value.UUID)
			}
		}
		if err != nil {
			tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}
}
func terminalDeployment(status string) bool {
	switch strings.ToLower(status) {
	case "finished", "success", "failed", "cancelled":
		return true
	}
	return false
}
func localResourceID(ctx context.Context, tx *sql.Tx, uuid string) string {
	var id string
	_ = tx.QueryRowContext(ctx, "SELECT id FROM resources WHERE stable_key=?", "coolify:"+uuid).Scan(&id)
	return id
}
func safeID(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, bounded(value, 128))
}
func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func classifyError(err error) string {
	if strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403") {
		return "authorization"
	}
	if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "large") {
		return "limit"
	}
	return "unavailable"
}
func (e *Enricher) setFailure(ctx context.Context, now time.Time, code string) {
	_, _ = e.db.ExecContext(ctx, "UPDATE coolify_sync_state SET state='degraded',last_attempt_at=?,error_code=? WHERE id=1", now.UnixMilli(), code)
	e.mu.Lock()
	e.state.State, e.state.LastAttemptAt, e.state.ErrorCode = "degraded", &now, code
	e.mu.Unlock()
}
func (e *Enricher) replace(values []ResourceMetadata, state SyncStatus) {
	byUUID := make(map[string]ResourceMetadata, len(values))
	for _, value := range values {
		byUUID[value.UUID] = value
	}
	e.mu.Lock()
	e.byUUID, e.state = byUUID, state
	e.mu.Unlock()
}
func (e *Enricher) loadCache(ctx context.Context) error {
	var payload string
	var fetched int64
	err := e.db.QueryRowContext(ctx, "SELECT payload_json,fetched_at FROM coolify_enrichment_cache WHERE cache_key='metadata'").Scan(&payload, &fetched)
	if err != nil {
		return err
	}
	var values []ResourceMetadata
	if json.Unmarshal([]byte(payload), &values) != nil {
		return fmt.Errorf("invalid Coolify cache")
	}
	at := time.UnixMilli(fetched).UTC()
	e.replace(values, SyncStatus{State: "degraded", LastSuccessAt: &at, Resources: len(values), ErrorCode: "stale_cache"})
	return nil
}
func (e *Enricher) Status() SyncStatus { e.mu.RLock(); defer e.mu.RUnlock(); return e.state }
func (e *Enricher) Decorate(_ context.Context, snapshot metrics.Snapshot) metrics.Snapshot {
	if e == nil {
		return snapshot
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	for i := range snapshot.Resources {
		resource := &snapshot.Resources[i]
		if !strings.HasPrefix(resource.StableKey, "coolify:") {
			continue
		}
		metadata, ok := e.byUUID[strings.TrimPrefix(resource.StableKey, "coolify:")]
		if !ok {
			continue
		}
		if !resource.ManualName && metadata.Name != "" {
			resource.Name = metadata.Name
		}
		if !resource.ManualContext && len(metadata.Domains) > 0 {
			resource.Context = metadata.Domains[0]
		}
		if metadata.Project != "" {
			resource.Project = metadata.Project
		}
		if metadata.Environment != "" {
			resource.Environment = metadata.Environment
		}
		if metadata.Category != "" {
			resource.Category = metadata.Category
		}
	}
	return snapshot
}
