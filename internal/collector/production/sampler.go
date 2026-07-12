// SPDX-License-Identifier: AGPL-3.0-only
package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dockercollector "github.com/drilonrecica/talos/internal/collector/docker"
	hostcollector "github.com/drilonrecica/talos/internal/collector/host"
	"github.com/drilonrecica/talos/internal/coolify"
	"github.com/drilonrecica/talos/internal/dockerapi"
	"github.com/drilonrecica/talos/internal/events"
	"github.com/drilonrecica/talos/internal/metrics"
	"github.com/drilonrecica/talos/internal/resources"
	"github.com/drilonrecica/talos/internal/storage"
	"golang.org/x/sys/unix"
)

type MetadataCache interface {
	Get(id string) (dockercollector.Metadata, bool)
	Set(v dockercollector.Metadata)
	Remove(id string)
}

type Sampler struct {
	Engine               *metrics.Engine
	Docker               dockerapi.Client
	Cache                MetadataCache
	HostProc             string
	DataDir              string
	Interval             func() time.Duration
	MaxDockerConcurrency int
	Store                interface {
		UpsertHost(context.Context, string, string, string) error
		UpsertResource(context.Context, storage.Resource) error
		ArchiveMissingResources(context.Context, []string, time.Time) error
	}
	cancel            context.CancelFunc
	mu                sync.Mutex
	previousCPU       hostcollector.CPUCounters
	haveCPU           bool
	previousNetwork   hostcollector.NetworkCounters
	previousNetworkAt time.Time
	previousDisk      hostcollector.DiskCounters
	previousDiskAt    time.Time
	previousStats     map[string]dockerSample
	lastResources     []metrics.ResourceSnapshot
	hostFailures      int
	dockerFailures    int
	LastDurationNanos atomic.Int64
}
type dockerSample struct {
	value dockerapi.Stats
	at    time.Time
}

func (s *Sampler) Start(ctx context.Context) error {
	if s.Engine == nil {
		return errors.New("metrics engine is required")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	go s.run(ctx)
	return nil
}
func (s *Sampler) Stop(context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	if closer, ok := s.Docker.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (s *Sampler) run(ctx context.Context) {
	var dockerEvents <-chan dockerapi.Event
	if s.Docker != nil {
		dockerEvents = s.Docker.Events(ctx)
	}
	pending := make([]metrics.Event, 0, 16)
	for {
		s.collect(ctx, pending)
		pending = pending[:0]
		interval := 2 * time.Second
		if s.Interval != nil && s.Interval() >= time.Second {
			interval = s.Interval()
		}
		timer := time.NewTimer(interval)
	wait:
		for {
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case event, ok := <-dockerEvents:
				if !ok {
					dockerEvents = nil
					continue
				}
				s.handleDockerEvent(ctx, event)
				if normalized, accepted := events.NormalizeDocker(event, false); accepted {
					if len(pending) == 128 {
						pending = pending[1:]
					}
					pending = append(pending, normalized)
				}
			case <-timer.C:
				break wait
			}
		}
	}
}

func (s *Sampler) collect(ctx context.Context, pending []metrics.Event) {
	started := time.Now()
	defer func() { s.LastDurationNanos.Store(time.Since(started).Nanoseconds()) }()
	now := time.Now().UTC()
	host, filesystems, boot, hostErr := s.collectHost(now)
	if len(filesystems) > 0 {
		s.Engine.PublishFilesystems(filesystems)
	}
	collectors := map[string]metrics.CollectorHealth{}
	if hostErr != nil {
		s.hostFailures++
		collectors["host"] = health("host", s.hostFailures, hostErr, now)
	} else {
		s.hostFailures = 0
		collectors["host"] = health("host", 0, nil, now)
	}
	resourceValues, dockerErr := s.collectDocker(ctx, now, host.MemoryTotalBytes)
	if dockerErr != nil {
		s.dockerFailures++
		collectors["docker"] = health("docker", s.dockerFailures, dockerErr, now)
		resourceValues = append([]metrics.ResourceSnapshot(nil), s.lastResources...)
	} else {
		s.dockerFailures = 0
		collectors["docker"] = health("docker", 0, nil, now)
		s.lastResources = append([]metrics.ResourceSnapshot(nil), resourceValues...)
		if s.Store != nil {
			_ = s.Store.UpsertHost(ctx, "host", storage.HostIdentity("", "talos-local-host"), "Server")
			ids := make([]string, 0, len(resourceValues))
			for _, resource := range resourceValues {
				ids = append(ids, string(resource.ID))
				_ = s.Store.UpsertResource(ctx, storage.Resource{ID: string(resource.ID), HostID: "host", StableKey: resource.StableKey, SourceKind: resource.SourceKind, Name: resource.Name, ProjectName: resource.Project, EnvironmentName: resource.Environment, Category: resource.Category, Status: string(resource.Status)})
			}
			_ = s.Store.ArchiveMissingResources(ctx, ids, now.Add(-5*time.Minute))
		}
	}
	s.Engine.Publish(metrics.Snapshot{At: now, BootIdentity: metrics.BootIdentity(boot), Host: host, Resources: resourceValues, Collectors: collectors}, pending...)
}
func (s *Sampler) handleDockerEvent(ctx context.Context, event dockerapi.Event) {
	if s.Cache == nil {
		return
	}
	switch strings.ToLower(event.Action) {
	case "destroy":
		s.Cache.Remove(event.ID)
	case "die", "stop", "pause":
		if v, ok := s.Cache.Get(event.ID); ok {
			v.State = strings.ToLower(event.Action)
			s.Cache.Set(v)
		}
	case "start", "unpause", "restart", "health_status":
		// Refresh metadata on lifecycle/health changes.
		if inspect, err := s.Docker.Inspect(ctx, event.ID); err == nil {
			s.Cache.Set(dockercollector.Metadata{ID: inspect.ID, Name: inspect.Name, Image: inspect.Image, Created: inspect.Created, State: inspect.State, Health: inspect.Health, Labels: inspect.Labels, Networks: inspect.Networks, Mounts: inspect.Mounts})
		}
	}
}

func (s *Sampler) CollectionDuration() time.Duration {
	if s == nil {
		return 0
	}
	return time.Duration(s.LastDurationNanos.Load())
}

func (s *Sampler) collectHost(now time.Time) (metrics.HostObservation, []metrics.FilesystemObservation, string, error) {
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(s.HostProc, name)) }
	statRaw, err := read("stat")
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	stats, err := hostcollector.ParseProcStat(string(statRaw))
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	memRaw, err := read("meminfo")
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	memory, err := hostcollector.ParseMeminfo(string(memRaw))
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	loadRaw, err := read("loadavg")
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	load, err := hostcollector.ParseLoadavgFull(string(loadRaw))
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	uptimeRaw, err := read("uptime")
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	uptime, err := hostcollector.ParseUptime(string(uptimeRaw))
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	networkRaw, err := read("net/dev")
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	networks, err := hostcollector.ParseNetDev(string(networkRaw))
	if err != nil {
		return metrics.HostObservation{}, nil, "", err
	}
	network := hostcollector.AggregateNetwork(networks)

	diskRaw, err := read("diskstats")
	var disk hostcollector.DiskCounters
	if err == nil {
		disks, _ := hostcollector.ParseDiskstats(string(diskRaw))
		for _, d := range disks {
			disk.Reads += d.Reads
			disk.ReadSectors += d.ReadSectors
			disk.Writes += d.Writes
			disk.WriteSectors += d.WriteSectors
		}
	}

	var cpu, cpuUser, cpuSystem, cpuIOWait, cpuSteal *float64
	if s.haveCPU {
		usage := hostcollector.CPUDelta(s.previousCPU, stats["cpu"])
		cpu, cpuUser, cpuSystem, cpuIOWait, cpuSteal = usage.Busy, usage.User, usage.System, usage.IOWait, usage.Steal
	}
	s.previousCPU, s.haveCPU = stats["cpu"], true

	var rx, tx, rxPackets, txPackets *float64
	var rxErrors, txErrors, rxDrops, txDrops *int64
	if !s.previousNetworkAt.IsZero() {
		elapsed := now.Sub(s.previousNetworkAt).Seconds()
		rx = hostcollector.Rate(network.RXBytes, s.previousNetwork.RXBytes, elapsed)
		tx = hostcollector.Rate(network.TXBytes, s.previousNetwork.TXBytes, elapsed)
		rxPackets = hostcollector.Rate(network.RXPackets, s.previousNetwork.RXPackets, elapsed)
		txPackets = hostcollector.Rate(network.TXPackets, s.previousNetwork.TXPackets, elapsed)
		rxErrors = deltaInt64(network.RXErrors, s.previousNetwork.RXErrors)
		txErrors = deltaInt64(network.TXErrors, s.previousNetwork.TXErrors)
		rxDrops = deltaInt64(network.RXDrops, s.previousNetwork.RXDrops)
		txDrops = deltaInt64(network.TXDrops, s.previousNetwork.TXDrops)
	}
	s.previousNetwork, s.previousNetworkAt = network, now

	var diskReadBPS, diskWriteBPS, diskReadIOPS, diskWriteIOPS *float64
	if !s.previousDiskAt.IsZero() {
		elapsed := now.Sub(s.previousDiskAt).Seconds()
		diskReadBPS = hostcollector.Rate(hostcollector.SectorToBytes(disk.ReadSectors), hostcollector.SectorToBytes(s.previousDisk.ReadSectors), elapsed)
		diskWriteBPS = hostcollector.Rate(hostcollector.SectorToBytes(disk.WriteSectors), hostcollector.SectorToBytes(s.previousDisk.WriteSectors), elapsed)
		diskReadIOPS = hostcollector.Rate(disk.Reads, s.previousDisk.Reads, elapsed)
		diskWriteIOPS = hostcollector.Rate(disk.Writes, s.previousDisk.Writes, elapsed)
	}
	s.previousDisk, s.previousDiskAt = disk, now

	used, total := int64(memory.Used), int64(memory.Total)
	available := int64(memory.Available)
	cached := int64(memory.Cached)
	buffers := int64(memory.Buffers)
	var memoryPercent *float64
	if memory.Total > 0 {
		v := float64(memory.Used) * 100 / float64(memory.Total)
		memoryPercent = &v
	}
	swapUsed := int64(memory.SwapTotal - memory.SwapFree)
	swapTotal := int64(memory.SwapTotal)
	var swapPercent *float64
	if memory.SwapTotal > 0 {
		v := float64(memory.SwapTotal-memory.SwapFree) * 100 / float64(memory.SwapTotal)
		swapPercent = &v
	}
	load1, load5, load15 := load.One, load.Five, load.Fifteen

	observation := metrics.HostObservation{
		At: now, CPUPercent: cpu, CPUUserPercent: cpuUser, CPUSystemPercent: cpuSystem, CPUIOWaitPercent: cpuIOWait, CPUStealPercent: cpuSteal,
		MemoryUsedBytes: &used, MemoryTotalBytes: &total, MemoryAvailableBytes: &available, MemoryPercent: memoryPercent,
		MemoryCachedBytes: &cached, MemoryBuffersBytes: &buffers, SwapUsedBytes: &swapUsed, SwapTotalBytes: &swapTotal, SwapPercent: swapPercent,
		Load1: &load1, Load5: &load5, Load15: &load15,
		NetworkRXBPS: rx, NetworkTXBPS: tx, NetworkRXPacketsPS: rxPackets, NetworkTXPacketsPS: txPackets,
		NetworkRXErrorsDelta: rxErrors, NetworkTXErrorsDelta: txErrors, NetworkRXDropsDelta: rxDrops, NetworkTXDropsDelta: txDrops,
		DiskReadBPS: diskReadBPS, DiskWriteBPS: diskWriteBPS, DiskReadIOPS: diskReadIOPS, DiskWriteIOPS: diskWriteIOPS,
		UptimeSeconds: &uptime,
	}

	fsystems, fsErr := s.collectFilesystems(now)
	if fsErr == nil && len(fsystems) > 0 {
		var totalBytes, usedBytes int64
		for _, fs := range fsystems {
			if fs.TotalBytes != nil {
				totalBytes += *fs.TotalBytes
			}
			if fs.UsedBytes != nil {
				usedBytes += *fs.UsedBytes
			}
		}
		observation.DiskTotalBytes, observation.DiskUsedBytes = &totalBytes, &usedBytes
	}

	bootRaw, _ := read("sys/kernel/random/boot_id")
	return observation, fsystems, strings.TrimSpace(string(bootRaw)), nil
}

func deltaInt64(current, previous uint64) *int64 {
	if current < previous {
		return nil
	}
	v := int64(current - previous)
	return &v
}

func (s *Sampler) collectFilesystems(now time.Time) ([]metrics.FilesystemObservation, error) {
	mountsRaw, err := os.ReadFile(filepath.Join(s.HostProc, "self/mountinfo"))
	if err != nil {
		mountsRaw, err = os.ReadFile(filepath.Join(s.HostProc, "mounts"))
		if err != nil {
			return nil, err
		}
	}
	mounts := hostcollector.ParseMounts(string(mountsRaw), s.DataDir)
	out := make([]metrics.FilesystemObservation, 0, len(mounts))
	seen := map[string]bool{}
	for _, m := range mounts {
		key := m.Source + "|" + m.Target
		if seen[key] {
			continue
		}
		seen[key] = true
		var fs unix.Statfs_t
		if unix.Statfs(m.Target, &fs) != nil {
			continue
		}
		total := int64(fs.Blocks) * int64(fs.Bsize)
		available := int64(fs.Bavail) * int64(fs.Bsize)
		free := int64(fs.Bfree) * int64(fs.Bsize)
		used := total - free
		var usedPct *float64
		if total > 0 {
			v := float64(used) * 100 / float64(total)
			usedPct = &v
		}
		var totalInodes, usedInodes *int64
		if fs.Files > 0 {
			ti := int64(fs.Files)
			ui := int64(fs.Files - fs.Ffree)
			totalInodes, usedInodes = &ti, &ui
		}
		out = append(out, metrics.FilesystemObservation{
			At: now, MountKey: key, MountPoint: m.Target, FSType: m.FSType,
			TotalBytes: &total, UsedBytes: &used, AvailableBytes: &available, UsedPercent: usedPct,
			InodesTotal: totalInodes, InodesUsed: usedInodes,
		})
	}
	return out, nil
}

type resourceGroup struct {
	identity                                     resources.Identity
	category, environment                        string
	infrastructure                               bool
	components                                   []metrics.ResourceComponent
	cpu, memory, rx, tx, read, write             float64
	cpuOK, memoryOK, rxOK, txOK, readOK, writeOK bool
	status                                       []metrics.ResourceStatus
}

func (s *Sampler) collectDocker(ctx context.Context, now time.Time, hostTotal *int64) ([]metrics.ResourceSnapshot, error) {
	if s.Docker == nil {
		return nil, errors.New("Docker client is not configured")
	}
	containers, err := s.Docker.List(ctx)
	if err != nil {
		return nil, err
	}
	groups := map[string]*resourceGroup{}
	if s.previousStats == nil {
		s.previousStats = map[string]dockerSample{}
	}
	for _, container := range containers {
		inspect, inspectErr := s.inspect(ctx, container.ID)
		if inspectErr != nil {
			return nil, inspectErr
		}
		identity := resources.Resolve(inspect.Labels, inspect.Name, "")
		group := groups[identity.StableKey]
		if group == nil {
			group = &resourceGroup{identity: identity, category: category(inspect.Labels, identity), environment: inspect.Labels["coolify.environment"]}
			if metadata, ok := coolify.Resolve(inspect.Labels); ok {
				group.infrastructure = metadata.Infrastructure
				if metadata.Environment != "" {
					group.environment = metadata.Environment
				}
				if metadata.Project != "" {
					group.identity.Project = metadata.Project
				}
			}
			groups[identity.StableKey] = group
		}
		status := containerStatus(inspect.State, inspect.Health)
		group.status = append(group.status, status)
		component := metrics.ResourceComponent{ID: metrics.ContainerID(container.ID), Name: inspect.Name, Status: status}
		if inspect.State != "running" {
			group.components = append(group.components, component)
			continue
		}
		stats, statsErr := s.Docker.Stats(ctx, container.ID)
		if statsErr != nil {
			return nil, statsErr
		}
		memory := dockercollector.NormalizeMemory(dockercollector.MemoryStats{Usage: stats.Memory.Usage, Limit: stats.Memory.Limit, InactiveFile: stats.Memory.InactiveFile, PIDs: stats.PIDs}, uint64Value(hostTotal))
		component.MemoryBytes = int64PtrFromFloat64Bytes(memory.WorkingSet)
		component.PIDs = stats.PIDs
		if memory.WorkingSet != nil {
			group.memory += *memory.WorkingSet
			group.memoryOK = true
		}
		if previous, ok := s.previousStats[container.ID]; ok {
			cpu := dockercollector.NormalizeCPU(dockercollector.CPUStats{Total: previous.value.CPU.TotalUsage, System: previous.value.CPU.SystemUsage, Online: previous.value.CPU.OnlineCPUs}, dockercollector.CPUStats{Total: stats.CPU.TotalUsage, System: stats.CPU.SystemUsage, Online: stats.CPU.OnlineCPUs}, stats.CPU.OnlineCPUs)
			component.CPUHostPercent = cpu.HostPercent
			if cpu.HostPercent != nil {
				group.cpu += *cpu.HostPercent
				group.cpuOK = true
			}
			io := dockercollector.NormalizeIO(dockercollector.IOCounters{RX: previous.value.IO.RX, TX: previous.value.IO.TX, Read: previous.value.IO.Read, Write: previous.value.IO.Write}, dockercollector.IOCounters{RX: stats.IO.RX, TX: stats.IO.TX, Read: stats.IO.Read, Write: stats.IO.Write}, now.Sub(previous.at).Seconds())
			component.RXBPS, component.TXBPS, component.BlockReadBPS, component.BlockWriteBPS = io.RX, io.TX, io.Read, io.Write
			if io.RX != nil {
				group.rx += *io.RX
				group.rxOK = true
			}
			if io.TX != nil {
				group.tx += *io.TX
				group.txOK = true
			}
			if io.Read != nil {
				group.read += *io.Read
				group.readOK = true
			}
			if io.Write != nil {
				group.write += *io.Write
				group.writeOK = true
			}
		}
		group.components = append(group.components, component)
		s.previousStats[container.ID] = dockerSample{value: stats, at: now}
	}
	result := make([]metrics.ResourceSnapshot, 0, len(groups))
	for stable, group := range groups {
		id := resourceID(stable)
		result = append(result, metrics.ResourceSnapshot{ID: id, Name: group.identity.Name, Status: resources.RollupStatus(group.status), CPUHostPercent: number(group.cpu, group.cpuOK), MemoryBytes: integer(group.memory, group.memoryOK), RXBPS: number(group.rx, group.rxOK), TXBPS: number(group.tx, group.txOK), BlockReadBPS: number(group.read, group.readOK), BlockWriteBPS: number(group.write, group.writeOK), LastSeenAt: now, Category: group.category, Project: group.identity.Project, Environment: group.environment, Infrastructure: group.infrastructure, Components: group.components, StableKey: stable, SourceKind: group.identity.Source})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (s *Sampler) inspect(ctx context.Context, id string) (dockerapi.Inspect, error) {
	if s.Cache != nil {
		if v, ok := s.Cache.Get(id); ok {
			return dockerapi.Inspect{ID: v.ID, Name: v.Name, Image: v.Image, Created: v.Created, State: v.State, Health: v.Health, Labels: v.Labels, Networks: v.Networks, Mounts: v.Mounts}, nil
		}
	}
	inspect, err := s.Docker.Inspect(ctx, id)
	if err != nil {
		return dockerapi.Inspect{}, err
	}
	if s.Cache != nil {
		s.Cache.Set(dockercollector.Metadata{ID: inspect.ID, Name: inspect.Name, Image: inspect.Image, Created: inspect.Created, State: inspect.State, Health: inspect.Health, Labels: inspect.Labels, Networks: inspect.Networks, Mounts: inspect.Mounts})
	}
	return inspect, nil
}

func containerStatus(state, health string) metrics.ResourceStatus {
	switch state {
	case "running":
		if health == "unhealthy" {
			return metrics.StatusDown
		}
		if health == "starting" {
			return metrics.StatusUnknown
		}
		return metrics.StatusHealthy
	case "paused":
		return metrics.StatusPaused
	case "exited", "dead":
		return metrics.StatusDown
	}
	return metrics.StatusUnknown
}

func health(name string, failures int, err error, now time.Time) metrics.CollectorHealth {
	state := metrics.CollectorHealthy
	if failures >= 6 {
		state = metrics.CollectorDown
	} else if failures >= 3 {
		state = metrics.CollectorDegraded
	}
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return metrics.CollectorHealth{Name: name, State: state, Reason: reason, FreshAt: now}
}
func resourceID(stable string) metrics.ResourceID {
	sum := sha256.Sum256([]byte(stable))
	return metrics.ResourceID("res_" + hex.EncodeToString(sum[:8]))
}
func category(labels map[string]string, identity resources.Identity) string {
	if value := strings.ToLower(labels["talos.category"]); resources.ValidCategory(value) {
		return value
	}
	if labels["coolify.type"] == "infrastructure" {
		return "infrastructure"
	}
	if identity.Source == "compose" || identity.Source == "coolify" {
		return "service"
	}
	return "unmanaged"
}
func number(value float64, ok bool) *float64 {
	if !ok {
		return nil
	}
	return &value
}
func integer(value float64, ok bool) *int64 {
	if !ok {
		return nil
	}
	result := int64(value)
	return &result
}
func uint64Value(value *int64) uint64 {
	if value == nil || *value < 0 {
		return 0
	}
	return uint64(*value)
}
func int64PtrFromFloat64Bytes(v *float64) *int64 {
	if v == nil {
		return nil
	}
	n := int64(*v)
	return &n
}
