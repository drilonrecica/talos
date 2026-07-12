// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMetricQueryValidationAndResolution(t *testing.T) {
	q := MetricQuery{Scope: "resource", ID: "res_test", Metrics: []Metric{MetricCPU}, From: time.Now().Add(-time.Hour), To: time.Now()}
	if err := q.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := selectResolution(30 * 24 * time.Hour); got != Resolution1h {
		t.Fatalf("resolution=%s", got)
	}
	q.Metrics = []Metric{MetricBlockRead}
	q.Scope = "host"
	if err := q.Validate(); err == nil {
		t.Fatal("host block metric accepted")
	}
}

func TestGapClassification(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	ms := base.UnixMilli()
	if _, err := m.db.ExecContext(ctx, "INSERT INTO resource_samples_10s(ts,resource_id,active_instance_count,status) VALUES(?,'res_test',0,'paused')", ms+1000); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(ctx, "INSERT INTO collector_state_events(id,ts,collector_name,new_state) VALUES('collector',?,'docker','down')", ms+61_000); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(ctx, "INSERT INTO events(id,ts,type,severity,summary,source,created_at) VALUES('persistence',?,'persistence_gap','warning','gap','binnacle',?)", ms+121_000, ms+121_000); err != nil {
		t.Fatal(err)
	}
	q := MetricQuery{Scope: "resource", ID: "res_test"}
	cases := []struct {
		from time.Time
		want string
	}{{base, "inactive"}, {base.Add(time.Minute), "collector_unavailable"}, {base.Add(2 * time.Minute), "persistence_failure"}}
	for _, tc := range cases {
		gap := Gap{From: tc.from, To: tc.from.Add(time.Minute)}
		if got := m.classifyGap(ctx, q, gap); got != tc.want {
			t.Fatalf("gap %s=%s want %s", tc.from, got, tc.want)
		}
	}
}

func TestNullBucketsBecomeExplicitMergedGaps(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	gaps := findGaps(base, base.Add(40*time.Second), ResolutionRaw, []Point{{At: base, Avg: nil}, {At: base.Add(10 * time.Second), Avg: nil}, {At: base.Add(20 * time.Second), Avg: floatPtr(2)}})
	for i := range gaps {
		gaps[i].Reason = "inactive"
	}
	merged := mergeGaps(gaps)
	if len(merged) != 1 || !merged[0].From.Equal(base) || !merged[0].To.Equal(base.Add(20*time.Second)) {
		t.Fatalf("gaps=%+v", merged)
	}
}

func TestHostMetricRollupSourceForBroadenedTelemetry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	for i := range 2 {
		ts := base.Add(time.Duration(i) * 10 * time.Second).UnixMilli()
		if _, err := m.db.ExecContext(ctx, `INSERT INTO host_samples_10s(
			ts,host_id,cpu_busy_pct,cpu_user_pct,cpu_system_pct,cpu_iowait_pct,cpu_steal_pct,
			load_1,load_5,load_15,memory_used_bytes,swap_used_bytes,
			disk_read_bps,disk_write_bps,disk_read_iops,disk_write_iops,
			network_rx_bps,network_tx_bps
		) VALUES(?,'host',1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1)`, ts); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RollupOnce(ctx, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, metric := range []Metric{MetricCPUUser, MetricCPUSystem, MetricCPUIOWait, MetricCPUSteal, MetricLoad1, MetricLoad5, MetricLoad15, MetricSwap, MetricDiskRead, MetricDiskWrite, MetricDiskIOPS} {
		resp, err := m.QueryMetrics(ctx, MetricQuery{Scope: "host", Metrics: []Metric{metric}, From: base, To: base.Add(3 * time.Hour)})
		if err != nil {
			t.Fatalf("%s: %v", metric, err)
		}
		if len(resp.Series) != 1 || len(resp.Series[0].Points) == 0 {
			t.Fatalf("%s: expected one non-empty series, got %+v", metric, resp.Series)
		}
		p := resp.Series[0].Points[0]
		wantAvg := 1.0
		if metric == MetricDiskIOPS {
			wantAvg = 2.0
		}
		if p.Avg == nil || *p.Avg != wantAvg || p.Count != 2 {
			t.Fatalf("%s: avg=%v count=%d", metric, p.Avg, p.Count)
		}
	}
}

func TestRawResourceMetricSources(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := New(filepath.Join(dir, "binnacle.db"), filepath.Join(dir, "run"))
	if err := m.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	if _, err := m.db.ExecContext(ctx, `INSERT INTO resource_samples_10s(
		ts,resource_id,cpu_host_pct,memory_working_set_bytes,network_rx_bps,
		network_tx_bps,block_read_bps,block_write_bps,active_instance_count,status
	) VALUES(?,'res_test',1,2,3,4,5,6,1,'healthy')`, base.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	metrics := []Metric{MetricCPU, MetricMemory, MetricNetworkRX, MetricNetworkTX, MetricBlockRead, MetricBlockWrite}
	response, err := m.QueryMetrics(ctx, MetricQuery{Scope: "resource", ID: "res_test", Metrics: metrics, From: base.Add(-time.Minute), To: base.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if response.Resolution != ResolutionRaw || len(response.Series) != len(metrics) {
		t.Fatalf("response=%+v", response)
	}
	for index, series := range response.Series {
		if len(series.Points) != 1 || series.Points[0].Avg == nil || *series.Points[0].Avg != float64(index+1) {
			t.Fatalf("metric %s points=%+v", series.Metric, series.Points)
		}
	}
}

func TestMetricsResponseJSONContract(t *testing.T) {
	encoded, err := json.Marshal(MetricsResponse{Scope: "host", From: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC), To: time.Date(2026, 7, 11, 13, 0, 0, 0, time.UTC), Resolution: ResolutionRaw, Series: []Series{}, Gaps: []Gap{}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, field := range []string{`"scope":"host"`, `"from":"2026-07-11T12:00:00Z"`, `"to":"2026-07-11T13:00:00Z"`, `"series":[]`, `"gaps":[]`} {
		if !strings.Contains(text, field) {
			t.Fatalf("json=%s missing %s", text, field)
		}
	}
}
func floatPtr(v float64) *float64 { return &v }
