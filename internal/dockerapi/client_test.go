// SPDX-License-Identifier: AGPL-3.0-only
package dockerapi

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	eventtypes "github.com/moby/moby/api/types/events"
	networktypes "github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

func TestAllowedEnvironmentOnlyRetainsCoolifyMetadata(t *testing.T) {
	got := allowedEnvironment([]string{"COOLIFY_FQDN=api.example.com", "COOLIFY_URL=https://api.example.com", "COOLIFY_RESOURCE_UUID=uuid", "DATABASE_PASSWORD=secret", "INVALID"})
	if len(got) != 3 || got["COOLIFY_FQDN"] != "api.example.com" || got["DATABASE_PASSWORD"] != "" {
		t.Fatalf("allowed environment=%v", got)
	}
}

func TestSDKBoundaryContainsOnlyReadOperations(t *testing.T) {
	typeOfBoundary := reflect.TypeOf((*sdkClient)(nil)).Elem()
	want := []string{"ContainerInspect", "ContainerList", "ContainerLogs", "ContainerStats", "Events", "ServerVersion"}
	if typeOfBoundary.NumMethod() != len(want) {
		t.Fatalf("sdkClient has %d methods, want %d", typeOfBoundary.NumMethod(), len(want))
	}
	for index, name := range want {
		if method := typeOfBoundary.Method(index); method.Name != name {
			t.Fatalf("sdkClient method %d=%s, want %s", index, method.Name, name)
		}
	}
}

func TestEngineAdaptsReadOnlySDKOperations(t *testing.T) {
	sdk := newFakeSDK()
	engine := &Engine{client: sdk, close: sdk.Close}
	ctx := context.Background()

	containers, err := engine.List(ctx)
	if err != nil || len(containers) != 1 || containers[0].Name != "web" || !sdk.listOptions.All {
		t.Fatalf("List()=%+v options=%+v error=%v", containers, sdk.listOptions, err)
	}
	inspect, err := engine.Inspect(ctx, "container-id")
	if err != nil || inspect.ID != "container-id" || inspect.State != "running" || inspect.Environment["COOLIFY_FQDN"] != "web.example.com" {
		t.Fatalf("Inspect()=%+v error=%v", inspect, err)
	}
	stats, err := engine.Stats(ctx, "container-id")
	if err != nil || stats.CPU.TotalUsage != 10 || stats.Memory.InactiveFile != 5 || stats.IO.RX != 7 || stats.IO.Read != 9 || stats.PIDs == nil || *stats.PIDs != 3 {
		t.Fatalf("Stats()=%+v error=%v", stats, err)
	}
	if sdk.statsOptions.Stream || sdk.statsOptions.IncludePreviousSample {
		t.Fatalf("Stats options=%+v, want one-shot", sdk.statsOptions)
	}
	var logs []string
	err = engine.ReadLogs(ctx, "container-id", LogOptions{Tail: 10, Follow: true}, func(stream, line string) error {
		logs = append(logs, stream+":"+line)
		return nil
	})
	if err != nil || len(logs) != 1 || logs[0] != "stdout:line" || sdk.logsOptions.Tail != "10" || !sdk.logsOptions.Follow {
		t.Fatalf("ReadLogs()=%v options=%+v error=%v", logs, sdk.logsOptions, err)
	}
	event := <-engine.Events(ctx)
	if event.ID != "container-id" || event.Action != "start" {
		t.Fatalf("Events()=%+v", event)
	}
	version, err := engine.Version(ctx)
	if err != nil || version.EngineVersion != "29.5.1" || version.APIVersion != "1.55" {
		t.Fatalf("Version()=%+v error=%v", version, err)
	}
	diagnostics, err := engine.Diagnostics(ctx)
	if err != nil || diagnostics.Containers != 1 {
		t.Fatalf("Diagnostics()=%+v error=%v", diagnostics, err)
	}
	if err := engine.Close(); err != nil || !sdk.closed {
		t.Fatalf("Close() error=%v closed=%v", err, sdk.closed)
	}
}

type fakeSDK struct {
	listOptions  dockerclient.ContainerListOptions
	statsOptions dockerclient.ContainerStatsOptions
	logsOptions  dockerclient.ContainerLogsOptions
	closed       bool
}

func newFakeSDK() *fakeSDK { return &fakeSDK{} }

func (sdk *fakeSDK) ContainerList(_ context.Context, options dockerclient.ContainerListOptions) (dockerclient.ContainerListResult, error) {
	sdk.listOptions = options
	return dockerclient.ContainerListResult{Items: []containertypes.Summary{{ID: "container-id", Names: []string{"/web"}, Image: "example/web"}}}, nil
}

func (sdk *fakeSDK) ContainerInspect(_ context.Context, id string, _ dockerclient.ContainerInspectOptions) (dockerclient.ContainerInspectResult, error) {
	return dockerclient.ContainerInspectResult{Container: containertypes.InspectResponse{
		ID: id, Name: "/web", Created: "2026-07-15T00:00:00Z",
		State:           &containertypes.State{Status: "running", Health: &containertypes.Health{Status: containertypes.Healthy}},
		Config:          &containertypes.Config{Image: "example/web", Tty: true, Env: []string{"COOLIFY_FQDN=web.example.com", "SECRET=hidden"}},
		NetworkSettings: &containertypes.NetworkSettings{Networks: map[string]*networktypes.EndpointSettings{}},
	}}, nil
}

func (sdk *fakeSDK) ContainerStats(_ context.Context, _ string, options dockerclient.ContainerStatsOptions) (dockerclient.ContainerStatsResult, error) {
	sdk.statsOptions = options
	body := `{"cpu_stats":{"cpu_usage":{"total_usage":10},"system_cpu_usage":100,"online_cpus":2},"memory_stats":{"usage":50,"limit":100,"stats":{"inactive_file":5}},"networks":{"eth0":{"rx_bytes":7,"tx_bytes":8}},"blkio_stats":{"io_service_bytes_recursive":[{"op":"Read","value":9}]},"pids_stats":{"current":3}}`
	return dockerclient.ContainerStatsResult{Body: io.NopCloser(bytes.NewBufferString(body))}, nil
}

func (sdk *fakeSDK) ContainerLogs(_ context.Context, _ string, options dockerclient.ContainerLogsOptions) (dockerclient.ContainerLogsResult, error) {
	sdk.logsOptions = options
	return io.NopCloser(bytes.NewBufferString("line\n")), nil
}

func (sdk *fakeSDK) Events(context.Context, dockerclient.EventsListOptions) dockerclient.EventsResult {
	messages := make(chan eventtypes.Message, 1)
	errs := make(chan error)
	messages <- eventtypes.Message{Action: "start", Actor: eventtypes.Actor{ID: "container-id"}, Time: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC).Unix()}
	close(messages)
	close(errs)
	return dockerclient.EventsResult{Messages: messages, Err: errs}
}

func (sdk *fakeSDK) ServerVersion(context.Context, dockerclient.ServerVersionOptions) (dockerclient.ServerVersionResult, error) {
	return dockerclient.ServerVersionResult{Version: "29.5.1", APIVersion: "1.55"}, nil
}

func (sdk *fakeSDK) Close() error { sdk.closed = true; return nil }
