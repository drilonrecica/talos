// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestRollupsPreserveTypedStatistics(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	bucket := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	for i, value := range []float64{10, 30} {
		ts := bucket.Add(time.Duration(i) * 10 * time.Second).UnixMilli()
		if _, err := m.db.ExecContext(ctx, "INSERT INTO host_samples_10s(ts,host_id,memory_used_bytes,network_rx_bps) VALUES(?,'host',?,?)", ts, int64(value), value); err != nil {
			t.Fatal(err)
		}
		if _, err := m.db.ExecContext(ctx, "INSERT INTO resource_samples_10s(ts,resource_id,memory_working_set_bytes,block_read_bps,active_instance_count) VALUES(?,'res_test',?,?,1)", ts, int64(value), value); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RollupOnce(ctx, bucket.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var min, avg, max float64
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT memory_min,memory_avg,memory_max,memory_count FROM host_rollups_1m WHERE ts=?", bucket.UnixMilli()).Scan(&min, &avg, &max, &count); err != nil {
		t.Fatal(err)
	}
	if min != 10 || avg != 20 || max != 30 || count != 2 {
		t.Fatalf("host min=%v avg=%v max=%v count=%d", min, avg, max, count)
	}
	if err := m.db.QueryRowContext(ctx, "SELECT block_read_min,block_read_avg,block_read_max,block_read_count FROM resource_rollups_1m WHERE resource_id='res_test' AND ts=?", bucket.UnixMilli()).Scan(&min, &avg, &max, &count); err != nil {
		t.Fatal(err)
	}
	if min != 10 || avg != 20 || max != 30 || count != 2 {
		t.Fatalf("resource min=%v avg=%v max=%v count=%d", min, avg, max, count)
	}
}

func TestRollupsIncludeBroadenedHostTelemetry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	bucket := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	for i, value := range []float64{10, 30} {
		ts := bucket.Add(time.Duration(i) * 10 * time.Second).UnixMilli()
		if _, err := m.db.ExecContext(ctx, `INSERT INTO host_samples_10s(
			ts,host_id,cpu_busy_pct,cpu_user_pct,cpu_system_pct,cpu_iowait_pct,cpu_steal_pct,
			load_1,load_5,load_15,memory_used_bytes,swap_used_bytes,
			disk_read_bps,disk_write_bps,disk_read_iops,disk_write_iops
		) VALUES(?,'host',?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			ts, value, value, value, value, value, value, value, value, int64(value), int64(value), value, value, value, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RollupOnce(ctx, bucket.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var min, avg, max float64
	var count int
	for _, tc := range []struct {
		col       string
		wantMin   float64
		wantAvg   float64
		wantMax   float64
		wantCount int
	}{
		{"cpu_user", 10, 20, 30, 2}, {"cpu_system", 10, 20, 30, 2}, {"cpu_iowait", 10, 20, 30, 2}, {"cpu_steal", 10, 20, 30, 2},
		{"load_1", 10, 20, 30, 2}, {"load_5", 10, 20, 30, 2}, {"load_15", 10, 20, 30, 2}, {"swap_used", 10, 20, 30, 2},
		{"disk_read", 10, 20, 30, 2}, {"disk_write", 10, 20, 30, 2},
		{"disk_iops", 20, 40, 60, 2},
	} {
		if err := m.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s_min,%s_avg,%s_max,%s_count FROM host_rollups_1m WHERE ts=?", tc.col, tc.col, tc.col, tc.col), bucket.UnixMilli()).Scan(&min, &avg, &max, &count); err != nil {
			t.Fatalf("%s: %v", tc.col, err)
		}
		if min != tc.wantMin || avg != tc.wantAvg || max != tc.wantMax || count != tc.wantCount {
			t.Fatalf("%s min=%v avg=%v max=%v count=%d", tc.col, min, avg, max, count)
		}
	}
}
