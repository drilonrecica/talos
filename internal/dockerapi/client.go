// SPDX-License-Identifier: AGPL-3.0-only
package dockerapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"
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
	TTY                                     bool
	Labels                                  map[string]string
	Environment                             map[string]string
	Networks                                []string
	Mounts                                  []Mount
}
type Mount struct{ Source, Destination, Type string }
type Event struct{ ID, Action, Time string }
type Version struct {
	EngineVersion string
	APIVersion    string
}
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

// sdkClient is the complete compile-time boundary to the Docker SDK. Keeping
// the full SDK client behind this interface prevents mutation methods from
// becoming available to the adapter implementation.
type sdkClient interface {
	ContainerList(context.Context, dockerclient.ContainerListOptions) (dockerclient.ContainerListResult, error)
	ContainerInspect(context.Context, string, dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error)
	ContainerStats(context.Context, string, dockerclient.ContainerStatsOptions) (dockerclient.ContainerStatsResult, error)
	ContainerLogs(context.Context, string, dockerclient.ContainerLogsOptions) (dockerclient.ContainerLogsResult, error)
	Events(context.Context, dockerclient.EventsListOptions) dockerclient.EventsResult
	ServerVersion(context.Context, dockerclient.ServerVersionOptions) (dockerclient.ServerVersionResult, error)
}

type Engine struct {
	client sdkClient
	close  func() error
}

func NewEngine(socketPath string) (*Engine, error) {
	client, err := dockerclient.New(dockerclient.WithHost("unix://" + socketPath))
	if err != nil {
		return nil, err
	}
	return &Engine{client: client, close: client.Close}, nil
}
func (e *Engine) Close() error {
	if e == nil || e.close == nil {
		return nil
	}
	return e.close()
}
func (e *Engine) List(ctx context.Context) ([]Container, error) {
	values, err := e.client.ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}
	result := make([]Container, 0, len(values.Items))
	for _, value := range values.Items {
		name := ""
		if len(value.Names) > 0 {
			name = strings.TrimPrefix(value.Names[0], "/")
		}
		result = append(result, Container{ID: value.ID, Name: name, Image: value.Image})
	}
	return result, nil
}
func (e *Engine) Inspect(ctx context.Context, id string) (Inspect, error) {
	response, err := e.client.ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return Inspect{}, err
	}
	value := response.Container
	result := Inspect{ID: value.ID, Name: strings.TrimPrefix(value.Name, "/"), Created: value.Created}
	if value.Config != nil {
		result.Image, result.Labels = value.Config.Image, value.Config.Labels
		result.TTY = value.Config.Tty
		result.Environment = allowedEnvironment(value.Config.Env)
	}
	if value.State != nil {
		result.State = string(value.State.Status)
		if value.State.Health != nil {
			result.Health = string(value.State.Health.Status)
		}
	}
	if value.NetworkSettings != nil {
		for name := range value.NetworkSettings.Networks {
			result.Networks = append(result.Networks, name)
		}
	}
	for _, mount := range value.Mounts {
		result.Mounts = append(result.Mounts, Mount{Source: mount.Source, Destination: mount.Destination, Type: string(mount.Type)})
	}
	return result, nil
}

type LogOptions struct {
	Since, Until time.Time
	Tail         int
	Follow       bool
}

// LogClient is a deliberately narrow, read-only extension to Client.
type LogClient interface {
	ReadLogs(context.Context, string, LogOptions, func(stream, line string) error) error
}

func (e *Engine) ReadLogs(ctx context.Context, id string, options LogOptions, emit func(string, string) error) error {
	inspect, err := e.client.ContainerInspect(ctx, id, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return err
	}
	tail := "all"
	if options.Tail > 0 {
		tail = fmt.Sprint(options.Tail)
	}
	reader, err := e.client.ContainerLogs(ctx, id, dockerclient.ContainerLogsOptions{
		ShowStdout: true, ShowStderr: true, Timestamps: true, Follow: options.Follow,
		Since: formatDockerTime(options.Since), Until: formatDockerTime(options.Until), Tail: tail,
	})
	if err != nil {
		return err
	}
	defer reader.Close()
	if inspect.Container.Config != nil && inspect.Container.Config.Tty {
		return scanLogLines(ctx, reader, "stdout", emit)
	}
	return scanMultiplexedLogs(ctx, reader, emit)
}

func formatDockerTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func scanLogLines(ctx context.Context, reader io.Reader, stream string, emit func(string, string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(stream, scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func scanMultiplexedLogs(ctx context.Context, reader io.Reader, emit func(string, string) error) error {
	buffered := bufio.NewReader(reader)
	for {
		header := make([]byte, 8)
		if _, err := io.ReadFull(buffered, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}
		length := binary.BigEndian.Uint32(header[4:])
		if length > 1<<20 {
			return fmt.Errorf("docker log frame exceeds limit")
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(buffered, payload); err != nil {
			return err
		}
		stream := "stdout"
		if header[0] == 2 {
			stream = "stderr"
		}
		if header[0] != 1 && header[0] != 2 {
			return fmt.Errorf("invalid docker log stream %d", header[0])
		}
		for _, line := range bytes.Split(bytes.TrimSuffix(payload, []byte{'\n'}), []byte{'\n'}) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := emit(stream, string(line)); err != nil {
				return err
			}
		}
	}
}

var allowedEnvironmentKeys = map[string]bool{
	"COOLIFY_FQDN":          true,
	"COOLIFY_URL":           true,
	"COOLIFY_RESOURCE_UUID": true,
}

// allowedEnvironment deliberately returns only non-secret Coolify metadata.
// Container environments commonly contain credentials and must never enter caches or APIs.
func allowedEnvironment(values []string) map[string]string {
	result := map[string]string{}
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		if ok && allowedEnvironmentKeys[key] {
			result[key] = raw
		}
	}
	return result
}
func (e *Engine) Stats(ctx context.Context, id string) (Stats, error) {
	response, err := e.client.ContainerStats(ctx, id, dockerclient.ContainerStatsOptions{Stream: false})
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
	stream := e.client.Events(ctx, dockerclient.EventsListOptions{})
	messages, errs := stream.Messages, stream.Err
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
	value, err := e.client.ServerVersion(ctx, dockerclient.ServerVersionOptions{})
	return Version{EngineVersion: value.Version, APIVersion: value.APIVersion}, err
}
func (e *Engine) Diagnostics(ctx context.Context) (Diagnostics, error) {
	values, err := e.client.ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	return Diagnostics{Containers: len(values.Items)}, err
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
func (l *Limited) ReadLogs(ctx context.Context, id string, options LogOptions, emit func(string, string) error) error {
	logs, ok := l.Client.(LogClient)
	if !ok {
		return fmt.Errorf("docker log access is unavailable")
	}
	return l.with(ctx, func() error { return logs.ReadLogs(ctx, id, options, emit) })
}
func (l *Limited) Close() error {
	if closer, ok := l.Client.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
