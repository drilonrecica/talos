// SPDX-License-Identifier: AGPL-3.0-only
package diagnostics

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
	"github.com/drilonrecica/binnacle/internal/storage"
)

type MonitorMetric struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Value  any    `json:"value"`
	Unit   string `json:"unit,omitempty"`
	Status string `json:"status"`
	Help   string `json:"help"`
}
type MonitorSnapshot struct {
	At      time.Time       `json:"at"`
	Metrics []MonitorMetric `json:"metrics"`
}
type PersistenceMetrics interface {
	Queue() int64
	DroppedCount() uint64
	WriteLatency() time.Duration
}
type DurationProvider interface{ CollectionDuration() time.Duration }

type Monitor struct {
	DatabasePath   string
	DatabaseTarget int64
	QueueCapacity  int
	Engine         *metrics.Engine
	Persistence    PersistenceMetrics
	Collector      DurationProvider
	mu             sync.Mutex
	previousCPU    time.Duration
	previousAt     time.Time
	readCPU        func() time.Duration
}

func (m *Monitor) Snapshot() MonitorSnapshot {
	now := time.Now().UTC()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	rss, cpu := processRSS(), m.processCPU(now)
	dbSize, walSize := fileSize(m.DatabasePath), fileSize(m.DatabasePath+"-wal")
	queue, dropped, writeLatency := int64(0), uint64(0), time.Duration(0)
	if m.Persistence != nil {
		queue, dropped, writeLatency = m.Persistence.Queue(), m.Persistence.DroppedCount(), m.Persistence.WriteLatency()
	}
	collectionDuration := time.Duration(0)
	if m.Collector != nil {
		collectionDuration = m.Collector.CollectionDuration()
	}
	dockerStatus := "unavailable"
	if m.Engine != nil {
		if value, ok := m.Engine.Snapshot().Collectors["docker"]; ok {
			dockerStatus = string(value.State)
		}
	}
	values := []MonitorMetric{
		metric("cpu", "Binnacle CPU", cpu, "percent", statusNumber(cpu, 5, 20), "Process CPU over the latest interval."),
		metric("rss", "Resident memory", rss, "bytes", available(rss), "Resident working memory reported by Linux."),
		metric("heap", "Go heap", int64(memory.HeapAlloc), "bytes", "normal", "Currently allocated Go heap."),
		metric("goroutines", "Goroutines", runtime.NumGoroutine(), "count", "normal", "Active Go goroutines."),
		metric("database", "SQLite database", dbSize, "bytes", budgetStatus(dbSize, m.DatabaseTarget), "Main SQLite file size."),
		metric("wal", "SQLite WAL", walSize, "bytes", available(walSize), "Write-ahead log size."),
		metric("queue", "Persistence queue", queue, "batches", queueStatus(queue, m.QueueCapacity), "Queued persistence batches."),
		metric("dropped", "Dropped batches", int64(dropped), "batches", nonzeroStatus(dropped), "History batches dropped after queue overflow."),
		metric("write_latency", "Persistence write latency", durationValue(writeLatency), "milliseconds", durationStatus(writeLatency, 50*time.Millisecond), "Latest SQLite persistence operation."),
		metric("rollup_duration", "Rollup duration", nil, "milliseconds", "unavailable", "Not currently instrumented by the rollup worker."),
		metric("retention_duration", "Retention duration", nil, "milliseconds", "unavailable", "Not currently instrumented by the retention worker."),
		metric("collection_duration", "Collector duration", durationValue(collectionDuration), "milliseconds", durationStatus(collectionDuration, 2*time.Second), "Latest host and Docker collection cycle."),
		metric("sse_clients", "SSE clients", sseClients(m.Engine), "clients", "normal", "Currently connected live clients."),
		metric("docker", "Docker API health", dockerStatus, "", dockerStatusValue(dockerStatus), "Current Docker collector state."),
	}
	return MonitorSnapshot{At: now, Metrics: values}
}

func (m *Monitor) processCPU(now time.Time) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	read := m.readCPU
	if read == nil {
		read = processCPUTime
	}
	current := read()
	if m.previousAt.IsZero() {
		m.previousAt, m.previousCPU = now, current
		return nil
	}
	elapsed, delta := now.Sub(m.previousAt), current-m.previousCPU
	m.previousAt, m.previousCPU = now, current
	if elapsed <= 0 || delta < 0 {
		return nil
	}
	return delta.Seconds() * 100 / elapsed.Seconds()
}
func processCPUTime() time.Duration {
	value, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(value))
	if len(fields) < 15 {
		return 0
	}
	user, _ := strconv.ParseInt(fields[13], 10, 64)
	system, _ := strconv.ParseInt(fields[14], 10, 64)
	return time.Duration(user+system) * time.Second / 100
}
func processRSS() any {
	value, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(value))
	if len(fields) < 2 {
		return nil
	}
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return nil
	}
	return pages * int64(os.Getpagesize())
}
func fileSize(path string) any {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	return info.Size()
}
func metric(id, label string, value any, unit, status, help string) MonitorMetric {
	return MonitorMetric{ID: id, Label: label, Value: value, Unit: unit, Status: status, Help: help}
}
func available(value any) string {
	if value == nil {
		return "unavailable"
	}
	return "normal"
}
func numeric(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	}
	return 0, false
}
func statusNumber(value any, warning, critical float64) string {
	number, ok := numeric(value)
	if !ok {
		return "unavailable"
	}
	if number >= critical {
		return "critical"
	}
	if number >= warning {
		return "warning"
	}
	return "normal"
}
func budgetStatus(value any, target int64) string {
	number, ok := numeric(value)
	if !ok {
		return "unavailable"
	}
	switch storage.EvaluateBudget(int64(number), target, .8, .95, .98) {
	case storage.BudgetEmergency, storage.BudgetCritical:
		return "critical"
	case storage.BudgetWarning:
		return "warning"
	}
	return "normal"
}
func queueStatus(value int64, capacity int) string {
	if capacity <= 0 {
		return "unavailable"
	}
	ratio := float64(value) / float64(capacity)
	if ratio >= .95 {
		return "critical"
	}
	if ratio >= .8 {
		return "warning"
	}
	return "normal"
}
func nonzeroStatus(value uint64) string {
	if value > 0 {
		return "warning"
	}
	return "normal"
}
func durationValue(value time.Duration) any {
	if value <= 0 {
		return nil
	}
	return float64(value) / float64(time.Millisecond)
}
func durationStatus(value, warning time.Duration) string {
	if value <= 0 {
		return "unavailable"
	}
	if value > warning {
		return "warning"
	}
	return "normal"
}
func sseClients(engine *metrics.Engine) int {
	if engine == nil {
		return 0
	}
	return engine.SSEClients()
}
func dockerStatusValue(value string) string {
	if value == "healthy" {
		return "normal"
	}
	if value == "degraded" {
		return "warning"
	}
	if value == "down" {
		return "critical"
	}
	return "unavailable"
}
