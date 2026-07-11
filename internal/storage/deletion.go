// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type DeletionKind string

const (
	DeleteResource DeletionKind = "resource"
	DeleteBefore   DeletionKind = "before"
	DeleteAll      DeletionKind = "all"
	DeleteArchived DeletionKind = "archived_resource"
)

type DeletionRequest struct {
	Kind       DeletionKind `json:"kind"`
	ResourceID string       `json:"resourceId,omitempty"`
	Before     time.Time    `json:"before,omitempty"`
}
type DeletionPreview struct {
	Token        string          `json:"token"`
	Confirmation string          `json:"confirmation"`
	TotalRows    int64           `json:"totalRows"`
	ExpiresAt    time.Time       `json:"expiresAt"`
	Scope        DeletionRequest `json:"scope"`
	FenceAt      time.Time       `json:"fenceAt"`
}
type DeletionJob struct {
	ID                     string       `json:"id"`
	Kind                   DeletionKind `json:"kind"`
	ResourceID             string       `json:"resourceId,omitempty"`
	State                  string       `json:"state"`
	TotalRows, DeletedRows int64        `json:"totalRows","deletedRows"`
	Error                  string       `json:"error,omitempty"`
}

func (m *Manager) PreviewDeletion(ctx context.Context, request DeletionRequest) (DeletionPreview, error) {
	if m.db == nil {
		return DeletionPreview{}, errors.New("storage is not open")
	}
	if err := validDeletion(request); err != nil {
		return DeletionPreview{}, err
	}
	if request.Kind == DeleteResource || request.Kind == DeleteArchived {
		var status string
		if err := m.db.QueryRowContext(ctx, "SELECT status FROM resources WHERE id=?", request.ResourceID).Scan(&status); err != nil {
			return DeletionPreview{}, errors.New("resource not found")
		}
		if request.Kind == DeleteArchived && status != "archived" {
			return DeletionPreview{}, errors.New("resource is not archived")
		}
	}
	now := time.Now().UTC()
	fence := now.UnixMilli()
	before := request.Before.UTC().UnixMilli()
	if request.Kind == DeleteBefore && before > fence {
		before = fence
	}
	total, err := m.deletionCount(ctx, request.Kind, request.ResourceID, before, fence)
	if err != nil {
		return DeletionPreview{}, err
	}
	token, err := randomDeletionToken()
	if err != nil {
		return DeletionPreview{}, err
	}
	confirmation := confirmationFor(request)
	summary, _ := json.Marshal(map[string]any{"totalRows": total, "kind": request.Kind, "resourceId": request.ResourceID})
	expires := now.Add(10 * time.Minute)
	_, err = m.db.ExecContext(ctx, "INSERT INTO history_deletion_previews(id_hash,kind,resource_id,before_ts,fence_ts,confirmation,summary_json,expires_at) VALUES(?,?,?,?,?,?,?,?)", hashDeletionToken(token), request.Kind, nullString(request.ResourceID), nullableBefore(request.Kind, before), fence, confirmation, string(summary), expires.UnixMilli())
	if err != nil {
		return DeletionPreview{}, err
	}
	return DeletionPreview{Token: token, Confirmation: confirmation, TotalRows: total, ExpiresAt: expires, Scope: request, FenceAt: time.UnixMilli(fence).UTC()}, nil
}
func (m *Manager) CreateDeletion(ctx context.Context, token, confirmation, actor string) (DeletionJob, error) {
	if m.db == nil {
		return DeletionJob{}, errors.New("storage is not open")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return DeletionJob{}, err
	}
	defer tx.Rollback()
	var kind DeletionKind
	var resource sql.NullString
	var before sql.NullInt64
	var fence, expires int64
	var expected string
	err = tx.QueryRowContext(ctx, "SELECT kind,resource_id,before_ts,fence_ts,confirmation,expires_at FROM history_deletion_previews WHERE id_hash=? AND used_at IS NULL", hashDeletionToken(token)).Scan(&kind, &resource, &before, &fence, &expected, &expires)
	if err != nil {
		return DeletionJob{}, errors.New("deletion preview is invalid")
	}
	if time.Now().UTC().UnixMilli() >= expires || confirmation != expected {
		return DeletionJob{}, errors.New("deletion confirmation is invalid")
	}
	total, err := m.deletionCountTx(ctx, tx, kind, resource.String, nullableInt(before), fence)
	if err != nil {
		return DeletionJob{}, err
	}
	id, err := newDeletionID()
	if err != nil {
		return DeletionJob{}, err
	}
	now := time.Now().UTC().UnixMilli()
	_, err = tx.ExecContext(ctx, "INSERT INTO history_deletion_jobs(id,kind,resource_id,before_ts,fence_ts,confirmation,state,requested_by,requested_at,total_rows) VALUES(?,?,?,?,?,?,?,?,?,?)", id, kind, resource, before, fence, expected, "queued", nullString(actor), now, total)
	if err != nil {
		return DeletionJob{}, fmt.Errorf("another history deletion is already active: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "UPDATE history_deletion_previews SET used_at=? WHERE id_hash=?", now, hashDeletionToken(token)); err != nil {
		return DeletionJob{}, err
	}
	if err = tx.Commit(); err != nil {
		return DeletionJob{}, err
	}
	return DeletionJob{ID: id, Kind: kind, ResourceID: resource.String, State: "queued", TotalRows: total}, nil
}
func (m *Manager) DeletionJob(ctx context.Context, id string) (DeletionJob, error) {
	var j DeletionJob
	var resource sql.NullString
	err := m.db.QueryRowContext(ctx, "SELECT id,kind,resource_id,state,total_rows,deleted_rows,COALESCE(error_message,'') FROM history_deletion_jobs WHERE id=?", id).Scan(&j.ID, &j.Kind, &resource, &j.State, &j.TotalRows, &j.DeletedRows, &j.Error)
	j.ResourceID = resource.String
	return j, err
}
func (m *Manager) CancelDeletion(ctx context.Context, id string) error {
	r, err := m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET state=CASE WHEN state='queued' THEN 'cancelled' ELSE 'cancelling' END,finished_at=CASE WHEN state='queued' THEN ? ELSE finished_at END WHERE id=? AND state IN ('queued','running')", time.Now().UTC().UnixMilli(), id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("deletion job is not cancellable")
	}
	return nil
}
func (m *Manager) RetryDeletion(ctx context.Context, id string) error {
	r, err := m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET state='queued',error_message=NULL,finished_at=NULL WHERE id=? AND state IN ('cancelled','failed')", id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("deletion job is not retryable")
	}
	return nil
}
func (m *Manager) RunDeletion(ctx context.Context, id string) error {
	var kind DeletionKind
	var resource sql.NullString
	var before sql.NullInt64
	var fence int64
	if err := m.db.QueryRowContext(ctx, "SELECT kind,resource_id,before_ts,fence_ts FROM history_deletion_jobs WHERE id=? AND state='queued'", id).Scan(&kind, &resource, &before, &fence); err != nil {
		return err
	}
	if _, err := m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET state='running',started_at=? WHERE id=?", time.Now().UTC().UnixMilli(), id); err != nil {
		return err
	}
	var deleted int64
	for _, table := range deletionTables(kind) {
		for {
			var state string
			if err := m.db.QueryRowContext(ctx, "SELECT state FROM history_deletion_jobs WHERE id=?", id).Scan(&state); err != nil {
				return err
			}
			if state == "cancelling" {
				_, err := m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET state='cancelled',finished_at=? WHERE id=?", time.Now().UTC().UnixMilli(), id)
				return err
			}
			n, err := m.deleteBatch(ctx, table, kind, resource.String, nullableInt(before), fence)
			if err != nil {
				_, _ = m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET state='failed',error_message=?,finished_at=? WHERE id=?", safeDeletionError(err), time.Now().UTC().UnixMilli(), id)
				return err
			}
			deleted += n
			_, _ = m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET deleted_rows=?,current_table=? WHERE id=?", deleted, table, id)
			if n == 0 {
				break
			}
		}
	}
	if kind == DeleteArchived {
		for _, query := range []string{"DELETE FROM container_instances WHERE resource_id=?", "DELETE FROM resources WHERE id=? AND status='archived'"} {
			result, e := m.db.ExecContext(ctx, query, resource.String)
			if e != nil {
				return e
			}
			n, _ := result.RowsAffected()
			deleted += n
		}
	}
	if kind == DeleteAll {
		for _, query := range []string{"DELETE FROM container_instances", "DELETE FROM resources", "DELETE FROM boot_sessions", "DELETE FROM hosts"} {
			result, e := m.db.ExecContext(ctx, query)
			if e != nil {
				return e
			}
			n, _ := result.RowsAffected()
			deleted += n
		}
	}
	_, err := m.db.ExecContext(ctx, "UPDATE history_deletion_jobs SET state='completed',deleted_rows=?,finished_at=?,current_table=NULL WHERE id=?", deleted, time.Now().UTC().UnixMilli(), id)
	return err
}

func (m *Manager) runDeletionWorker(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		var id string
		if m.db != nil {
			_ = m.db.QueryRowContext(ctx, "SELECT id FROM history_deletion_jobs WHERE state='queued' ORDER BY requested_at LIMIT 1").Scan(&id)
		}
		if id != "" {
			_ = m.RunDeletion(ctx, id)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func deletionTables(kind DeletionKind) []string {
	tables := []string{"container_instance_samples_10s", "resource_samples_10s", "resource_rollups_1m", "resource_rollups_15m", "resource_rollups_1h", "events"}
	if kind == DeleteBefore || kind == DeleteAll {
		return append([]string{"host_samples_10s", "host_rollups_1m", "host_rollups_15m", "host_rollups_1h", "collector_state_events"}, tables...)
	}
	return tables
}
func (m *Manager) deleteBatch(ctx context.Context, table string, kind DeletionKind, resource string, before, fence int64) (int64, error) {
	where := "ts<=?"
	args := []any{fence}
	if kind == DeleteBefore {
		where = "ts<?"
		args = []any{before}
	}
	if (kind == DeleteResource || kind == DeleteArchived) && table == "container_instance_samples_10s" {
		q := "DELETE FROM container_instance_samples_10s WHERE rowid IN (SELECT samples.rowid FROM container_instance_samples_10s samples JOIN container_instances instances ON instances.id=samples.container_instance_id WHERE samples.ts<=? AND instances.resource_id=? LIMIT 500)"
		result, err := m.db.ExecContext(ctx, q, fence, resource)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}
	if (kind == DeleteResource || kind == DeleteArchived) && resourceScopedTable(table) {
		where += " AND resource_id=?"
		args = append(args, resource)
	}
	q := "DELETE FROM " + table + " WHERE rowid IN (SELECT rowid FROM " + table + " WHERE " + where + " LIMIT 500)"
	r, err := m.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}
func (m *Manager) deletionCount(ctx context.Context, k DeletionKind, r string, b, f int64) (int64, error) {
	return m.deletionCountTx(ctx, m.db, k, r, b, f)
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (m *Manager) deletionCountTx(ctx context.Context, q queryer, k DeletionKind, r string, b, f int64) (int64, error) {
	var total int64
	for _, table := range deletionTables(k) {
		where := "ts<=?"
		args := []any{f}
		if k == DeleteBefore {
			where = "ts<?"
			args = []any{b}
		}
		if (k == DeleteResource || k == DeleteArchived) && table == "container_instance_samples_10s" {
			var n int64
			if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM container_instance_samples_10s samples JOIN container_instances instances ON instances.id=samples.container_instance_id WHERE samples.ts<=? AND instances.resource_id=?", f, r).Scan(&n); err != nil {
				return 0, err
			}
			total += n
			continue
		}
		if (k == DeleteResource || k == DeleteArchived) && resourceScopedTable(table) {
			where += " AND resource_id=?"
			args = append(args, r)
		}
		var n int64
		if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE "+where, args...).Scan(&n); err != nil {
			return 0, err
		}
		total += n
	}
	if k == DeleteArchived {
		for _, query := range []string{"SELECT COUNT(*) FROM container_instances WHERE resource_id=?", "SELECT COUNT(*) FROM resources WHERE id=? AND status='archived'"} {
			var n int64
			if err := q.QueryRowContext(ctx, query, r).Scan(&n); err != nil {
				return 0, err
			}
			total += n
		}
	}
	if k == DeleteAll {
		for _, table := range []string{"container_instances", "resources", "boot_sessions", "hosts"} {
			var n int64
			if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
				return 0, err
			}
			total += n
		}
	}
	return total, nil
}
func resourceScopedTable(table string) bool {
	return table == "resource_samples_10s" || table == "resource_rollups_1m" || table == "resource_rollups_15m" || table == "resource_rollups_1h" || table == "events"
}
func validDeletion(r DeletionRequest) error {
	switch r.Kind {
	case DeleteResource, DeleteArchived:
		if r.ResourceID == "" {
			return errors.New("resource id is required")
		}
	case DeleteBefore:
		if r.Before.IsZero() {
			return errors.New("before timestamp is required")
		}
	case DeleteAll:
	default:
		return errors.New("invalid deletion kind")
	}
	return nil
}
func confirmationFor(r DeletionRequest) string {
	switch r.Kind {
	case DeleteResource:
		return "DELETE HISTORY " + r.ResourceID
	case DeleteArchived:
		return "PURGE ARCHIVED " + r.ResourceID
	case DeleteBefore:
		return "DELETE HISTORY BEFORE " + r.Before.UTC().Format("2006-01-02")
	default:
		return "RESET ALL HISTORY"
	}
}
func randomDeletionToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}
func hashDeletionToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.RawStdEncoding.EncodeToString(h[:])
}
func newDeletionID() (string, error) {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	return "del_" + hex.EncodeToString(b), err
}
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullableBefore(k DeletionKind, b int64) any {
	if k != DeleteBefore {
		return nil
	}
	return b
}
func nullableInt(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}
func safeDeletionError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 160 {
		return s[:160]
	}
	return s
}
