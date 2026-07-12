// SPDX-License-Identifier: AGPL-3.0-only

package demo

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
)

type Clock interface{ Now() time.Time }
type Generator struct {
	seed       uint64
	clock      Clock
	Containers int
}

func New(seed uint64, clock Clock) *Generator {
	return &Generator{seed: seed, clock: clock, Containers: 1}
}
func (g *Generator) Snapshot(step uint64) metrics.Snapshot {
	r := rand.New(rand.NewPCG(g.seed, step))
	now := g.clock.Now().UTC()
	cpu := 5 + r.Float64()*40
	memory := int64(2<<30) + int64(r.Uint64()%uint64(2<<30))
	count := g.Containers
	if count < 1 {
		count = 1
	}
	resources := make([]metrics.ResourceSnapshot, count)
	for i := 0; i < count; i++ {
		status := metrics.StatusHealthy
		offset := step + uint64(i)
		if offset%11 == 0 {
			status = metrics.StatusDegraded
		}
		if offset%17 == 0 {
			status = metrics.StatusArchived
		}
		name := fmt.Sprintf("demo-service-%d", i+1)
		id := fmt.Sprintf("res_demo_%d", i+1)
		resCPU := cpu + r.Float64()*10
		resMem := memory + int64(r.Int64()%(1<<30))
		resources[i] = metrics.ResourceSnapshot{ID: metrics.ResourceID(id), Name: name, Status: status, CPUHostPercent: &resCPU, MemoryBytes: &resMem, LastSeenAt: now, Category: "service", StableKey: name}
	}
	return metrics.Snapshot{Sequence: metrics.Sequence(step + 1), At: now, BootIdentity: "demo-boot-1", Host: metrics.HostObservation{At: now, CPUPercent: &cpu, MemoryUsedBytes: &memory}, Resources: resources, Collectors: map[string]metrics.CollectorHealth{"host": {Name: "host", State: metrics.CollectorHealthy, FreshAt: now}, "docker": {Name: "docker", State: metrics.CollectorHealthy, FreshAt: now}}}
}
func (g *Generator) Events(step uint64) []metrics.Event {
	now := g.clock.Now().UTC()
	switch {
	case step%17 == 0:
		return []metrics.Event{{ID: metrics.Sequence(step + 1), At: now, Type: "resource_archived", ResourceID: "res_demo_web", Severity: "info", Message: "Demo resource archived", Details: `{"reason":"scheduled"}`}}
	case step%11 == 0:
		return []metrics.Event{{ID: metrics.Sequence(step + 1), At: now, Type: "collector_degraded", Severity: "warning", Message: "Demo collector degraded", Details: `{"collector":"docker"}`}}
	case step%7 == 0:
		return []metrics.Event{{ID: metrics.Sequence(step + 1), At: now, Type: "oom", ResourceID: "res_demo_web", Severity: "critical", Message: "Demo out-of-memory restart", Details: `{"container":"demo-container-1"}`}}
	}
	return nil
}
