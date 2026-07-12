// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/drilonrecica/binnacle/internal/metrics"
)

const hostID = "host"

func (m *Manager) WriteBatch(ctx context.Context, b metrics.PersistenceBatch) error {
	tx, e := m.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	s := b.Snapshot
	if s.Sequence > 0 {
		_, e = tx.ExecContext(ctx, `INSERT OR REPLACE INTO host_samples_10s(
			ts,host_id,boot_session_id,
			cpu_busy_pct,cpu_user_pct,cpu_system_pct,cpu_iowait_pct,cpu_steal_pct,
			load_1,load_5,load_15,
			memory_used_bytes,memory_available_bytes,memory_total_bytes,memory_used_pct,
			memory_cached_bytes,memory_buffers_bytes,
			swap_used_bytes,swap_total_bytes,swap_used_pct,
			network_rx_bps,network_tx_bps,network_rx_packets_ps,network_tx_packets_ps,
			network_rx_errors_delta,network_tx_errors_delta,network_rx_drops_delta,network_tx_drops_delta,
			disk_read_bps,disk_write_bps,disk_read_iops,disk_write_iops
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			s.At.UnixMilli(), hostID, string(s.BootIdentity),
			s.Host.CPUPercent, s.Host.CPUUserPercent, s.Host.CPUSystemPercent, s.Host.CPUIOWaitPercent, s.Host.CPUStealPercent,
			s.Host.Load1, s.Host.Load5, s.Host.Load15,
			s.Host.MemoryUsedBytes, s.Host.MemoryAvailableBytes, s.Host.MemoryTotalBytes, s.Host.MemoryPercent,
			s.Host.MemoryCachedBytes, s.Host.MemoryBuffersBytes,
			s.Host.SwapUsedBytes, s.Host.SwapTotalBytes, s.Host.SwapPercent,
			s.Host.NetworkRXBPS, s.Host.NetworkTXBPS, s.Host.NetworkRXPacketsPS, s.Host.NetworkTXPacketsPS,
			s.Host.NetworkRXErrorsDelta, s.Host.NetworkTXErrorsDelta, s.Host.NetworkRXDropsDelta, s.Host.NetworkTXDropsDelta,
			s.Host.DiskReadBPS, s.Host.DiskWriteBPS, s.Host.DiskReadIOPS, s.Host.DiskWriteIOPS,
		)
		if e != nil {
			return e
		}
		for _, resource := range s.Resources {
			_, e = tx.ExecContext(ctx, "INSERT OR REPLACE INTO resource_samples_10s(ts,resource_id,cpu_host_pct,memory_working_set_bytes,network_rx_bps,network_tx_bps,block_read_bps,block_write_bps,active_instance_count,status) VALUES(?,?,?,?,?,?,?,?,?,?)", s.At.UnixMilli(), resource.ID, resource.CPUHostPercent, resource.MemoryBytes, resource.RXBPS, resource.TXBPS, resource.BlockReadBPS, resource.BlockWriteBPS, len(resource.Components), resource.Status)
			if e != nil {
				return e
			}
			for _, comp := range resource.Components {
				if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO container_instances(id,resource_id,name,created_at) VALUES(?,?,?,?)", string(comp.ID), string(resource.ID), comp.Name, s.At.UnixMilli()); e != nil {
					return e
				}
				if _, e = tx.ExecContext(ctx, "UPDATE container_instances SET resource_id=?, name=? WHERE id=?", string(resource.ID), comp.Name, string(comp.ID)); e != nil {
					return e
				}
				if _, e = tx.ExecContext(ctx, "INSERT OR REPLACE INTO container_instance_samples_10s(ts,container_instance_id,cpu_host_pct,memory_working_set_bytes,memory_usage_bytes,network_rx_bps,network_tx_bps,block_read_bps,block_write_bps,pids) VALUES(?,?,?,?,?,?,?,?,?,?)", s.At.UnixMilli(), string(comp.ID), comp.CPUHostPercent, comp.MemoryBytes, nil, comp.RXBPS, comp.TXBPS, comp.BlockReadBPS, comp.BlockWriteBPS, comp.PIDs); e != nil {
					return e
				}
			}
		}
	}
	for _, fs := range b.Filesystems {
		if _, e = tx.ExecContext(ctx, "INSERT OR REPLACE INTO filesystem_samples_1m(ts,host_id,mount_key,mount_point,fs_type,total_bytes,used_bytes,available_bytes,used_pct,inodes_total,inodes_used,inodes_used_pct) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", fs.At.UnixMilli(), hostID, fs.MountKey, fs.MountPoint, fs.FSType, fs.TotalBytes, fs.UsedBytes, fs.AvailableBytes, fs.UsedPercent, fs.InodesTotal, fs.InodesUsed, fs.InodesUsedPercent); e != nil {
			return e
		}
	}
	for _, event := range b.Events {
		if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO events(id,ts,host_id,resource_id,container_instance_id,type,severity,summary,details_json,correlation_key,source,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", event.ID, event.At.UnixMilli(), hostID, nullableResource(event.ResourceID), nullableContainer(event.ContainerInstance), event.Type, severity(event), event.Message, nullableString(event.Details), nullableString(event.CorrelationKey), eventSource(event.Type), event.At.UnixMilli()); e != nil {
			return e
		}
	}
	if e = m.writeCollectorStateEvents(ctx, tx, s.At, s.Collectors); e != nil {
		return e
	}
	return tx.Commit()
}

func (m *Manager) writeCollectorStateEvents(ctx context.Context, tx querier, at time.Time, collectors map[string]metrics.CollectorHealth) error {
	if m.prevCollectors == nil {
		m.prevCollectors = map[string]string{}
	}
	for name, health := range collectors {
		prev := m.prevCollectors[name]
		if prev == "" {
			prev = string(metrics.CollectorUnknown)
		}
		current := string(health.State)
		if prev == current {
			continue
		}
		id := fmt.Sprintf("cse_%d_%s", at.UnixMilli(), name)
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO collector_state_events(id,ts,collector_name,previous_state,new_state,reason_code,message) VALUES(?,?,?,?,?,?,?)", id, at.UnixMilli(), name, prev, current, health.State, health.Reason); err != nil {
			return err
		}
		m.prevCollectors[name] = current
	}
	return nil
}

type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func nullableResource(id metrics.ResourceID) any {
	if id == "" {
		return nil
	}
	return string(id)
}
func nullableContainer(id metrics.ContainerID) any {
	if id == "" {
		return nil
	}
	return string(id)
}
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func severity(e metrics.Event) string {
	if e.Severity != "" {
		return e.Severity
	}
	return eventSeverity(e.Type)
}

func eventSeverity(eventType string) string {
	switch eventType {
	case "container_oom", "collector_down", "persistence_degraded", "host_reboot":
		return "critical"
	case "container_restart", "container_stop", "container_die", "persistence_gap":
		return "warning"
	}
	return "info"
}

func eventSource(eventType string) string {
	if eventType == "host_reboot" || eventType == "collector_down" || eventType == "collector_degraded" || eventType == "persistence_degraded" || eventType == "persistence_gap" {
		return "binnacle"
	}
	return "docker"
}

func (m *Manager) WriteEvent(ctx context.Context, e metrics.Event) error {
	_, err := m.db.ExecContext(ctx, "INSERT OR IGNORE INTO events(id,ts,host_id,resource_id,container_instance_id,type,severity,summary,details_json,correlation_key,source,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", e.ID, e.At.UnixMilli(), hostID, nullableResource(e.ResourceID), nullableContainer(e.ContainerInstance), e.Type, severity(e), e.Message, nullableString(e.Details), nullableString(e.CorrelationKey), eventSource(e.Type), e.At.UnixMilli())
	return err
}
