// SPDX-License-Identifier: AGPL-3.0-only

// Package alerts implements Binnacle's deterministic local alert catalog.
package alerts

import (
	"fmt"
	"time"
)

type Severity string

const (
	Warning  Severity = "warning"
	Critical Severity = "critical"
)

type Phase string

const (
	Healthy    Phase = "healthy"
	Pending    Phase = "pending"
	Firing     Phase = "firing"
	Recovering Phase = "recovering"
)

const (
	FamilyHostCPU            = "host_cpu_warning"
	FamilyHostMemory         = "host_memory_warning"
	FamilyFilesystemWarning  = "filesystem_warning"
	FamilyFilesystemCritical = "filesystem_critical"
	FamilyInodeWarning       = "inode_warning"
	FamilyInodeCritical      = "inode_critical"
	FamilyRestartStorm       = "restart_storm"
	FamilyOOMLoop            = "oom_loop"
	FamilyRequiredCheck      = "required_check_failure"
	FamilyOptionalCheck      = "optional_check_failure"
	FamilyDockerDown         = "docker_collector_down"
	FamilyPersistence        = "persistence_failure"
)

type Rule struct {
	ID                       string        `json:"id"`
	Family                   string        `json:"family"`
	Name                     string        `json:"name"`
	BuiltIn                  bool          `json:"builtIn"`
	Enabled                  bool          `json:"enabled"`
	Severity                 Severity      `json:"severity"`
	ScopeType                string        `json:"scopeType"`
	ScopeID                  string        `json:"scopeId,omitempty"`
	Threshold                *float64      `json:"threshold,omitempty"`
	RecoveryThreshold        *float64      `json:"recoveryThreshold,omitempty"`
	TriggerDuration          time.Duration `json:"-"`
	RecoveryDuration         time.Duration `json:"-"`
	Window                   time.Duration `json:"-"`
	Cooldown                 time.Duration `json:"-"`
	Repeat                   time.Duration `json:"-"`
	SuppressDuringDeployment bool          `json:"suppressDuringDeployment"`
}
type Alert struct {
	ID             string     `json:"id"`
	DedupKey       string     `json:"dedupKey"`
	RuleID         string     `json:"ruleId"`
	Family         string     `json:"family"`
	Severity       Severity   `json:"severity"`
	TargetType     string     `json:"targetType"`
	TargetID       string     `json:"targetId"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"startedAt"`
	ResolvedAt     *time.Time `json:"resolvedAt,omitempty"`
	LastObservedAt time.Time  `json:"lastObservedAt"`
	ObservedValue  *float64   `json:"observedValue,omitempty"`
	Message        string     `json:"message"`
}
type Silence struct {
	ID        string    `json:"id"`
	ScopeType string    `json:"scopeType"`
	ScopeID   string    `json:"scopeId,omitempty"`
	Reason    string    `json:"reason"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

func SilencePresetEnd(now time.Time, preset string, custom time.Time) (time.Time, error) {
	switch preset {
	case "30m":
		return now.Add(30 * time.Minute), nil
	case "1h":
		return now.Add(time.Hour), nil
	case "4h":
		return now.Add(4 * time.Hour), nil
	case "tomorrow":
		next := now.AddDate(0, 0, 1)
		return time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location()), nil
	case "custom":
		if custom.After(now) {
			return custom, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid silence end")
}

func ptr(v float64) *float64 { return &v }
func DefaultRules() []Rule {
	return []Rule{
		{ID: "builtin-host-cpu-warning", Family: FamilyHostCPU, Name: "Host CPU warning", BuiltIn: true, Enabled: true, Severity: Warning, ScopeType: "global", Threshold: ptr(90), RecoveryThreshold: ptr(80), TriggerDuration: 5 * time.Minute, RecoveryDuration: 2 * time.Minute},
		{ID: "builtin-host-memory-warning", Family: FamilyHostMemory, Name: "Host memory warning", BuiltIn: true, Enabled: true, Severity: Warning, ScopeType: "global", Threshold: ptr(85), RecoveryThreshold: ptr(80), TriggerDuration: 10 * time.Minute, RecoveryDuration: 2 * time.Minute},
		{ID: "builtin-filesystem-warning", Family: FamilyFilesystemWarning, Name: "Filesystem warning", BuiltIn: true, Enabled: true, Severity: Warning, ScopeType: "global", Threshold: ptr(80), RecoveryThreshold: ptr(80), TriggerDuration: 5 * time.Minute, RecoveryDuration: 2 * time.Minute},
		{ID: "builtin-filesystem-critical", Family: FamilyFilesystemCritical, Name: "Filesystem critical", BuiltIn: true, Enabled: true, Severity: Critical, ScopeType: "global", Threshold: ptr(95), RecoveryThreshold: ptr(90), TriggerDuration: 2 * time.Minute, RecoveryDuration: 2 * time.Minute},
		{ID: "builtin-inode-warning", Family: FamilyInodeWarning, Name: "Inode warning", BuiltIn: true, Enabled: true, Severity: Warning, ScopeType: "global", Threshold: ptr(80), RecoveryThreshold: ptr(80), TriggerDuration: 5 * time.Minute, RecoveryDuration: 2 * time.Minute},
		{ID: "builtin-inode-critical", Family: FamilyInodeCritical, Name: "Inode critical", BuiltIn: true, Enabled: true, Severity: Critical, ScopeType: "global", Threshold: ptr(95), RecoveryThreshold: ptr(90), TriggerDuration: 2 * time.Minute, RecoveryDuration: 2 * time.Minute},
		{ID: "builtin-restart-storm", Family: FamilyRestartStorm, Name: "Restart storm", BuiltIn: true, Enabled: true, Severity: Warning, ScopeType: "global", Threshold: ptr(3), TriggerDuration: 0, RecoveryDuration: 10 * time.Minute, Window: 10 * time.Minute, SuppressDuringDeployment: true},
		{ID: "builtin-oom-loop", Family: FamilyOOMLoop, Name: "OOM loop", BuiltIn: true, Enabled: true, Severity: Critical, ScopeType: "global", Threshold: ptr(2), TriggerDuration: 0, RecoveryDuration: 10 * time.Minute, Window: 10 * time.Minute},
		{ID: "builtin-required-check", Family: FamilyRequiredCheck, Name: "Required check failure", BuiltIn: true, Enabled: true, Severity: Critical, ScopeType: "global", TriggerDuration: 2 * time.Minute, RecoveryDuration: 2 * time.Minute, SuppressDuringDeployment: true},
		{ID: "builtin-optional-check", Family: FamilyOptionalCheck, Name: "Optional check failure", BuiltIn: true, Enabled: true, Severity: Warning, ScopeType: "global", TriggerDuration: 2 * time.Minute, RecoveryDuration: 2 * time.Minute, SuppressDuringDeployment: true},
		{ID: "builtin-docker-down", Family: FamilyDockerDown, Name: "Docker collector down", BuiltIn: true, Enabled: true, Severity: Critical, ScopeType: "global", TriggerDuration: 2 * time.Minute, RecoveryDuration: time.Minute},
		{ID: "builtin-persistence-failure", Family: FamilyPersistence, Name: "Persistence failure", BuiltIn: true, Enabled: true, Severity: Critical, ScopeType: "global", TriggerDuration: 0, RecoveryDuration: time.Minute},
	}
}
