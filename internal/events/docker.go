// SPDX-License-Identifier: AGPL-3.0-only
package events

import (
	"github.com/drilonrecica/binnacle/internal/dockerapi"
	"github.com/drilonrecica/binnacle/internal/metrics"
	"strings"
	"time"
)

func NormalizeDocker(e dockerapi.Event, oomKilled bool) (metrics.Event, bool) {
	action := strings.ToLower(e.Action)
	kind := map[string]string{"start": "container_start", "stop": "container_stop", "die": "container_die", "destroy": "container_destroy", "create": "container_create", "rename": "container_rename", "oom": "container_oom", "restart": "container_restart"}[action]
	if strings.HasPrefix(action, "health_status") {
		kind = "container_health_status_change"
	}
	if kind == "" {
		return metrics.Event{}, false
	}
	if oomKilled || action == "oom" {
		kind = "container_oom"
	}
	at, _ := time.Parse(time.RFC3339, e.Time)
	return metrics.Event{At: at.UTC(), Type: kind, Message: kind + " for container", ContainerInstance: metrics.ContainerID(e.ID), Details: `{"action":"` + e.Action + `"}`, CorrelationKey: e.ID}, true
}
