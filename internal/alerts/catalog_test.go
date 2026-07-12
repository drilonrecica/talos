// SPDX-License-Identifier: AGPL-3.0-only
package alerts

import (
	"testing"
	"time"
)

func TestEventAndHealthDefaultSemantics(t *testing.T) {
	byFamily := map[string]Rule{}
	for _, r := range DefaultRules() {
		byFamily[r.Family] = r
	}
	tests := []struct {
		family            string
		threshold         float64
		trigger, recovery time.Duration
		suppressed        bool
	}{{FamilyRestartStorm, 3, 0, 10 * time.Minute, true}, {FamilyOOMLoop, 2, 0, 10 * time.Minute, false}, {FamilyRequiredCheck, 0, 2 * time.Minute, 2 * time.Minute, true}, {FamilyOptionalCheck, 0, 2 * time.Minute, 2 * time.Minute, true}, {FamilyDockerDown, 0, 2 * time.Minute, time.Minute, false}, {FamilyPersistence, 0, 0, time.Minute, false}}
	for _, tt := range tests {
		r, ok := byFamily[tt.family]
		if !ok {
			t.Fatalf("missing %s", tt.family)
		}
		if r.TriggerDuration != tt.trigger || r.RecoveryDuration != tt.recovery || r.SuppressDuringDeployment != tt.suppressed {
			t.Fatalf("rule %s=%+v", tt.family, r)
		}
		if tt.threshold > 0 && (r.Threshold == nil || *r.Threshold != tt.threshold) {
			t.Fatalf("threshold %s", tt.family)
		}
	}
}
