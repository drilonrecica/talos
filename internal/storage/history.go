// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"fmt"
	"time"
)

type Metric string

const (
	MetricCPU        Metric = "cpu"
	MetricMemory     Metric = "memory"
	MetricNetworkRX  Metric = "network_rx"
	MetricNetworkTX  Metric = "network_tx"
	MetricBlockRead  Metric = "block_read"
	MetricBlockWrite Metric = "block_write"
)

type Resolution string

const (
	ResolutionRaw Resolution = "10s"
	Resolution1m  Resolution = "1m"
	Resolution15m Resolution = "15m"
	Resolution1h  Resolution = "1h"
)

type MetricQuery struct {
	Scope, ID string
	Metrics   []Metric
	From, To  time.Time
}
type Point struct {
	At    time.Time `json:"at"`
	Min   *float64  `json:"min"`
	Avg   *float64  `json:"avg"`
	Max   *float64  `json:"max"`
	Count int       `json:"count"`
}
type Series struct {
	Metric Metric  `json:"metric"`
	Unit   string  `json:"unit"`
	Points []Point `json:"points"`
}
type Gap struct {
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Reason string    `json:"reason"`
}
type MetricsResponse struct {
	Scope, ID  string
	From, To   time.Time
	Resolution Resolution `json:"resolution"`
	Series     []Series   `json:"series"`
	Gaps       []Gap      `json:"gaps"`
}

func (q MetricQuery) Validate() error {
	if q.Scope != "host" && q.Scope != "resource" {
		return fmt.Errorf("invalid scope")
	}
	if q.Scope == "host" && q.ID != "" {
		return fmt.Errorf("host metrics do not accept id")
	}
	if q.Scope == "resource" && q.ID == "" {
		return fmt.Errorf("resource id is required")
	}
	if q.From.IsZero() || q.To.IsZero() || !q.From.Before(q.To) || q.To.Sub(q.From) > 30*24*time.Hour {
		return fmt.Errorf("invalid time range")
	}
	if len(q.Metrics) == 0 || len(q.Metrics) > 6 {
		return fmt.Errorf("invalid metric count")
	}
	seen := map[Metric]bool{}
	for _, metric := range q.Metrics {
		if seen[metric] || !metricAllowed(q.Scope, metric) {
			return fmt.Errorf("invalid metric %q", metric)
		}
		seen[metric] = true
	}
	return nil
}
func metricAllowed(scope string, metric Metric) bool {
	if scope == "host" {
		return metric == MetricCPU || metric == MetricMemory || metric == MetricNetworkRX || metric == MetricNetworkTX
	}
	return metric == MetricCPU || metric == MetricMemory || metric == MetricNetworkRX || metric == MetricNetworkTX || metric == MetricBlockRead || metric == MetricBlockWrite
}
func selectResolution(d time.Duration) Resolution {
	switch {
	case d <= 2*time.Hour:
		return ResolutionRaw
	case d <= 1000*time.Minute:
		return Resolution1m
	case d <= 1000*15*time.Minute:
		return Resolution15m
	default:
		return Resolution1h
	}
}
func metricUnit(metric Metric) string {
	if metric == MetricCPU {
		return "percent"
	}
	if metric == MetricMemory {
		return "bytes"
	}
	return "bytes_per_second"
}

func (m *Manager) QueryMetrics(ctx context.Context, q MetricQuery) (MetricsResponse, error) {
	if err := q.Validate(); err != nil {
		return MetricsResponse{}, err
	}
	r := MetricsResponse{Scope: q.Scope, ID: q.ID, From: q.From.UTC(), To: q.To.UTC(), Resolution: selectResolution(q.To.Sub(q.From))}
	for index, metric := range q.Metrics {
		points, err := m.metricPoints(ctx, q, metric, r.Resolution)
		if err != nil {
			return MetricsResponse{}, err
		}
		r.Series = append(r.Series, Series{Metric: metric, Unit: metricUnit(metric), Points: points})
		if index == 0 {
			gaps := findGaps(q.From, q.To, r.Resolution, points)
			for i := range gaps {
				gaps[i].Reason = m.classifyGap(ctx, q, gaps[i])
			}
			r.Gaps = gaps
		}
	}
	return r, nil
}

func (m *Manager) classifyGap(ctx context.Context, q MetricQuery, gap Gap) string {
	var count int
	if q.Scope == "resource" {
		_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource_samples_10s WHERE resource_id=? AND ts>=? AND ts<? AND (active_instance_count=0 OR status IN ('paused','archived'))`, q.ID, gap.From.UnixMilli(), gap.To.UnixMilli()).Scan(&count)
		if count > 0 {
			return "inactive"
		}
	}
	_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM collector_state_events WHERE ts>=? AND ts<? AND new_state IN ('degraded','down')`, gap.From.UnixMilli(), gap.To.UnixMilli()).Scan(&count)
	if count > 0 {
		return "collector_unavailable"
	}
	_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE ts>=? AND ts<? AND type IN ('persistence_degraded','persistence_gap')`, gap.From.UnixMilli(), gap.To.UnixMilli()).Scan(&count)
	if count > 0 {
		return "persistence_failure"
	}
	return "missing"
}
func (m *Manager) metricPoints(ctx context.Context, q MetricQuery, metric Metric, res Resolution) ([]Point, error) {
	column, table, err := metricSource(q.Scope, metric, res)
	if err != nil {
		return nil, err
	}
	var query string
	args := []any{}
	if res == ResolutionRaw {
		query = "SELECT ts," + column + " FROM " + table + " WHERE ts>=? AND ts<=?"
		args = []any{q.From.UnixMilli(), q.To.UnixMilli()}
		if q.Scope == "resource" {
			query += " AND resource_id=?"
			args = append(args, q.ID)
		}
		query += " ORDER BY ts"
	} else {
		query = "SELECT ts," + column + "," + column + "," + column + ",sample_count FROM " + table + " WHERE ts>=? AND ts<=?"
		args = []any{q.From.UnixMilli(), q.To.UnixMilli()}
		if q.Scope == "resource" {
			query += " AND resource_id=?"
			args = append(args, q.ID)
		}
		query += " ORDER BY ts"
	}
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Point{}
	for rows.Next() {
		var ms int64
		var p Point
		if res == ResolutionRaw {
			if err = rows.Scan(&ms, &p.Avg); err != nil {
				return nil, err
			}
			p.Min = p.Avg
			p.Max = p.Avg
			if p.Avg != nil {
				p.Count = 1
			}
		} else {
			if err = rows.Scan(&ms, &p.Min, &p.Avg, &p.Max, &p.Count); err != nil {
				return nil, err
			}
		}
		p.At = time.UnixMilli(ms).UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}
func metricSource(scope string, metric Metric, res Resolution) (string, string, error) {
	raw := map[Metric]string{MetricCPU: "cpu_busy_pct", MetricMemory: "memory_used_bytes", MetricNetworkRX: "network_rx_bps", MetricNetworkTX: "network_tx_bps", MetricBlockRead: "block_read_bps", MetricBlockWrite: "block_write_bps"}
	roll := map[Metric]string{MetricCPU: "cpu_avg", MetricMemory: "memory_avg", MetricNetworkRX: "network_rx_avg", MetricNetworkTX: "network_tx_avg", MetricBlockRead: "block_read_avg", MetricBlockWrite: "block_write_avg"}
	column := raw[metric]
	if res != ResolutionRaw {
		column = roll[metric]
	}
	if column == "" {
		return "", "", fmt.Errorf("unsupported metric")
	}
	if scope == "host" {
		if res == ResolutionRaw {
			return column, "host_samples_10s", nil
		}
		return column, "host_rollups_" + string(res), nil
	}
	if res == ResolutionRaw {
		return column, "resource_samples_10s", nil
	}
	return column, "resource_rollups_" + string(res), nil
}
func findGaps(from, to time.Time, res Resolution, points []Point) []Gap {
	step := map[Resolution]time.Duration{ResolutionRaw: 10 * time.Second, Resolution1m: time.Minute, Resolution15m: 15 * time.Minute, Resolution1h: time.Hour}[res]
	if step == 0 {
		return nil
	}
	out := []Gap{}
	cursor := from.UTC()
	for _, p := range points {
		if p.At.Sub(cursor) > step*2 {
			out = append(out, Gap{From: cursor, To: p.At, Reason: "missing"})
		}
		cursor = p.At.Add(step)
	}
	if to.UTC().Sub(cursor) > step*2 {
		out = append(out, Gap{From: cursor, To: to.UTC(), Reason: "missing"})
	}
	return out
}

func (m *Manager) HostCPU(ctx context.Context, from, to time.Time, limit int) ([]Point, error) {
	r, err := m.QueryMetrics(ctx, MetricQuery{Scope: "host", Metrics: []Metric{MetricCPU}, From: from, To: to})
	if err != nil {
		return nil, err
	}
	if len(r.Series) == 0 {
		return nil, nil
	}
	if limit > 0 && len(r.Series[0].Points) > limit {
		return r.Series[0].Points[:limit], nil
	}
	return r.Series[0].Points, nil
}
