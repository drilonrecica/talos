// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"database/sql"
	"time"
)

type HistoricalEvent struct {
	ID                string    `json:"id"`
	At                time.Time `json:"ts"`
	Type              string    `json:"type"`
	Severity          string    `json:"severity"`
	Summary           string    `json:"summary"`
	Details           *string   `json:"details,omitempty"`
	CorrelationKey    *string   `json:"correlationKey,omitempty"`
	ContainerInstance *string   `json:"containerInstanceId,omitempty"`
	ResourceID        *string   `json:"resourceId,omitempty"`
	Source            string    `json:"source"`
}

func (m *Manager) Events(ctx context.Context, from, to time.Time, limit int) ([]HistoricalEvent, error) {
	return m.EventsFor(ctx, from, to, limit, "")
}
func (m *Manager) EventsFor(ctx context.Context, from, to time.Time, limit int, resourceID string) ([]HistoricalEvent, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	query := "SELECT id,ts,type,severity,summary,details_json,correlation_key,container_instance_id,resource_id,source FROM events WHERE ts>=? AND ts<=?"
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
		var details, correlation, containerInstance, resourceIDVal sql.NullString
		if e = rows.Scan(&v.ID, &ms, &v.Type, &v.Severity, &v.Summary, &details, &correlation, &containerInstance, &resourceIDVal, &v.Source); e != nil {
			return nil, e
		}
		v.At = time.UnixMilli(ms).UTC()
		if details.Valid {
			v.Details = &details.String
		}
		if correlation.Valid {
			v.CorrelationKey = &correlation.String
		}
		if containerInstance.Valid {
			v.ContainerInstance = &containerInstance.String
		}
		if resourceIDVal.Valid {
			v.ResourceID = &resourceIDVal.String
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *Manager) ExportEvents(ctx context.Context, from, to time.Time, limit int) ([]HistoricalEvent, error) {
	if limit < 1 || limit > 10001 {
		limit = 10001
	}
	rows, e := m.db.QueryContext(ctx, "SELECT id,ts,type,severity,summary,details_json,correlation_key,container_instance_id,resource_id,source FROM events WHERE ts>=? AND ts<=? ORDER BY ts,id LIMIT ?", from.UnixMilli(), to.UnixMilli(), limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []HistoricalEvent{}
	for rows.Next() {
		var v HistoricalEvent
		var ms int64
		var details, correlation, container, resource sql.NullString
		if e = rows.Scan(&v.ID, &ms, &v.Type, &v.Severity, &v.Summary, &details, &correlation, &container, &resource, &v.Source); e != nil {
			return nil, e
		}
		v.At = time.UnixMilli(ms).UTC()
		if details.Valid {
			v.Details = &details.String
		}
		if correlation.Valid {
			v.CorrelationKey = &correlation.String
		}
		if container.Valid {
			v.ContainerInstance = &container.String
		}
		if resource.Valid {
			v.ResourceID = &resource.String
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
