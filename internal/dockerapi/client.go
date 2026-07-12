// SPDX-License-Identifier: AGPL-3.0-only
package dockerapi

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	eventtypes "github.com/docker/docker/api/types/events"
	dockerclient "github.com/docker/docker/client"
)

// Client is deliberately read-only; mutation operations are not part of Binnacle's boundary.
type Client interface {
	List(context.Context) ([]Container, error)
	Inspect(context.Context, string) (Inspect, error)
	Stats(context.Context, string) (Stats, error)
	Events(context.Context) <-chan Event
	Version(context.Context) (Version, error)
	Diagnostics(context.Context) (Diagnostics, error)
}
type Container struct{ ID, Name, Image string }
type Inspect struct {
	ID, Name, Image, Created, State, Health string
	Labels                                  map[string]string
	Networks                                []string
	Mounts                                  []Mount
}
type Mount struct{ Source, Destination, Type string }
type Event struct{ ID, Action, Time string }
type Version struct{ APIVersion string }
type Diagnostics struct{ Containers int }
type Stats struct {
	CPU    CPUStats
	Memory MemoryStats
	IO     IOCounters
	PIDs   *uint64
}
type CPUStats struct {
	TotalUsage, SystemUsage uint64
	OnlineCPUs              int
}
type MemoryStats struct{ Usage, Limit, InactiveFile uint64 }
type IOCounters struct{ RX, TX, Read, Write uint64 }
type Limited struct {
	Client Client
	sem    chan struct{}
	once   sync.Once
}

type Engine struct{ client *dockerclient.Client }

func NewEngine(socketPath string) (*Engine, error) {
	client, err := dockerclient.NewClientWithOpts(dockerclient.WithHost("unix://"+socketPath), dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Engine{client: client}, nil
}
func (e *Engine) Close() error {
	if e == nil || e.client == nil {
		return nil
	}
	return e.client.Close()
}
func (e *Engine) List(ctx context.Context) ([]Container, error) {
	values, err := e.client.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	result := make([]Container, 0, len(values))
	for _, value := range values {
		name := ""
		if len(value.Names) > 0 {
			name = strings.TrimPrefix(value.Names[0], "/")
		}
		result = append(result, Container{ID: value.ID, Name: name, Image: value.Image})
	}
	return result, nil
}
func (e *Engine) Inspect(ctx context.Context, id string) (Inspect, error) {
	value, err := e.client.ContainerInspect(ctx, id)
	if err != nil {
		return Inspect{}, err
	}
	result := Inspect{ID: value.ID, Name: strings.TrimPrefix(value.Name, "/"), Created: value.Created}
	if value.Config != nil {
		result.Image, result.Labels = value.Config.Image, value.Config.Labels
	}
	if value.State != nil {
		result.State = value.State.Status
		if value.State.Health != nil {
			result.Health = value.State.Health.Status
		}
	}
	for name := range value.NetworkSettings.Networks {
		result.Networks = append(result.Networks, name)
	}
	for _, mount := range value.Mounts {
		result.Mounts = append(result.Mounts, Mount{Source: mount.Source, Destination: mount.Destination, Type: string(mount.Type)})
	}
	return result, nil
}
func (e *Engine) Stats(ctx context.Context, id string) (Stats, error) {
	response, err := e.client.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return Stats{}, err
	}
	defer response.Body.Close()
	var value containertypes.StatsResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&value); err != nil {
		return Stats{}, err
	}
	result := Stats{CPU: CPUStats{TotalUsage: value.CPUStats.CPUUsage.TotalUsage, SystemUsage: value.CPUStats.SystemUsage, OnlineCPUs: int(value.CPUStats.OnlineCPUs)}, Memory: MemoryStats{Usage: value.MemoryStats.Usage, Limit: value.MemoryStats.Limit, InactiveFile: value.MemoryStats.Stats["inactive_file"]}}
	for _, network := range value.Networks {
		result.IO.RX += network.RxBytes
		result.IO.TX += network.TxBytes
	}
	for _, entry := range value.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			result.IO.Read += entry.Value
		case "write":
			result.IO.Write += entry.Value
		}
	}
	if value.PidsStats.Current > 0 {
		pids := value.PidsStats.Current
		result.PIDs = &pids
	}
	return result, nil
}
func (e *Engine) Events(ctx context.Context) <-chan Event {
	output := make(chan Event, 32)
	messages, errs := e.client.Events(ctx, eventtypes.ListOptions{})
	go func() {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-errs:
				if !ok {
					errs = nil
				}
				if errs == nil && messages == nil {
					return
				}
			case message, ok := <-messages:
				if !ok {
					messages = nil
					if errs == nil {
						return
					}
					continue
				}
				at := time.Unix(message.Time, 0).UTC().Format(time.RFC3339)
				select {
				case output <- Event{ID: message.Actor.ID, Action: string(message.Action), Time: at}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return output
}
func (e *Engine) Version(ctx context.Context) (Version, error) {
	value, err := e.client.ServerVersion(ctx)
	return Version{APIVersion: value.APIVersion}, err
}
func (e *Engine) Diagnostics(ctx context.Context) (Diagnostics, error) {
	values, err := e.client.ContainerList(ctx, containertypes.ListOptions{All: true})
	return Diagnostics{Containers: len(values)}, err
}

func New(client Client, max int) *Limited {
	if max < 1 {
		max = 1
	}
	return &Limited{Client: client, sem: make(chan struct{}, max)}
}
func (l *Limited) with(ctx context.Context, fn func() error) error {
	select {
	case l.sem <- struct{}{}:
		defer func() { <-l.sem }()
		return fn()
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (l *Limited) List(ctx context.Context) ([]Container, error) {
	var out []Container
	err := l.with(ctx, func() error {
		var err error
		out, err = l.Client.List(ctx)
		return err
	})
	return out, err
}
func (l *Limited) Inspect(ctx context.Context, id string) (Inspect, error) {
	var out Inspect
	err := l.with(ctx, func() error {
		var err error
		out, err = l.Client.Inspect(ctx, id)
		return err
	})
	return out, err
}
func (l *Limited) Stats(ctx context.Context, id string) (Stats, error) {
	var out Stats
	err := l.with(ctx, func() error {
		var err error
		out, err = l.Client.Stats(ctx, id)
		return err
	})
	return out, err
}
func (l *Limited) Events(ctx context.Context) <-chan Event { return l.Client.Events(ctx) }
func (l *Limited) Version(ctx context.Context) (Version, error) {
	var out Version
	err := l.with(ctx, func() error {
		var err error
		out, err = l.Client.Version(ctx)
		return err
	})
	return out, err
}
func (l *Limited) Diagnostics(ctx context.Context) (Diagnostics, error) {
	var out Diagnostics
	err := l.with(ctx, func() error {
		var err error
		out, err = l.Client.Diagnostics(ctx)
		return err
	})
	return out, err
}
func (l *Limited) Close() error {
	if closer, ok := l.Client.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
