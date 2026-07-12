// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"context"
	"fmt"
	"time"
)

// RollupOnce materializes closed raw buckets. Replacing an existing bucket is
// idempotent and lets late samples repair the aggregate before retention runs.
func (m *Manager) RollupOnce(ctx context.Context, now time.Time) error {
	if m.db == nil {
		return fmt.Errorf("storage is not open")
	}
	for _, tier := range []struct {
		name string
		size time.Duration
	}{{"1m", time.Minute}, {"15m", 15 * time.Minute}, {"1h", time.Hour}} {
		bucket := tier.size.Milliseconds()
		cutoff := now.UTC().Truncate(tier.size).UnixMilli()
		host := fmt.Sprintf(`INSERT OR REPLACE INTO host_rollups_%s(
	ts,
	cpu_avg,cpu_min,cpu_max,sample_count,
	memory_avg,memory_min,memory_max,memory_count,
	network_rx_avg,network_rx_min,network_rx_max,network_rx_count,
	network_tx_avg,network_tx_min,network_tx_max,network_tx_count,
	cpu_user_avg,cpu_user_min,cpu_user_max,cpu_user_count,
	cpu_system_avg,cpu_system_min,cpu_system_max,cpu_system_count,
	cpu_iowait_avg,cpu_iowait_min,cpu_iowait_max,cpu_iowait_count,
	cpu_steal_avg,cpu_steal_min,cpu_steal_max,cpu_steal_count,
	load_1_avg,load_1_min,load_1_max,load_1_count,
	load_5_avg,load_5_min,load_5_max,load_5_count,
	load_15_avg,load_15_min,load_15_max,load_15_count,
	swap_used_avg,swap_used_min,swap_used_max,swap_used_count,
	disk_read_avg,disk_read_min,disk_read_max,disk_read_count,
	disk_write_avg,disk_write_min,disk_write_max,disk_write_count,
	disk_iops_avg,disk_iops_min,disk_iops_max,disk_iops_count
)
SELECT (ts/%d)*%d,
	AVG(cpu_busy_pct),MIN(cpu_busy_pct),MAX(cpu_busy_pct),COUNT(cpu_busy_pct),
	AVG(memory_used_bytes),MIN(memory_used_bytes),MAX(memory_used_bytes),COUNT(memory_used_bytes),
	AVG(network_rx_bps),MIN(network_rx_bps),MAX(network_rx_bps),COUNT(network_rx_bps),
	AVG(network_tx_bps),MIN(network_tx_bps),MAX(network_tx_bps),COUNT(network_tx_bps),
	AVG(cpu_user_pct),MIN(cpu_user_pct),MAX(cpu_user_pct),COUNT(cpu_user_pct),
	AVG(cpu_system_pct),MIN(cpu_system_pct),MAX(cpu_system_pct),COUNT(cpu_system_pct),
	AVG(cpu_iowait_pct),MIN(cpu_iowait_pct),MAX(cpu_iowait_pct),COUNT(cpu_iowait_pct),
	AVG(cpu_steal_pct),MIN(cpu_steal_pct),MAX(cpu_steal_pct),COUNT(cpu_steal_pct),
	AVG(load_1),MIN(load_1),MAX(load_1),COUNT(load_1),
	AVG(load_5),MIN(load_5),MAX(load_5),COUNT(load_5),
	AVG(load_15),MIN(load_15),MAX(load_15),COUNT(load_15),
	AVG(swap_used_bytes),MIN(swap_used_bytes),MAX(swap_used_bytes),COUNT(swap_used_bytes),
	AVG(disk_read_bps),MIN(disk_read_bps),MAX(disk_read_bps),COUNT(disk_read_bps),
	AVG(disk_write_bps),MIN(disk_write_bps),MAX(disk_write_bps),COUNT(disk_write_bps),
	AVG(disk_read_iops+disk_write_iops),MIN(disk_read_iops+disk_write_iops),MAX(disk_read_iops+disk_write_iops),COUNT(disk_read_iops+disk_write_iops)
FROM host_samples_10s WHERE ts<? GROUP BY (ts/%d)`, tier.name, bucket, bucket, bucket)
		if _, err := m.db.ExecContext(ctx, host, cutoff); err != nil {
			return err
		}
		resource := fmt.Sprintf(`INSERT OR REPLACE INTO resource_rollups_%s(ts,resource_id,cpu_avg,cpu_min,cpu_max,sample_count,memory_avg,memory_min,memory_max,memory_count,network_rx_avg,network_rx_min,network_rx_max,network_rx_count,network_tx_avg,network_tx_min,network_tx_max,network_tx_count,block_read_avg,block_read_min,block_read_max,block_read_count,block_write_avg,block_write_min,block_write_max,block_write_count)
SELECT (ts/%d)*%d,resource_id,AVG(cpu_host_pct),MIN(cpu_host_pct),MAX(cpu_host_pct),COUNT(cpu_host_pct),AVG(memory_working_set_bytes),MIN(memory_working_set_bytes),MAX(memory_working_set_bytes),COUNT(memory_working_set_bytes),AVG(network_rx_bps),MIN(network_rx_bps),MAX(network_rx_bps),COUNT(network_rx_bps),AVG(network_tx_bps),MIN(network_tx_bps),MAX(network_tx_bps),COUNT(network_tx_bps),AVG(block_read_bps),MIN(block_read_bps),MAX(block_read_bps),COUNT(block_read_bps),AVG(block_write_bps),MIN(block_write_bps),MAX(block_write_bps),COUNT(block_write_bps)
FROM resource_samples_10s WHERE ts<? GROUP BY resource_id,(ts/%d)`, tier.name, bucket, bucket, bucket)
		if _, err := m.db.ExecContext(ctx, resource, cutoff); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) runRollups(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		_ = m.RollupOnce(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
