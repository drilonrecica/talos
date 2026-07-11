// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"time"
)

type HistoricalEvent struct {
	ID       string    `json:"id"`
	At       time.Time `json:"ts"`
	Type     string    `json:"type"`
	Severity string    `json:"severity"`
	Summary  string    `json:"summary"`
	Source   string    `json:"source"`
}

func (m *Manager) Events(ctx context.Context, from, to time.Time, limit int) ([]HistoricalEvent, error) {
	return m.EventsFor(ctx, from, to, limit, "")
}
func (m *Manager) EventsFor(ctx context.Context, from, to time.Time, limit int, resourceID string) ([]HistoricalEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	query := "SELECT id,ts,type,severity,summary,source FROM events WHERE ts>=? AND ts<=?"
	args := []any{from.UnixMilli(), to.UnixMilli()}
	if resourceID != "" {
		query += " AND resource_id=?"
		args = append(args, resourceID)
	}
	query += " ORDER BY ts DESC,id DESC LIMIT ?"
	args = append(args, limit)
	rows, e := m.db.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []HistoricalEvent{}
	for rows.Next() {
		var v HistoricalEvent
		var ms int64
		if e = rows.Scan(&v.ID, &ms, &v.Type, &v.Severity, &v.Summary, &v.Source); e != nil {
			return nil, e
		}
		v.At = time.UnixMilli(ms).UTC()
		out = append(out, v)
	}
	return out, rows.Err()
}
