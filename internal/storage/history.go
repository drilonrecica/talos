// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"fmt"
	"time"
)

type Metric string

const (
	MetricCPU            Metric = "cpu"
	MetricCPUUser        Metric = "cpu_user"
	MetricCPUSystem      Metric = "cpu_system"
	MetricCPUIOWait      Metric = "cpu_iowait"
	MetricCPUSteal       Metric = "cpu_steal"
	MetricMemory         Metric = "memory"
	MetricSwap           Metric = "swap"
	MetricLoad1          Metric = "load_1"
	MetricLoad5          Metric = "load_5"
	MetricLoad15         Metric = "load_15"
	MetricNetworkRX      Metric = "network_rx"
	MetricNetworkTX      Metric = "network_tx"
	MetricNetworkPackets Metric = "network_packets"
	MetricDiskRead       Metric = "disk_read"
	MetricDiskWrite      Metric = "disk_write"
	MetricDiskIOPS       Metric = "disk_iops"
	MetricBlockRead      Metric = "block_read"
	MetricBlockWrite     Metric = "block_write"
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
	Scope      string     `json:"scope"`
	ID         string     `json:"id,omitempty"`
	From       time.Time  `json:"from"`
	To         time.Time  `json:"to"`
	Resolution Resolution `json:"resolution"`
	Series     []Series   `json:"series"`
	Gaps       []Gap      `json:"gaps"`
}

func (m *Manager) HasMetricHistory(ctx context.Context) (bool, error) {
	var exists bool
	err := m.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM host_samples_10s LIMIT 1)").Scan(&exists)
	return exists, err
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

var hostMetrics = map[Metric]bool{
	MetricCPU: true, MetricCPUUser: true, MetricCPUSystem: true, MetricCPUIOWait: true, MetricCPUSteal: true,
	MetricMemory: true, MetricSwap: true,
	MetricLoad1: true, MetricLoad5: true, MetricLoad15: true,
	MetricNetworkRX: true, MetricNetworkTX: true,
	MetricDiskRead: true, MetricDiskWrite: true, MetricDiskIOPS: true,
}
var resourceMetrics = map[Metric]bool{
	MetricCPU: true, MetricMemory: true, MetricNetworkRX: true, MetricNetworkTX: true, MetricBlockRead: true, MetricBlockWrite: true,
}

func metricAllowed(scope string, metric Metric) bool {
	if scope == "host" {
		return hostMetrics[metric]
	}
	return resourceMetrics[metric]
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
	switch metric {
	case MetricCPU, MetricCPUUser, MetricCPUSystem, MetricCPUIOWait, MetricCPUSteal:
		return "percent"
	case MetricMemory, MetricSwap:
		return "bytes"
	case MetricLoad1, MetricLoad5, MetricLoad15:
		return "load"
	case MetricDiskIOPS:
		return "ops_per_second"
	}
	return "bytes_per_second"
}

func (m *Manager) QueryMetrics(ctx context.Context, q MetricQuery) (MetricsResponse, error) {
	if err := q.Validate(); err != nil {
		return MetricsResponse{}, err
	}
	r := MetricsResponse{Scope: q.Scope, ID: q.ID, From: q.From.UTC(), To: q.To.UTC(), Resolution: selectResolution(q.To.Sub(q.From)), Gaps: []Gap{}}
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
			r.Gaps = mergeGaps(gaps)
		}
	}
	return r, nil
}

func (m *Manager) classifyGap(ctx context.Context, q MetricQuery, gap Gap) string {
	var count int
	_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE ts>=? AND ts<? AND type IN ('persistence_degraded','persistence_gap')`, gap.From.UnixMilli(), gap.To.UnixMilli()).Scan(&count)
	if count > 0 {
		return "persistence_failure"
	}
	collector := "host"
	if q.Scope == "resource" {
		collector = "docker"
	}
	var state string
	_ = m.db.QueryRowContext(ctx, "SELECT new_state FROM collector_state_events WHERE collector_name=? AND ts<=? ORDER BY ts DESC LIMIT 1", collector, gap.From.UnixMilli()).Scan(&state)
	if state == "degraded" || state == "down" {
		return "collector_unavailable"
	}
	_ = m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM collector_state_events WHERE collector_name=? AND ts>=? AND ts<? AND new_state IN ('degraded','down')`, collector, gap.From.UnixMilli(), gap.To.UnixMilli()).Scan(&count)
	if count > 0 {
		return "collector_unavailable"
	}
	if q.Scope == "resource" {
		var status string
		var active int
		_ = m.db.QueryRowContext(ctx, `SELECT COALESCE(status,''),active_instance_count FROM resource_samples_10s WHERE resource_id=? AND ts<? ORDER BY ts DESC LIMIT 1`, q.ID, gap.To.UnixMilli()).Scan(&status, &active)
		if active == 0 || status == "paused" || status == "archived" {
			return "inactive"
		}
	}
	return "missing"
}
func (m *Manager) metricPoints(ctx context.Context, q MetricQuery, metric Metric, res Resolution) ([]Point, error) {
	minColumn, avgColumn, maxColumn, countColumn, table, err := metricSource(q.Scope, metric, res)
	if err != nil {
		return nil, err
	}
	var query string
	args := []any{}
	if res == ResolutionRaw {
		query = "SELECT ts," + avgColumn + " FROM " + table + " WHERE ts>=? AND ts<=?"
		args = []any{q.From.UnixMilli(), q.To.UnixMilli()}
		if q.Scope == "resource" {
			query += " AND resource_id=?"
			args = append(args, q.ID)
		}
		query += " ORDER BY ts"
	} else {
		query = "SELECT ts," + minColumn + "," + avgColumn + "," + maxColumn + "," + countColumn + " FROM " + table + " WHERE ts>=? AND ts<=?"
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
func metricSource(scope string, metric Metric, res Resolution) (string, string, string, string, string, error) {
	raw := map[Metric]string{
		MetricCPU: "cpu_busy_pct", MetricCPUUser: "cpu_user_pct", MetricCPUSystem: "cpu_system_pct", MetricCPUIOWait: "cpu_iowait_pct", MetricCPUSteal: "cpu_steal_pct",
		MetricMemory: "memory_used_bytes", MetricSwap: "swap_used_bytes",
		MetricLoad1: "load_1", MetricLoad5: "load_5", MetricLoad15: "load_15",
		MetricNetworkRX: "network_rx_bps", MetricNetworkTX: "network_tx_bps",
		MetricDiskRead: "disk_read_bps", MetricDiskWrite: "disk_write_bps",
		MetricDiskIOPS:  "disk_read_iops + disk_write_iops",
		MetricBlockRead: "block_read_bps", MetricBlockWrite: "block_write_bps",
	}
	prefix := map[Metric]string{
		MetricCPU: "cpu", MetricCPUUser: "cpu_user", MetricCPUSystem: "cpu_system", MetricCPUIOWait: "cpu_iowait", MetricCPUSteal: "cpu_steal",
		MetricMemory: "memory", MetricSwap: "swap_used",
		MetricLoad1: "load_1", MetricLoad5: "load_5", MetricLoad15: "load_15",
		MetricNetworkRX: "network_rx", MetricNetworkTX: "network_tx",
		MetricDiskRead: "disk_read", MetricDiskWrite: "disk_write", MetricDiskIOPS: "disk_iops",
		MetricBlockRead: "block_read", MetricBlockWrite: "block_write",
	}[metric]
	if raw[metric] == "" {
		return "", "", "", "", "", fmt.Errorf("unsupported metric")
	}
	if res == ResolutionRaw {
		table := "resource_samples_10s"
		if scope == "host" {
			table = "host_samples_10s"
		}
		column := raw[metric]
		if scope == "resource" {
			switch metric {
			case MetricCPU:
				column = "cpu_host_pct"
			case MetricMemory:
				column = "memory_working_set_bytes"
			}
		}
		return column, column, column, "", table, nil
	}
	if prefix == "" {
		return "", "", "", "", "", fmt.Errorf("unsupported metric for rollup resolution")
	}
	count := prefix + "_count"
	if metric == MetricCPU {
		count = "sample_count"
	}
	tablePrefix := "resource_rollups_"
	if scope == "host" {
		tablePrefix = "host_rollups_"
	}
	return prefix + "_min", prefix + "_avg", prefix + "_max", count, tablePrefix + string(res), nil
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
		if p.Avg == nil {
			out = append(out, Gap{From: p.At, To: p.At.Add(step), Reason: "missing"})
		}
		cursor = p.At.Add(step)
	}
	if to.UTC().Sub(cursor) > step*2 {
		out = append(out, Gap{From: cursor, To: to.UTC(), Reason: "missing"})
	}
	return out
}

func mergeGaps(gaps []Gap) []Gap {
	if len(gaps) < 2 {
		return gaps
	}
	out := []Gap{gaps[0]}
	for _, gap := range gaps[1:] {
		last := &out[len(out)-1]
		if gap.Reason == last.Reason && !gap.From.After(last.To) {
			if gap.To.After(last.To) {
				last.To = gap.To
			}
			continue
		}
		out = append(out, gap)
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
