// SPDX-License-Identifier: AGPL-3.0-only
package docker

type CPUStats struct {
	Total, System uint64
	Online        int
}
type CPUUsage struct{ HostPercent, DockerPercent, Cores *float64 }

func NormalizeCPU(previous, current CPUStats, hostCPUs int) CPUUsage {
	if current.Total < previous.Total || current.System <= previous.System {
		return CPUUsage{}
	}
	cpus := current.Online
	if cpus < 1 {
		cpus = hostCPUs
	}
	if cpus < 1 {
		return CPUUsage{}
	}
	ratio := float64(current.Total-previous.Total) / float64(current.System-previous.System)
	host := ratio * 100
	docker := host * float64(cpus)
	return CPUUsage{&host, &docker, &ratio}
}
