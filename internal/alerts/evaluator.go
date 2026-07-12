// SPDX-License-Identifier: AGPL-3.0-only

package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/drilonrecica/binnacle/internal/metrics"
	"sync"
	"time"
)

type Evaluator struct {
	Repo     *Repository
	Engine   *metrics.Engine
	Interval time.Duration
	Now      func() time.Time
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewEvaluator(repo *Repository, engine *metrics.Engine) *Evaluator {
	return &Evaluator{Repo: repo, Engine: engine, Interval: 10 * time.Second, Now: time.Now}
}
func (e *Evaluator) Start(ctx context.Context) error {
	if e.Repo == nil || e.Engine == nil {
		return errors.New("alert evaluator dependencies unavailable")
	}
	ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = e.Evaluate(ctx)
			}
		}
	}()
	return nil
}
func (e *Evaluator) Stop(context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
	return nil
}
func (e *Evaluator) Evaluate(ctx context.Context) error {
	rules, err := e.Repo.Rules(ctx)
	if err != nil {
		return err
	}
	snap := e.Engine.Snapshot()
	now := e.Now().UTC()
	if err := e.Repo.Cleanup(ctx, now); err != nil {
		return err
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch r.Family {
		case FamilyHostCPU:
			if effectiveRule(rules, r.Family, "host", "server").ID != r.ID {
				continue
			}
			if snap.Host.CPUPercent != nil {
				err = e.evaluate(ctx, r, "host", "server", *snap.Host.CPUPercent, *snap.Host.CPUPercent > *r.Threshold, *snap.Host.CPUPercent < *r.RecoveryThreshold, now)
			}
		case FamilyHostMemory:
			if effectiveRule(rules, r.Family, "host", "server").ID != r.ID {
				continue
			}
			if snap.Host.MemoryPercent != nil {
				err = e.evaluate(ctx, r, "host", "server", *snap.Host.MemoryPercent, *snap.Host.MemoryPercent > *r.Threshold, *snap.Host.MemoryPercent < *r.RecoveryThreshold, now)
			}
		case FamilyDockerDown:
			if effectiveRule(rules, r.Family, "server", "docker").ID != r.ID {
				continue
			}
			h := snap.Collectors["docker"]
			down := h.State == metrics.CollectorDown
			err = e.evaluate(ctx, r, "server", "docker", 0, down, h.State == metrics.CollectorHealthy, now)
		}
		if err != nil {
			return err
		}
	}
	if err := e.evaluateFilesystems(ctx, rules, now); err != nil {
		return err
	}
	if err := e.evaluateEvents(ctx, rules, now); err != nil {
		return err
	}
	if err := e.evaluatePersistence(ctx, rules, now); err != nil {
		return err
	}
	if err := e.correlateDeployments(ctx, now); err != nil {
		return err
	}
	return e.evaluateChecks(ctx, rules, now)
}

func (e *Evaluator) evaluatePersistence(ctx context.Context, rules []Rule, now time.Time) error {
	rule := effectiveRule(rules, FamilyPersistence, "server", "persistence")
	if rule.ID == "" {
		return nil
	}
	var eventType string
	err := e.Repo.db.QueryRowContext(ctx, `SELECT type FROM events WHERE type IN ('persistence_emergency','persistence_degraded','persistence_resumed') ORDER BY ts DESC,id DESC LIMIT 1`).Scan(&eventType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	failing := eventType == "persistence_emergency" || eventType == "persistence_degraded"
	return e.evaluate(ctx, rule, "server", "persistence", 0, failing, eventType == "persistence_resumed", now)
}
func (e *Evaluator) evaluateChecks(ctx context.Context, rules []Rule, now time.Time) error {
	rows, err := e.Repo.db.QueryContext(ctx, `SELECT c.id,c.resource_id,c.required,COALESCE(s.status,'unknown'),COALESCE(s.consecutive_successes,0) FROM health_checks c LEFT JOIN health_check_state s ON s.check_id=c.id WHERE c.enabled=1`)
	if err != nil {
		return err
	}
	type observation struct {
		checkID, resource, status string
		required                  bool
		successes                 int
	}
	observations := []observation{}
	for rows.Next() {
		var v observation
		if err = rows.Scan(&v.checkID, &v.resource, &v.required, &v.status, &v.successes); err != nil {
			rows.Close()
			return err
		}
		observations = append(observations, v)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, v := range observations {
		family := FamilyOptionalCheck
		if v.required {
			family = FamilyRequiredCheck
		}
		rule := effectiveRule(rules, family, "resource", v.resource)
		if rule.ID == "" {
			continue
		}
		failing := v.status == "failure"
		recovered := v.status == "success"
		if v.successes >= 2 {
			rule.RecoveryDuration = 0
		}
		if err = e.evaluate(ctx, rule, "resource", v.resource, 0, failing, recovered, now); err != nil {
			return err
		}
	}
	return nil
}

func (e *Evaluator) evaluateFilesystems(ctx context.Context, rules []Rule, now time.Time) error {
	rows, err := e.Repo.db.QueryContext(ctx, `SELECT f.mount_key,f.used_pct,f.inodes_used_pct FROM filesystem_samples_1m f JOIN (SELECT mount_key,MAX(ts) ts FROM filesystem_samples_1m GROUP BY mount_key) latest ON latest.mount_key=f.mount_key AND latest.ts=f.ts`)
	if err != nil {
		return err
	}
	type observation struct {
		mount        string
		used, inodes sql.NullFloat64
	}
	observations := []observation{}
	for rows.Next() {
		var v observation
		if err = rows.Scan(&v.mount, &v.used, &v.inodes); err != nil {
			rows.Close()
			return err
		}
		observations = append(observations, v)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, v := range observations {
		for _, rule := range rules {
			if !rule.Enabled || rule.Threshold == nil || rule.RecoveryThreshold == nil {
				continue
			}
			if effectiveRule(rules, rule.Family, "filesystem", v.mount).ID != rule.ID {
				continue
			}
			var value sql.NullFloat64
			switch rule.Family {
			case FamilyFilesystemWarning, FamilyFilesystemCritical:
				value = v.used
			case FamilyInodeWarning, FamilyInodeCritical:
				value = v.inodes
			default:
				continue
			}
			if value.Valid {
				if err = e.evaluate(ctx, rule, "filesystem", v.mount, value.Float64, value.Float64 > *rule.Threshold, value.Float64 < *rule.RecoveryThreshold, now); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func effectiveRule(rules []Rule, family, targetType, target string) Rule {
	var global Rule
	for _, rule := range rules {
		if !rule.Enabled || rule.Family != family {
			continue
		}
		if rule.ScopeType == targetType && rule.ScopeID == target {
			return rule
		}
		if rule.ScopeType == "global" {
			global = rule
		}
	}
	return global
}

func (e *Evaluator) evaluateEvents(ctx context.Context, rules []Rule, now time.Time) error {
	for _, rule := range rules {
		if !rule.Enabled || rule.Threshold == nil || (rule.Family != FamilyRestartStorm && rule.Family != FamilyOOMLoop) {
			continue
		}
		eventType := "container_restart"
		if rule.Family == FamilyOOMLoop {
			eventType = "container_oom"
		}
		rows, err := e.Repo.db.QueryContext(ctx, `SELECT resource_id,COUNT(*) FROM events WHERE type=? AND resource_id IS NOT NULL AND ts>=? GROUP BY resource_id`, eventType, now.Add(-rule.Window).UnixMilli())
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		counts := map[string]float64{}
		for rows.Next() {
			var resource string
			var count float64
			if err = rows.Scan(&resource, &count); err != nil {
				rows.Close()
				return err
			}
			counts[resource] = count
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for resource, count := range counts {
			seen[resource] = true
			failing := count > *rule.Threshold
			if rule.Family == FamilyOOMLoop {
				failing = count >= *rule.Threshold
			}
			if err = e.evaluate(ctx, rule, "resource", resource, count, failing, count <= 0, now); err != nil {
				return err
			}
		}
		stateRows, err := e.Repo.db.QueryContext(ctx, `SELECT target_id FROM alert_evaluation_state WHERE rule_id=? AND phase!='healthy'`, rule.ID)
		if err != nil {
			return err
		}
		unseen := []string{}
		for stateRows.Next() {
			var resource string
			if err = stateRows.Scan(&resource); err != nil {
				stateRows.Close()
				return err
			}
			if !seen[resource] {
				unseen = append(unseen, resource)
			}
		}
		if err = stateRows.Err(); err != nil {
			stateRows.Close()
			return err
		}
		stateRows.Close()
		for _, resource := range unseen {
			if err = e.evaluate(ctx, rule, "resource", resource, 0, false, true, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Evaluator) correlateDeployments(ctx context.Context, now time.Time) error {
	rows, err := e.Repo.db.QueryContext(ctx, `SELECT resource_id,MAX(ts),CASE WHEN MAX(CASE WHEN type='deployment' THEN 1 ELSE 0 END)=1 THEN 'confirmed' ELSE 'likely' END FROM events WHERE type IN ('deployment','deployment_likely','container_replacement') AND resource_id IS NOT NULL AND ts>=? GROUP BY resource_id`, now.Add(-10*time.Minute).UnixMilli())
	if err != nil {
		return err
	}
	type deployment struct {
		ms         int64
		confidence string
	}
	deployments := map[string]deployment{}
	for rows.Next() {
		var resource string
		var ms int64
		var confidence string
		if err = rows.Scan(&resource, &ms, &confidence); err != nil {
			rows.Close()
			return err
		}
		deployments[resource] = deployment{ms, confidence}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for resource, deployment := range deployments {
		at := time.UnixMilli(deployment.ms).UTC()
		if _, err = e.Repo.db.ExecContext(ctx, `INSERT INTO deployment_grace_periods(resource_id,starts_at,ends_at,confidence)VALUES(?,?,?,?) ON CONFLICT(resource_id) DO UPDATE SET starts_at=excluded.starts_at,ends_at=excluded.ends_at,confidence=excluded.confidence`, resource, at.Unix(), at.Add(5*time.Minute).Unix(), deployment.confidence); err != nil {
			return err
		}
	}
	return nil
}
func (e *Evaluator) evaluate(ctx context.Context, rule Rule, targetType, target string, value float64, failing, recovered bool, now time.Time) error {
	key := dedup(rule.Family, target)
	tx, err := e.Repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	state := State{Phase: Healthy, Since: now}
	var phase string
	var since int64
	var notified, cooldown sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT phase,phase_since,last_notified_at,cooldown_until FROM alert_evaluation_state WHERE dedup_key=?`, key).Scan(&phase, &since, &notified, &cooldown)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		state.Phase = Phase(phase)
		state.Since = time.Unix(since, 0).UTC()
		if notified.Valid {
			v := time.Unix(notified.Int64, 0).UTC()
			state.LastNotifiedAt = &v
		}
		if cooldown.Valid {
			v := time.Unix(cooldown.Int64, 0).UTC()
			state.CooldownUntil = &v
		}
	}
	suppressed, err := suppressedTx(ctx, tx, rule, target, now)
	if err != nil {
		return err
	}
	tr := Advance(now, state, failing, recovered, suppressed, rule)
	details, _ := json.Marshal(map[string]any{"observed": value})
	_, err = tx.ExecContext(ctx, `INSERT INTO alert_evaluation_state(dedup_key,rule_id,target_type,target_id,phase,phase_since,last_evaluated_at,last_notified_at,cooldown_until,observed_value,details_json)VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(dedup_key) DO UPDATE SET rule_id=excluded.rule_id,phase=excluded.phase,phase_since=excluded.phase_since,last_evaluated_at=excluded.last_evaluated_at,last_notified_at=excluded.last_notified_at,cooldown_until=excluded.cooldown_until,observed_value=excluded.observed_value,details_json=excluded.details_json`, key, rule.ID, targetType, target, tr.State.Phase, tr.State.Since.Unix(), now.Unix(), timePtr(tr.State.LastNotifiedAt), timePtr(tr.State.CooldownUntil), value, string(details))
	if err != nil {
		return err
	}
	if tr.Triggered {
		alertID := id()
		_, err = tx.ExecContext(ctx, `INSERT INTO alerts(id,dedup_key,rule_id,family,severity,target_type,target_id,status,started_at,last_observed_at,observed_value,message)VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, alertID, key, rule.ID, rule.Family, rule.Severity, targetType, target, "firing", now.Unix(), now.Unix(), value, message(rule, target))
		if err == nil {
			err = insertEvent(ctx, tx, alertID, rule, target, "alert_triggered", now)
		}
	} else if tr.Repeated {
		err = insertEvent(ctx, tx, id(), rule, target, "alert_repeated", now)
	} else if tr.Resolved {
		var alertID string
		err = tx.QueryRowContext(ctx, `SELECT id FROM alerts WHERE dedup_key=? AND status='firing' ORDER BY started_at DESC LIMIT 1`, key).Scan(&alertID)
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		} else if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE alerts SET status='resolved',resolved_at=?,last_observed_at=? WHERE id=?`, now.Unix(), now.Unix(), alertID)
			if err == nil {
				err = insertEvent(ctx, tx, alertID, rule, target, "alert_resolved", now)
			}
		}
	} else if state.Phase == Firing || state.Phase == Recovering {
		_, err = tx.ExecContext(ctx, `UPDATE alerts SET last_observed_at=?,observed_value=? WHERE dedup_key=? AND status='firing'`, now.Unix(), value, key)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func suppressedTx(ctx context.Context, tx *sql.Tx, rule Rule, target string, now time.Time) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM silences s WHERE s.starts_at<=? AND s.ends_at>? AND ((s.scope_type='server') OR (s.scope_type='resource' AND s.scope_id=?) OR (s.scope_type='rule' AND s.scope_id=?) OR (s.scope_type='project' AND EXISTS(SELECT 1 FROM resources r WHERE r.id=? AND r.project_name=s.scope_id)))`, now.Unix(), now.Unix(), target, rule.ID, target).Scan(&n)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	if rule.SuppressDuringDeployment {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM deployment_grace_periods WHERE resource_id=? AND starts_at<=? AND ends_at>?`, target, now.Unix(), now.Unix()).Scan(&n)
	}
	return n > 0, err
}
func insertEvent(ctx context.Context, tx *sql.Tx, eventID string, rule Rule, target, eventType string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO events(id,ts,resource_id,type,severity,summary,correlation_key,source,created_at)VALUES(?,?,?,?,?,?,?,?,?)`, "alert-"+eventID, now.UnixMilli(), nullableTarget(target), eventType, rule.Severity, message(rule, target), dedup(rule.Family, target), "alerts", now.UnixMilli())
	return err
}
func nullableTarget(v string) any {
	if v == "server" || v == "docker" {
		return nil
	}
	return v
}
func timePtr(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.Unix()
}
func (e *Evaluator) DeploymentGrace(ctx context.Context, resource, confidence string, at time.Time) error {
	if confidence != "confirmed" && confidence != "likely" {
		return fmt.Errorf("invalid deployment confidence")
	}
	_, err := e.Repo.db.ExecContext(ctx, `INSERT INTO deployment_grace_periods(resource_id,starts_at,ends_at,confidence)VALUES(?,?,?,?) ON CONFLICT(resource_id) DO UPDATE SET starts_at=excluded.starts_at,ends_at=excluded.ends_at,confidence=excluded.confidence`, resource, at.Unix(), at.Add(5*time.Minute).Unix(), confidence)
	return err
}
