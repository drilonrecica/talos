// SPDX-License-Identifier: AGPL-3.0-only

package alerts

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }
func (r *Repository) SetDB(db *sql.DB)     { r.db = db }
func (r *Repository) SeedDefaults(ctx context.Context) error {
	if r.db == nil {
		return errors.New("alerts repository unavailable")
	}
	now := time.Now().UTC().Unix()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, v := range DefaultRules() {
		_, err = tx.ExecContext(ctx, `INSERT INTO alert_rules(id,family,name,built_in,enabled,severity,scope_type,scope_id,threshold,recovery_threshold,trigger_seconds,recovery_seconds,window_seconds,cooldown_seconds,repeat_seconds,suppress_during_deployment,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, v.ID, v.Family, v.Name, v.BuiltIn, v.Enabled, v.Severity, v.ScopeType, v.ScopeID, v.Threshold, v.RecoveryThreshold, int(v.TriggerDuration/time.Second), int(v.RecoveryDuration/time.Second), int(v.Window/time.Second), 300, 7200, v.SuppressDuringDeployment, now, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (r *Repository) Rules(ctx context.Context) ([]Rule, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,family,name,built_in,enabled,severity,scope_type,scope_id,threshold,recovery_threshold,trigger_seconds,recovery_seconds,window_seconds,cooldown_seconds,repeat_seconds,suppress_during_deployment FROM alert_rules ORDER BY built_in DESC,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var v Rule
		var t, rec sql.NullFloat64
		var td, rd, win, cd, rp int64
		if err = rows.Scan(&v.ID, &v.Family, &v.Name, &v.BuiltIn, &v.Enabled, &v.Severity, &v.ScopeType, &v.ScopeID, &t, &rec, &td, &rd, &win, &cd, &rp, &v.SuppressDuringDeployment); err != nil {
			return nil, err
		}
		if t.Valid {
			v.Threshold = &t.Float64
		}
		if rec.Valid {
			v.RecoveryThreshold = &rec.Float64
		}
		v.TriggerDuration = time.Duration(td) * time.Second
		v.RecoveryDuration = time.Duration(rd) * time.Second
		v.Window = time.Duration(win) * time.Second
		v.Cooldown = time.Duration(cd) * time.Second
		v.Repeat = time.Duration(rp) * time.Second
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *Repository) Alerts(ctx context.Context, status, severity, resource, family string, limit, offset int) ([]Alert, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q := `SELECT id,dedup_key,rule_id,family,severity,target_type,target_id,status,started_at,resolved_at,last_observed_at,observed_value,message FROM alerts WHERE 1=1`
	args := []any{}
	for _, f := range []struct{ v, col string }{{status, "status"}, {severity, "severity"}, {resource, "target_id"}, {family, "family"}} {
		if f.v != "" {
			q += " AND " + f.col + "=?"
			args = append(args, f.v)
		}
	}
	q += ` ORDER BY CASE severity WHEN 'critical' THEN 0 ELSE 1 END,started_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Alert{}
	for rows.Next() {
		var a Alert
		var start, last int64
		var resolved sql.NullInt64
		if err = rows.Scan(&a.ID, &a.DedupKey, &a.RuleID, &a.Family, &a.Severity, &a.TargetType, &a.TargetID, &a.Status, &start, &resolved, &last, &a.ObservedValue, &a.Message); err != nil {
			return nil, err
		}
		a.StartedAt = time.Unix(start, 0).UTC()
		a.LastObservedAt = time.Unix(last, 0).UTC()
		if resolved.Valid {
			v := time.Unix(resolved.Int64, 0).UTC()
			a.ResolvedAt = &v
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (r *Repository) Silences(ctx context.Context, activeOnly bool) ([]Silence, error) {
	q := `SELECT id,scope_type,scope_id,reason,starts_at,ends_at,created_by,created_at FROM silences`
	if activeOnly {
		q += ` WHERE starts_at<=unixepoch() AND ends_at>unixepoch()`
	}
	q += ` ORDER BY ends_at DESC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Silence{}
	for rows.Next() {
		var s Silence
		var a, b, c int64
		if err = rows.Scan(&s.ID, &s.ScopeType, &s.ScopeID, &s.Reason, &a, &b, &s.CreatedBy, &c); err != nil {
			return nil, err
		}
		s.StartsAt = time.Unix(a, 0).UTC()
		s.EndsAt = time.Unix(b, 0).UTC()
		s.CreatedAt = time.Unix(c, 0).UTC()
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *Repository) CreateSilence(ctx context.Context, s *Silence) error {
	if s.EndsAt.Sub(s.StartsAt) <= 0 || s.EndsAt.Sub(s.StartsAt) > 365*24*time.Hour || len(s.Reason) < 1 || len(s.Reason) > 500 {
		return errors.New("invalid silence")
	}
	if s.ID == "" {
		s.ID = id()
	}
	s.CreatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO silences(id,scope_type,scope_id,reason,starts_at,ends_at,created_by,created_at)VALUES(?,?,?,?,?,?,?,?)`, s.ID, s.ScopeType, s.ScopeID, s.Reason, s.StartsAt.Unix(), s.EndsAt.Unix(), s.CreatedBy, s.CreatedAt.Unix())
	return err
}
func (r *Repository) DeleteSilence(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM silences WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (r *Repository) UpdateRule(ctx context.Context, v Rule) error {
	if v.Severity != Warning && v.Severity != Critical {
		return errors.New("invalid severity")
	}
	if v.TriggerDuration < 0 || v.RecoveryDuration < 0 || v.Repeat < 0 || v.Cooldown < 0 {
		return errors.New("invalid duration")
	}
	res, err := r.db.ExecContext(ctx, `UPDATE alert_rules SET enabled=?,severity=?,threshold=?,recovery_threshold=?,trigger_seconds=?,recovery_seconds=?,window_seconds=?,cooldown_seconds=?,repeat_seconds=?,suppress_during_deployment=?,updated_at=? WHERE id=?`, v.Enabled, v.Severity, v.Threshold, v.RecoveryThreshold, int(v.TriggerDuration/time.Second), int(v.RecoveryDuration/time.Second), int(v.Window/time.Second), int(v.Cooldown/time.Second), int(v.Repeat/time.Second), v.SuppressDuringDeployment, time.Now().UTC().Unix(), v.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (r *Repository) CreateRule(ctx context.Context, v Rule) error {
	if v.ID == "" || v.Family == "" || v.Name == "" || v.ScopeType == "global" || v.ScopeID == "" {
		return errors.New("invalid scoped rule")
	}
	if v.Severity != Warning && v.Severity != Critical {
		return errors.New("invalid severity")
	}
	switch v.ScopeType {
	case "host", "filesystem", "project", "resource", "check":
	default:
		return errors.New("invalid scope")
	}
	now := time.Now().UTC().Unix()
	_, err := r.db.ExecContext(ctx, `INSERT INTO alert_rules(id,family,name,built_in,enabled,severity,scope_type,scope_id,threshold,recovery_threshold,trigger_seconds,recovery_seconds,window_seconds,cooldown_seconds,repeat_seconds,suppress_during_deployment,created_at,updated_at)VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, v.ID, v.Family, v.Name, false, v.Enabled, v.Severity, v.ScopeType, v.ScopeID, v.Threshold, v.RecoveryThreshold, int(v.TriggerDuration/time.Second), int(v.RecoveryDuration/time.Second), int(v.Window/time.Second), int(v.Cooldown/time.Second), int(v.Repeat/time.Second), v.SuppressDuringDeployment, now, now)
	return err
}
func (r *Repository) Cleanup(ctx context.Context, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM alerts WHERE status='resolved' AND resolved_at<?`, now.Add(-365*24*time.Hour).Unix()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM silences WHERE ends_at<?`, now.Add(-90*24*time.Hour).Unix()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM deployment_grace_periods WHERE ends_at<?`, now.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
func dedup(family, target string) string      { return family + ":" + target }
func message(rule Rule, target string) string { return fmt.Sprintf("%s on %s", rule.Name, target) }
