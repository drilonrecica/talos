// SPDX-License-Identifier: AGPL-3.0-only
package docker

import "github.com/drilonrecica/binnacle/internal/metrics"

type HealthTracker struct {
	Failures int
	State    metrics.CollectorState
}

func (t *HealthTracker) Observe(err error) metrics.CollectorState {
	if err == nil {
		t.Failures = 0
		t.State = metrics.CollectorHealthy
		return t.State
	}
	t.Failures++
	switch {
	case t.Failures >= 6:
		t.State = metrics.CollectorDown
	case t.Failures >= 3:
		t.State = metrics.CollectorDegraded
	default:
		if t.State == "" {
			t.State = metrics.CollectorHealthy
		}
	}
	return t.State
}
