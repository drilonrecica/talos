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
	m := New(filepath.Join(dir, "talos.db"), filepath.Join(dir, "run"))
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
	if _, err := m.db.ExecContext(ctx, "INSERT INTO events(id,ts,type,severity,summary,source,created_at) VALUES('persistence',?,'persistence_gap','warning','gap','talos',?)", ms+121_000, ms+121_000); err != nil {
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
