// SPDX-License-Identifier: AGPL-3.0-only
package alerts

import (
	"testing"
	"time"
)

func TestAdvanceLifecycle(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rule := Rule{TriggerDuration: 2 * time.Minute, RecoveryDuration: time.Minute, Repeat: 2 * time.Hour, Cooldown: 5 * time.Minute}
	tr := Advance(now, State{}, true, false, false, rule)
	if tr.State.Phase != Pending {
		t.Fatalf("phase=%s", tr.State.Phase)
	}
	tr = Advance(now.Add(2*time.Minute), tr.State, true, false, false, rule)
	if !tr.Triggered || tr.State.Phase != Firing {
		t.Fatalf("expected trigger: %+v", tr)
	}
	tr = Advance(now.Add(3*time.Minute), tr.State, false, true, false, rule)
	if tr.State.Phase != Recovering {
		t.Fatalf("phase=%s", tr.State.Phase)
	}
	tr = Advance(now.Add(4*time.Minute), tr.State, false, true, false, rule)
	if !tr.Resolved || tr.State.Phase != Healthy || tr.State.CooldownUntil == nil {
		t.Fatalf("expected resolution: %+v", tr)
	}
}
func TestAdvanceSilenceAndRepeat(t *testing.T) {
	now := time.Now().UTC()
	rule := Rule{TriggerDuration: time.Minute, Repeat: 2 * time.Hour}
	state := State{Phase: Pending, Since: now.Add(-time.Minute)}
	tr := Advance(now, state, true, false, true, rule)
	if tr.Triggered {
		t.Fatal("silence triggered alert")
	}
	tr = Advance(now, state, true, false, false, rule)
	if !tr.Triggered {
		t.Fatal("alert did not fire when silence expired")
	}
	tr = Advance(now.Add(2*time.Hour), tr.State, true, false, false, rule)
	if !tr.Repeated {
		t.Fatal("repeat not emitted")
	}
}
func TestAdvancePersistsPendingDuration(t *testing.T) {
	now := time.Now().UTC()
	rule := Rule{TriggerDuration: 5 * time.Minute}
	persisted := State{Phase: Pending, Since: now.Add(-5 * time.Minute)}
	if tr := Advance(now, persisted, true, false, false, rule); !tr.Triggered {
		t.Fatal("restart reset pending duration")
	}
}

func TestEffectiveRulePrefersTargetOverride(t *testing.T) {
	rules := []Rule{{ID: "global", Family: FamilyFilesystemWarning, Enabled: true, ScopeType: "global"}, {ID: "override", Family: FamilyFilesystemWarning, Enabled: true, ScopeType: "filesystem", ScopeID: "root"}}
	if got := effectiveRule(rules, FamilyFilesystemWarning, "filesystem", "root").ID; got != "override" {
		t.Fatalf("effective rule=%q", got)
	}
	if got := effectiveRule(rules, FamilyFilesystemWarning, "filesystem", "data").ID; got != "global" {
		t.Fatalf("fallback rule=%q", got)
	}
}

func TestSilencePresets(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	for preset, want := range map[string]time.Duration{"30m": 30 * time.Minute, "1h": time.Hour, "4h": 4 * time.Hour} {
		end, err := SilencePresetEnd(now, preset, time.Time{})
		if err != nil || end.Sub(now) != want {
			t.Fatalf("preset %s end=%v err=%v", preset, end, err)
		}
	}
	tomorrow, err := SilencePresetEnd(now, "tomorrow", time.Time{})
	if err != nil || tomorrow.Hour() != 0 || tomorrow.Day() != 14 {
		t.Fatalf("tomorrow=%v err=%v", tomorrow, err)
	}
}
