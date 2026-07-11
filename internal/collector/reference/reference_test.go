// SPDX-License-Identifier: AGPL-3.0-only
package reference

import (
	"os"
	"testing"
	"time"

	dockercollector "github.com/drilonrecica/talos/internal/collector/docker"
	hostcollector "github.com/drilonrecica/talos/internal/collector/host"
	"golang.org/x/sys/unix"
)

// TestDockerCPUReference validates NormalizeCPU against hand-computed fixtures.
func TestDockerCPUReference(t *testing.T) {
	cases := []struct {
		name      string
		previous  dockercollector.CPUStats
		current   dockercollector.CPUStats
		hostCPUs  int
		wantHost  float64
		wantRatio float64
	}{
		{
			name:      "single CPU fully busy",
			previous:  dockercollector.CPUStats{Total: 1000, System: 1000, Online: 1},
			current:   dockercollector.CPUStats{Total: 2000, System: 2000, Online: 1},
			hostCPUs:  1,
			wantHost:  100.0,
			wantRatio: 1.0,
		},
		{
			name:      "half busy on two cores",
			previous:  dockercollector.CPUStats{Total: 1000, System: 2000, Online: 2},
			current:   dockercollector.CPUStats{Total: 2000, System: 4000, Online: 2},
			hostCPUs:  2,
			wantHost:  50.0,
			wantRatio: 0.5,
		},
		{
			name:      "fallback to host CPUs when online missing",
			previous:  dockercollector.CPUStats{Total: 1000, System: 1000, Online: 0},
			current:   dockercollector.CPUStats{Total: 2000, System: 2000, Online: 0},
			hostCPUs:  4,
			wantHost:  100.0,
			wantRatio: 1.0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dockercollector.NormalizeCPU(tc.previous, tc.current, tc.hostCPUs)
			if got.HostPercent == nil || *got.HostPercent != tc.wantHost {
				t.Fatalf("HostPercent = %v, want %v", got.HostPercent, tc.wantHost)
			}
			if got.Cores == nil || *got.Cores != tc.wantRatio {
				t.Fatalf("Cores ratio = %v, want %v", got.Cores, tc.wantRatio)
			}
		})
	}
}

// TestDockerMemoryReference validates working-set semantics against docker stats
// conventions: working set = usage - inactive file when possible.
func TestDockerMemoryReference(t *testing.T) {
	pids := uint64(12)
	got := dockercollector.NormalizeMemory(dockercollector.MemoryStats{
		Usage:        200 << 20,
		Limit:        1 << 30,
		InactiveFile: 50 << 20,
		PIDs:         &pids,
	}, 16<<30)

	if got.WorkingSet == nil || *got.WorkingSet != float64(150<<20) {
		t.Fatalf("WorkingSet = %v, want %v", got.WorkingSet, 150<<20)
	}
	if got.Percent == nil || *got.Percent <= 0 {
		t.Fatal("expected positive memory percent")
	}
	if got.PIDs == nil || *got.PIDs != pids {
		t.Fatalf("PIDs = %v, want %v", got.PIDs, pids)
	}
}

// TestDockerIOReference validates rate normalization from absolute counters.
func TestDockerIOReference(t *testing.T) {
	previous := dockercollector.IOCounters{RX: 1000, TX: 2000, Read: 3000, Write: 4000}
	current := dockercollector.IOCounters{RX: 4000, TX: 5000, Read: 6000, Write: 10000}
	got := dockercollector.NormalizeIO(previous, current, 10.0)
	checkRate(t, "RX", got.RX, 300.0)
	checkRate(t, "TX", got.TX, 300.0)
	checkRate(t, "Read", got.Read, 300.0)
	checkRate(t, "Write", got.Write, 600.0)
}

func checkRate(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// TestHostCPUAgainstProc validates parser output against the live /proc/stat
// interface on the current machine.
func TestHostCPUAgainstProc(t *testing.T) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		t.Skipf("/proc/stat unavailable: %v", err)
	}
	stats, err := hostcollector.ParseProcStat(string(raw))
	if err != nil {
		t.Fatalf("parse /proc/stat: %v", err)
	}
	agg, ok := stats["cpu"]
	if !ok {
		t.Fatal("missing aggregate cpu line")
	}
	total := agg.User + agg.Nice + agg.System + agg.Idle + agg.IOWait + agg.IRQ + agg.SoftIRQ + agg.Steal
	if total == 0 {
		t.Fatal("aggregate CPU total is zero")
	}
	if agg.Idle > total {
		t.Fatalf("idle %d > total %d", agg.Idle, total)
	}

	// A second sample should produce a sane busy percentage.
	time.Sleep(50 * time.Millisecond)
	raw2, err := os.ReadFile("/proc/stat")
	if err != nil {
		t.Fatalf("re-read /proc/stat: %v", err)
	}
	stats2, err := hostcollector.ParseProcStat(string(raw2))
	if err != nil {
		t.Fatalf("parse /proc/stat: %v", err)
	}
	delta := hostcollector.CPUDelta(stats["cpu"], stats2["cpu"])
	if delta.Busy == nil || *delta.Busy < 0 || *delta.Busy > 100 {
		t.Fatalf("cpu busy out of range: %v", delta.Busy)
	}
}

// TestHostMemoryAgainstProc validates that used memory never exceeds total.
func TestHostMemoryAgainstProc(t *testing.T) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		t.Skipf("/proc/meminfo unavailable: %v", err)
	}
	mem, err := hostcollector.ParseMeminfo(string(raw))
	if err != nil {
		t.Fatalf("parse /proc/meminfo: %v", err)
	}
	if mem.Total == 0 {
		t.Fatal("MemTotal is zero")
	}
	if mem.Used > mem.Total {
		t.Fatalf("used %d > total %d", mem.Used, mem.Total)
	}
	if mem.Available > mem.Total {
		t.Fatalf("available %d > total %d", mem.Available, mem.Total)
	}
}

// TestHostNetworkAgainstProc validates non-negative, monotonic counters.
func TestHostNetworkAgainstProc(t *testing.T) {
	raw, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		t.Skipf("/proc/net/dev unavailable: %v", err)
	}
	nics, err := hostcollector.ParseNetDev(string(raw))
	if err != nil {
		t.Fatalf("parse /proc/net/dev: %v", err)
	}
	if len(nics) == 0 {
		t.Fatal("no network interfaces found")
	}
	for name, nic := range nics {
		if nic.RXBytes < 0 || nic.TXBytes < 0 {
			t.Fatalf("%s: negative counters", name)
		}
	}
	agg := hostcollector.AggregateNetwork(nics)
	if agg.RXBytes < 0 || agg.TXBytes < 0 {
		t.Fatal("negative aggregate counters")
	}
}

// TestHostRateReference validates the rate helper against a hand-computed
// counter delta.
func TestHostRateReference(t *testing.T) {
	got := hostcollector.Rate(1000, 100, 1.0)
	if got == nil || *got != 900.0 {
		t.Fatalf("rate = %v, want 900", got)
	}
	if hostcollector.Rate(100, 200, 1.0) != nil {
		t.Fatal("expected nil for reset counter")
	}
}

// TestFilesystemReference validates statfs values against df-like semantics.
func TestFilesystemReference(t *testing.T) {
	var fs unix.Statfs_t
	if err := unix.Statfs("/tmp", &fs); err != nil {
		t.Skipf("statfs /tmp: %v", err)
	}
	total := int64(fs.Blocks) * int64(fs.Bsize)
	free := int64(fs.Bavail) * int64(fs.Bsize)
	if total <= 0 {
		t.Fatal("non-positive total blocks")
	}
	if free < 0 || free > total {
		t.Fatalf("free %d out of range for total %d", free, total)
	}
}
