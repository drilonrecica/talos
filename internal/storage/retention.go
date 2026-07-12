// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"os"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
)

// SetRetention updates the automatic retention policy at runtime.
func (m *Manager) SetRetention(r RetentionCutoffs) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retention = r
}

// SetBudgetTarget updates the soft storage budget target.
func (m *Manager) SetBudgetTarget(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budget.TargetBytes = bytes
}

// EmergencyPause reports whether raw persistence should pause because the
// database volume reached the emergency budget threshold.
func (m *Manager) EmergencyPause() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.emergencyPause
}

// Budget returns the latest evaluated budget state and used bytes.
func (m *Manager) Budget() DatabaseBudget {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.budget
}

func (m *Manager) runRetentionAndBudget(ctx context.Context) {
	// Evaluate budget frequently; run retention hourly.
	budgetTick := time.NewTicker(time.Minute)
	defer budgetTick.Stop()
	retentionTick := time.NewTicker(time.Hour)
	defer retentionTick.Stop()
	m.evaluateBudgetAndRetention(ctx, true)
	for {
		select {
		case <-ctx.Done():
			return
		case <-budgetTick.C:
			m.evaluateBudgetAndRetention(ctx, false)
		case <-retentionTick.C:
			m.evaluateBudgetAndRetention(ctx, true)
		}
	}
}

func (m *Manager) evaluateBudgetAndRetention(ctx context.Context, runRetention bool) {
	used, err := m.databaseSize()
	if err != nil {
		return
	}
	m.mu.Lock()
	target := m.budget.TargetBytes
	state := EvaluateBudget(used, target, 0.80, 0.95, 0.98)
	m.budget = DatabaseBudget{UsedBytes: used, TargetBytes: target, State: state}
	wasPaused := m.emergencyPause
	m.emergencyPause = state == BudgetEmergency
	retention := m.retention
	m.mu.Unlock()

	if wasPaused != m.emergencyPause {
		// Log state transitions outside the lock.
		if m.emergencyPause {
			_ = m.WriteEvent(ctx, metrics.Event{At: time.Now().UTC(), Type: "persistence_emergency", Severity: "critical", Message: "Raw history persistence paused; database volume reached emergency threshold"})
		} else {
			_ = m.WriteEvent(ctx, metrics.Event{At: time.Now().UTC(), Type: "persistence_resumed", Severity: "info", Message: "Raw history persistence resumed; database volume below emergency threshold"})
		}
	}

	if !runRetention {
		return
	}
	// At critical or emergency, run retention more aggressively.
	aggressive := state == BudgetCritical || state == BudgetEmergency
	_ = m.applyRetention(ctx, retention, aggressive)
}

func (m *Manager) databaseSize() (int64, error) {
	var total int64
	for _, name := range []string{m.Path, m.Path + "-wal", m.Path + "-shm"} {
		info, err := os.Stat(name)
		if err == nil {
			total += info.Size()
		}
	}
	return total, nil
}

func (m *Manager) applyRetention(ctx context.Context, r RetentionCutoffs, aggressive bool) error {
	if r.Raw == 0 {
		return nil
	}
	now := time.Now().UTC()
	cutoffs := map[string]time.Duration{
		"host_samples_10s":               r.Raw,
		"resource_samples_10s":           r.Raw,
		"container_instance_samples_10s": r.Raw,
		"filesystem_samples_1m":          r.Raw,
		"network_interface_samples_1m":   r.Raw,
		"events":                         r.Raw,
		"collector_state_events":         r.Raw,
		"host_rollups_1m":                r.OneMinute,
		"resource_rollups_1m":            r.OneMinute,
		"host_rollups_15m":               r.FifteenMinute,
		"resource_rollups_15m":           r.FifteenMinute,
		"host_rollups_1h":                r.OneHour,
		"resource_rollups_1h":            r.OneHour,
	}
	limit := 500
	if aggressive {
		limit = 2000
	}
	for table, duration := range cutoffs {
		if duration <= 0 {
			continue
		}
		cutoff := now.Add(-duration).UnixMilli()
		for {
			var n int64
			err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE ts<?", cutoff).Scan(&n)
			if err != nil || n == 0 {
				break
			}
			_, err = m.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE rowid IN (SELECT rowid FROM "+table+" WHERE ts<? LIMIT ?)", cutoff, limit)
			if err != nil {
				return err
			}
			if aggressive {
				// Yield briefly to avoid long write locks during aggressive cleanup.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
	}
	// Vacuum only when there is meaningful free space and not during aggressive runs,
	// to avoid blocking the database during an emergency.
	if !aggressive {
		_ = m.vacuumIfWorthwhile(ctx)
	}
	return nil
}

func (m *Manager) vacuumIfWorthwhile(ctx context.Context) error {
	var pages, freelist int
	if err := m.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		return err
	}
	if err := m.db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freelist); err != nil {
		return err
	}
	if pages == 0 || float64(freelist)/float64(pages) < 0.20 {
		return nil
	}
	_, err := m.db.ExecContext(ctx, "VACUUM")
	return err
}
