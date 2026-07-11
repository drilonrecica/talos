// SPDX-License-Identifier: AGPL-3.0-only
package resources

import "github.com/drilonrecica/talos/internal/metrics"

func RollupStatus(states []metrics.ResourceStatus) metrics.ResourceStatus {
	best := metrics.StatusHealthy
	rank := map[metrics.ResourceStatus]int{metrics.StatusHealthy: 0, metrics.StatusPaused: 1, metrics.StatusUnknown: 2, metrics.StatusDegraded: 3, metrics.StatusDown: 4}
	for _, s := range states {
		if rank[s] > rank[best] {
			best = s
		}
	}
	return best
}
