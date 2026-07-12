// SPDX-License-Identifier: AGPL-3.0-only
package production

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drilonrecica/talos/internal/dockerapi"
)

type fakeDocker struct{ stats dockerapi.Stats }

func (f *fakeDocker) List(context.Context) ([]dockerapi.Container, error) {
	return []dockerapi.Container{{ID: "123456789012", Name: "web"}}, nil
}
func (f *fakeDocker) Inspect(context.Context, string) (dockerapi.Inspect, error) {
	return dockerapi.Inspect{ID: "123456789012", Name: "web", State: "running", Labels: map[string]string{"com.docker.compose.project": "project", "com.docker.compose.service": "web"}}, nil
}
func (f *fakeDocker) Stats(context.Context, string) (dockerapi.Stats, error) { return f.stats, nil }
func (f *fakeDocker) Events(context.Context) <-chan dockerapi.Event {
	return make(chan dockerapi.Event)
}
func (f *fakeDocker) Version(context.Context) (dockerapi.Version, error) {
	return dockerapi.Version{APIVersion: "1.47"}, nil
}
func (f *fakeDocker) Diagnostics(context.Context) (dockerapi.Diagnostics, error) {
	return dockerapi.Diagnostics{Containers: 1}, nil
}

func TestSamplerCollectsMergedHostAndResourceState(t *testing.T) {
	root := t.TempDir()
	write := func(name, value string) {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("stat", "cpu 10 0 10 80 0 0 0 0\n")
	write("meminfo", "MemTotal: 1000 kB\nMemAvailable: 400 kB\n")
	write("loadavg", "0.25 0.20 0.10 1/10 1\n")
	write("uptime", "100.0 50.0\n")
	write("net/dev", "eth0: 10 1 0 0 0 0 0 0 20 2 0 0 0 0 0 0\n")
	write("diskstats", "   8       0 sda 10 0 20 0 5 0 40 0 0 0 0 0 0 0 0\n")
	write("self/mountinfo", "1 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw\n")
	write("sys/kernel/random/boot_id", "boot-one\n")
	docker := &fakeDocker{stats: dockerapi.Stats{CPU: dockerapi.CPUStats{TotalUsage: 100, SystemUsage: 1000, OnlineCPUs: 2}, Memory: dockerapi.MemoryStats{Usage: 100, Limit: 1000}}}
	sampler := &Sampler{HostProc: root, Docker: docker}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	host, _, boot, err := sampler.collectHost(now)
	if err != nil || boot != "boot-one" || host.MemoryUsedBytes == nil {
		t.Fatalf("host=%+v boot=%q err=%v", host, boot, err)
	}
	if _, err = sampler.collectDocker(context.Background(), now, host.MemoryTotalBytes); err != nil {
		t.Fatal(err)
	}
	docker.stats.CPU.TotalUsage, docker.stats.CPU.SystemUsage = 200, 2000
	resources, err := sampler.collectDocker(context.Background(), now.Add(2*time.Second), host.MemoryTotalBytes)
	if err != nil || len(resources) != 1 || resources[0].CPUHostPercent == nil || resources[0].Project != "project" || len(resources[0].Components) != 1 {
		t.Fatalf("resources=%+v err=%v", resources, err)
	}
}
