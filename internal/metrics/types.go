// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import (
	"fmt"
	"strings"
	"time"
)

type ResourceID string
type ContainerID string
type BootIdentity string
type Sequence uint64
type Unit string

const (
	UnitBytes          Unit = "bytes"
	UnitBytesPerSecond Unit = "bytes_per_second"
	UnitPercent        Unit = "percent"
	UnitCount          Unit = "count"
)

func (u Unit) Valid() bool {
	switch u {
	case UnitBytes, UnitBytesPerSecond, UnitPercent, UnitCount:
		return true
	}
	return false
}
func (id ResourceID) Valid() bool  { return strings.HasPrefix(string(id), "res_") && len(id) > 4 }
func (id ContainerID) Valid() bool { return len(id) >= 12 }

type ResourceStatus string

const (
	StatusHealthy  ResourceStatus = "healthy"
	StatusPaused   ResourceStatus = "paused"
	StatusUnknown  ResourceStatus = "unknown"
	StatusDegraded ResourceStatus = "degraded"
	StatusDown     ResourceStatus = "down"
	StatusArchived ResourceStatus = "archived"
)

func (s ResourceStatus) Valid() bool {
	switch s {
	case StatusHealthy, StatusPaused, StatusUnknown, StatusDegraded, StatusDown, StatusArchived:
		return true
	}
	return false
}

type CollectorState string

const (
	CollectorHealthy  CollectorState = "healthy"
	CollectorDegraded CollectorState = "degraded"
	CollectorDown     CollectorState = "down"
	CollectorUnknown  CollectorState = "unknown"
)

func (s CollectorState) Valid() bool {
	switch s {
	case CollectorHealthy, CollectorDegraded, CollectorDown, CollectorUnknown:
		return true
	}
	return false
}

type HostObservation struct {
	At                   time.Time `json:"at"`
	CPUPercent           *float64  `json:"cpuPct"`
	CPUUserPercent       *float64  `json:"cpuUserPct"`
	CPUSystemPercent     *float64  `json:"cpuSystemPct"`
	CPUIOWaitPercent     *float64  `json:"cpuIOWaitPct"`
	CPUStealPercent      *float64  `json:"cpuStealPct"`
	MemoryUsedBytes      *int64    `json:"memoryUsedBytes"`
	MemoryTotalBytes     *int64    `json:"memoryTotalBytes"`
	MemoryAvailableBytes *int64    `json:"memoryAvailableBytes"`
	MemoryPercent        *float64  `json:"memoryPct"`
	MemoryCachedBytes    *int64    `json:"memoryCachedBytes"`
	MemoryBuffersBytes   *int64    `json:"memoryBuffersBytes"`
	SwapUsedBytes        *int64    `json:"swapUsedBytes"`
	SwapTotalBytes       *int64    `json:"swapTotalBytes"`
	SwapPercent          *float64  `json:"swapPct"`
	Load1                *float64  `json:"load1"`
	Load5                *float64  `json:"load5"`
	Load15               *float64  `json:"load15"`
	NetworkRXBPS         *float64  `json:"networkRxBps"`
	NetworkTXBPS         *float64  `json:"networkTxBps"`
	NetworkRXPacketsPS   *float64  `json:"networkRxPacketsPs"`
	NetworkTXPacketsPS   *float64  `json:"networkTxPacketsPs"`
	NetworkRXErrorsDelta *int64    `json:"networkRxErrorsDelta"`
	NetworkTXErrorsDelta *int64    `json:"networkTxErrorsDelta"`
	NetworkRXDropsDelta  *int64    `json:"networkRxDropsDelta"`
	NetworkTXDropsDelta  *int64    `json:"networkTxDropsDelta"`
	DiskReadBPS          *float64  `json:"diskReadBps"`
	DiskWriteBPS         *float64  `json:"diskWriteBps"`
	DiskReadIOPS         *float64  `json:"diskReadIops"`
	DiskWriteIOPS        *float64  `json:"diskWriteIops"`
	DiskUsedBytes        *int64    `json:"diskUsedBytes"`
	DiskTotalBytes       *int64    `json:"diskTotalBytes"`
	UptimeSeconds        *float64  `json:"uptimeSeconds"`
}
type ContainerObservation struct {
	ID             ContainerID    `json:"id"`
	ResourceID     ResourceID     `json:"resourceId"`
	At             time.Time      `json:"at"`
	CPUHostPercent *float64       `json:"cpuHostPct"`
	MemoryBytes    *int64         `json:"memoryBytes"`
	RXBPS          *float64       `json:"rxBps"`
	TXBPS          *float64       `json:"txBps"`
	BlockReadBPS   *float64       `json:"blockReadBps"`
	BlockWriteBPS  *float64       `json:"blockWriteBps"`
	Status         ResourceStatus `json:"status"`
}
type ResourceComponent struct {
	ID             ContainerID    `json:"id"`
	Name           string         `json:"name"`
	Status         ResourceStatus `json:"status"`
	RuntimeState   string         `json:"runtimeState,omitempty"`
	HealthStatus   string         `json:"healthStatus,omitempty"`
	CPUHostPercent *float64       `json:"cpuHostPct,omitempty"`
	MemoryBytes    *int64         `json:"memoryBytes,omitempty"`
	RXBPS          *float64       `json:"rxBps,omitempty"`
	TXBPS          *float64       `json:"txBps,omitempty"`
	BlockReadBPS   *float64       `json:"blockReadBps,omitempty"`
	BlockWriteBPS  *float64       `json:"blockWriteBps,omitempty"`
	PIDs           *uint64        `json:"pids,omitempty"`
}
type ResourceSnapshot struct {
	ID             ResourceID          `json:"id"`
	Name           string              `json:"name"`
	Status         ResourceStatus      `json:"status"`
	SignalStatus   ResourceStatus      `json:"signalStatus"`
	CPUHostPercent *float64            `json:"cpuHostPct"`
	MemoryBytes    *int64              `json:"memoryBytes"`
	RXBPS          *float64            `json:"rxBps"`
	TXBPS          *float64            `json:"txBps"`
	BlockReadBPS   *float64            `json:"blockReadBps"`
	BlockWriteBPS  *float64            `json:"blockWriteBps"`
	LastSeenAt     time.Time           `json:"lastSeenAt"`
	Category       string              `json:"category,omitempty"`
	Context        string              `json:"context,omitempty"`
	Project        string              `json:"project,omitempty"`
	Environment    string              `json:"environment,omitempty"`
	Infrastructure bool                `json:"infrastructure,omitempty"`
	Components     []ResourceComponent `json:"components,omitempty"`
	StableKey      string              `json:"-"`
	SourceKind     string              `json:"-"`
	ManualName     bool                `json:"-"`
	ManualContext  bool                `json:"-"`
}
type CollectorHealth struct {
	Name    string         `json:"name"`
	State   CollectorState `json:"state"`
	Reason  string         `json:"reason,omitempty"`
	FreshAt time.Time      `json:"freshAt"`
}
type Event struct {
	ID                Sequence    `json:"id"`
	At                time.Time   `json:"at"`
	Type              string      `json:"type"`
	ResourceID        ResourceID  `json:"resourceId,omitempty"`
	ContainerInstance ContainerID `json:"containerInstanceId,omitempty"`
	Severity          string      `json:"severity,omitempty"`
	Message           string      `json:"message"`
	Details           string      `json:"details,omitempty"`
	CorrelationKey    string      `json:"correlationKey,omitempty"`
}
type Snapshot struct {
	Sequence     Sequence                   `json:"seq"`
	At           time.Time                  `json:"ts"`
	BootIdentity BootIdentity               `json:"bootIdentity"`
	Host         HostObservation            `json:"host"`
	Resources    []ResourceSnapshot         `json:"resources"`
	Collectors   map[string]CollectorHealth `json:"collectors"`
}
type FilesystemObservation struct {
	At                time.Time `json:"at"`
	MountKey          string    `json:"mountKey"`
	MountPoint        string    `json:"mountPoint"`
	FSType            string    `json:"fsType"`
	TotalBytes        *int64    `json:"totalBytes"`
	UsedBytes         *int64    `json:"usedBytes"`
	AvailableBytes    *int64    `json:"availableBytes"`
	UsedPercent       *float64  `json:"usedPct"`
	InodesTotal       *int64    `json:"inodesTotal"`
	InodesUsed        *int64    `json:"inodesUsed"`
	InodesUsedPercent *float64  `json:"inodesUsedPct"`
}

type PersistenceBatch struct {
	Snapshot    Snapshot
	Events      []Event
	Filesystems []FilesystemObservation
}

type TimeRange struct{ From, To time.Time }

func (r TimeRange) Validate() error {
	if r.From.IsZero() || r.To.IsZero() || !r.From.Before(r.To) {
		return fmt.Errorf("time range start must precede end")
	}
	return nil
}
func UTC(t time.Time) time.Time { return t.UTC() }
