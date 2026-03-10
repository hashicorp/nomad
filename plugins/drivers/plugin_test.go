// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package drivers

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/ci"
	//	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"

	"github.com/shoenig/test/must"
)

var (
	errTest = errors.New("testing error")
	taskID  = "test-task-id"
)

func TestDriverPlugin_PluginInfo(t *testing.T) {
	ci.Parallel(t)

	var (
		apiVersions = []string{"v0.1.0", "v0.2.0"}
	)

	const (
		pluginVersion = "v0.2.1"
		pluginName    = "mock_driver"
	)

	knownType := func() (*base.PluginInfoResponse, error) {
		info := &base.PluginInfoResponse{
			Type:              base.PluginTypeDriver,
			PluginApiVersions: apiVersions,
			PluginVersion:     pluginVersion,
			Name:              pluginName,
		}
		return info, nil
	}
	unknownType := func() (*base.PluginInfoResponse, error) {
		info := &base.PluginInfoResponse{
			Type:              "bad",
			PluginApiVersions: apiVersions,
			PluginVersion:     pluginVersion,
			Name:              pluginName,
		}
		return info, nil
	}

	mock := &MockDriverPlugin{
		MockPlugin: &base.MockPlugin{
			PluginInfoF: knownType,
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, true, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDriver{impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DriverPlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	resp, err := impl.PluginInfo()
	must.NoError(t, err)
	must.Eq(t, apiVersions, resp.PluginApiVersions)
	must.Eq(t, pluginVersion, resp.PluginVersion)
	must.Eq(t, pluginName, resp.Name)
	must.Eq(t, base.PluginTypeDriver, resp.Type)

	// Swap the implementation to return an unknown type
	mock.PluginInfoF = unknownType
	_, err = impl.PluginInfo()
	must.ErrorContains(t, err, "unknown type")
}

func TestDriverPlugin_ConfigSchema(t *testing.T) {
	ci.Parallel(t)

	mock := &MockDriverPlugin{
		MockPlugin: &base.MockPlugin{
			ConfigSchemaF: func() (*hclspec.Spec, error) {
				return base.TestSpec, nil
			},
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, true, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDriver{impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DriverPlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	specOut, err := impl.ConfigSchema()
	must.NoError(t, err)
	must.True(t, pb.Equal(base.TestSpec, specOut))
}

func TestDriverPlugin_SetConfig(t *testing.T) {
	ci.Parallel(t)

	var receivedData []byte
	mock := &MockDriverPlugin{
		MockPlugin: &base.MockPlugin{
			PluginInfoF: func() (*base.PluginInfoResponse, error) {
				return &base.PluginInfoResponse{
					Type:              base.PluginTypeDevice,
					PluginApiVersions: []string{"v0.0.1"},
					PluginVersion:     "v0.0.1",
					Name:              "mock_device",
				}, nil
			},
			ConfigSchemaF: func() (*hclspec.Spec, error) {
				return base.TestSpec, nil
			},
			SetConfigF: func(cfg *base.Config) error {
				receivedData = cfg.PluginConfig
				return nil
			},
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, true, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDriver{impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DriverPlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	config := cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("v1"),
		"bar": cty.NumberIntVal(1337),
		"baz": cty.BoolVal(true),
	})
	cdata, err := msgpack.Marshal(config, config.Type())
	must.NoError(t, err)
	must.NoError(t, impl.SetConfig(&base.Config{PluginConfig: cdata}))
	must.Eq(t, cdata, receivedData)

	// Decode the value back
	var actual base.TestConfig
	must.NoError(t, structs.Decode(receivedData, &actual))
	must.Eq(t, "v1", actual.Foo)
	must.Eq(t, 1337, actual.Bar)
	must.True(t, actual.Baz)
}

func makeTestPlugin(t *testing.T, mock DriverPlugin) DriverPlugin {
	t.Helper()

	logger := testlog.HCLogger(t)
	if testing.Verbose() {
		logger.SetLevel(hclog.Trace)
	} else {
		logger.SetLevel(hclog.Info)
	}

	client, server := plugin.TestPluginGRPCConn(t, true, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDriver: &PluginDriver{impl: mock, logger: logger},
	})

	t.Cleanup(func() {
		server.Stop()
		client.Close()
	})

	raw, err := client.Dispense(base.PluginTypeDriver)
	must.NoError(t, err)
	impl, ok := raw.(DriverPlugin)
	must.True(t, ok, must.Sprintf("not valid DriverPlugin - %#v", impl))
	return impl
}

func TestDriverPlugin_Capabilities(t *testing.T) {
	ci.Parallel(t)

	t.Run("ok", func(t *testing.T) {
		caps := &Capabilities{SendSignals: true, Exec: true, FSIsolation: "none"}
		mock := &MockDriverPlugin{
			CapabilitiesFn: func() (*Capabilities, error) {
				return caps, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		result, err := impl.Capabilities()
		must.NoError(t, err)
		must.Eq(t, caps, result)
	})

	t.Run("bad", func(t *testing.T) {
		mock := &MockDriverPlugin{
			CapabilitiesFn: func() (*Capabilities, error) {
				return nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		_, err := impl.Capabilities()
		must.ErrorContains(t, err, errTest.Error())
	})
}

func TestDriverPlugin_Fingerprint(t *testing.T) {
	ci.Parallel(t)
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		f := &Fingerprint{
			Health:            HealthStateHealthy,
			HealthDescription: "very healthy",
		}
		mock := &MockDriverPlugin{
			FingerprintFn: func(_ context.Context) (<-chan *Fingerprint, error) {
				outCh := make(chan *Fingerprint, 1)
				outCh <- f
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		fCh, err := impl.Fingerprint(ctx)
		must.NoError(t, err)

		var result *Fingerprint
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-fCh:
		}
		must.Eq(t, f, result)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			FingerprintFn: func(_ context.Context) (<-chan *Fingerprint, error) {
				return nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		fCh, err := impl.Fingerprint(ctx)
		must.NoError(t, err)

		var result *Fingerprint
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-fCh:
		}
		must.ErrorContains(t, result.Err, errTest.Error())
	})

	t.Run("driver error", func(t *testing.T) {
		f := &Fingerprint{
			Err: errTest,
		}
		mock := &MockDriverPlugin{
			FingerprintFn: func(_ context.Context) (<-chan *Fingerprint, error) {
				outCh := make(chan *Fingerprint, 1)
				outCh <- f
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		fCh, err := impl.Fingerprint(ctx)
		must.NoError(t, err)

		var result *Fingerprint
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-fCh:
		}

		must.ErrorContains(t, result.Err, errTest.Error())
	})

	t.Run("channel closed", func(t *testing.T) {
		mock := &MockDriverPlugin{
			FingerprintFn: func(_ context.Context) (<-chan *Fingerprint, error) {
				outCh := make(chan *Fingerprint, 1)
				close(outCh)
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		fCh, err := impl.Fingerprint(ctx)
		must.NoError(t, err)

		var result *Fingerprint
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-fCh:
		}

		must.ErrorContains(t, result.Err, ErrChannelClosed.Error())

		select {
		case _, ok := <-fCh:
			must.False(t, ok)
		case <-time.After(10 * time.Millisecond):
			t.Fatal("channel not closed")
		}
	})
}

func TestDriverPlugin_RecoverTask(t *testing.T) {
	ci.Parallel(t)

	t.Run("ok", func(t *testing.T) {
		handle := &TaskHandle{
			Version: 42,
			State:   TaskStateRunning,
			Config: &TaskConfig{
				ID:        "test-id",
				Resources: &Resources{},
			},
		}
		mock := &MockDriverPlugin{
			RecoverTaskFn: func(th *TaskHandle) error {
				must.Eq(t, handle, th)
				must.NotNil(t, th.Config)
				must.Eq(t, handle.Config.ID, th.Config.ID)
				return nil
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.RecoverTask(handle)
		must.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			RecoverTaskFn: func(*TaskHandle) error {
				return errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.RecoverTask(&TaskHandle{})
		must.ErrorContains(t, err, errTest.Error())
	})
}

func TestDriverPlugin_StartTask(t *testing.T) {
	ci.Parallel(t)

	t.Run("ok", func(t *testing.T) {
		config := &TaskConfig{
			ID:        "test-id",
			Resources: &Resources{},
		}
		handle := &TaskHandle{
			Version: 42,
			Config:  config,
			State:   TaskStateRunning,
		}
		dnet := &DriverNetwork{IP: "127.0.0.1", PortMap: map[string]int{}}
		mock := &MockDriverPlugin{
			StartTaskFn: func(tc *TaskConfig) (*TaskHandle, *DriverNetwork, error) {
				must.Eq(t, config, tc)
				return handle, dnet, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		resultHandle, resultNet, err := impl.StartTask(config)
		must.NoError(t, err)
		must.Eq(t, dnet, resultNet)
		must.Eq(t, handle, resultHandle)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			StartTaskFn: func(*TaskConfig) (*TaskHandle, *DriverNetwork, error) {
				return nil, nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		_, _, err := impl.StartTask(&TaskConfig{})
		must.ErrorContains(t, err, errTest.Error())
	})
}

func TestDriverPlugin_WaitTask(t *testing.T) {
	ci.Parallel(t)
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		e := &ExitResult{
			ExitCode: 1,
			Signal:   2,
		}
		mock := &MockDriverPlugin{
			WaitTaskFn: func(ctx context.Context, tid string) (<-chan *ExitResult, error) {
				must.Eq(t, taskID, tid)

				outCh := make(chan *ExitResult, 1)
				outCh <- e
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		wCh, err := impl.WaitTask(ctx, taskID)
		must.NoError(t, err)

		var result *ExitResult
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-wCh:
		}

		must.Eq(t, e, result)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			WaitTaskFn: func(context.Context, string) (<-chan *ExitResult, error) {
				return nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		wCh, err := impl.WaitTask(ctx, taskID)
		must.NoError(t, err)

		var result *ExitResult
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-wCh:
		}

		must.ErrorContains(t, result.Err, errTest.Error())
	})

	t.Run("driver error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			WaitTaskFn: func(context.Context, string) (<-chan *ExitResult, error) {
				outCh := make(chan *ExitResult, 1)
				outCh <- &ExitResult{Err: errTest}
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		wCh, err := impl.WaitTask(ctx, taskID)
		must.NoError(t, err)

		var result *ExitResult
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-wCh:
		}

		must.ErrorContains(t, result.Err, errTest.Error())

	})

	t.Run("channel closed", func(t *testing.T) {
		mock := &MockDriverPlugin{
			WaitTaskFn: func(context.Context, string) (<-chan *ExitResult, error) {
				outCh := make(chan *ExitResult, 1)
				close(outCh)
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		wCh, err := impl.WaitTask(ctx, taskID)
		must.NoError(t, err)

		var result *ExitResult
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-wCh:
		}

		must.ErrorContains(t, result.Err, ErrChannelClosed.Error())

		select {
		case _, ok := <-wCh:
			must.False(t, ok)
		case <-time.After(10 * time.Millisecond):
			t.Fatal("channel not closed")
		}
	})
}

func TestDriverPlugin_StopTask(t *testing.T) {
	ci.Parallel(t)
	signal := "test-signal"

	t.Run("ok", func(t *testing.T) {
		timeout := 42 * time.Second
		mock := &MockDriverPlugin{
			StopTaskFn: func(tid string, to time.Duration, sig string) error {
				must.Eq(t, taskID, tid)
				must.Eq(t, timeout, to)
				must.Eq(t, signal, sig)
				return nil
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.StopTask(taskID, timeout, signal)
		must.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			StopTaskFn: func(string, time.Duration, string) error {
				return errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.StopTask(taskID, 0, signal)
		must.ErrorContains(t, err, errTest.Error())
	})
}

func TestDriverPlugin_DestroyTask(t *testing.T) {
	ci.Parallel(t)

	t.Run("ok", func(t *testing.T) {
		mock := &MockDriverPlugin{
			DestroyTaskFn: func(tid string, force bool) error {
				must.Eq(t, taskID, tid)
				must.False(t, force, must.Sprint("force should be false"))
				return nil
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.DestroyTask(taskID, false)
		must.NoError(t, err)
	})

	t.Run("ok - force", func(t *testing.T) {
		mock := &MockDriverPlugin{
			DestroyTaskFn: func(tid string, force bool) error {
				must.Eq(t, taskID, tid)
				must.True(t, force, must.Sprint("force should be true"))
				return nil
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.DestroyTask(taskID, true)
		must.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			DestroyTaskFn: func(string, bool) error {
				return errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.DestroyTask(taskID, false)
		must.ErrorContains(t, err, errTest.Error())
	})
}

func TestDriverPlugin_InspectTask(t *testing.T) {
	ci.Parallel(t)

	t.Run("ok", func(t *testing.T) {
		mock := &MockDriverPlugin{
			InspectTaskFn: func(tid string) (*TaskStatus, error) {
				must.Eq(t, taskID, tid)
				return &TaskStatus{
					ID:    tid,
					State: TaskStateRunning,
				}, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		result, err := impl.InspectTask(taskID)
		must.NoError(t, err)
		expected := &TaskStatus{
			ID:         taskID,
			State:      TaskStateRunning,
			ExitResult: new(ExitResult),
		}
		must.Eq(t, expected, result)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			InspectTaskFn: func(string) (*TaskStatus, error) {
				return nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		_, err := impl.InspectTask(taskID)
		must.ErrorContains(t, err, errTest.Error())
	})
}

func TestDriverPlugin_TaskStats(t *testing.T) {
	ci.Parallel(t)
	ctx := context.Background()

	t.Run("ok - empty", func(t *testing.T) {
		duration := 42 * time.Second

		mock := &MockDriverPlugin{
			TaskStatsFn: func(ctx context.Context, tid string, dur time.Duration) (<-chan *TaskResourceUsage, error) {
				must.Eq(t, taskID, tid)
				must.Eq(t, duration, dur)
				outCh := make(chan *TaskResourceUsage, 1)
				outCh <- &TaskResourceUsage{}
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		tCh, err := impl.TaskStats(ctx, taskID, duration)
		must.NoError(t, err)

		var result *TaskResourceUsage
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-tCh:
		}

		expected := &TaskResourceUsage{
			ResourceUsage: &ResourceUsage{
				MemoryStats: &MemoryStats{},
				CpuStats:    &CpuStats{},
			},
			Pids: map[string]*ResourceUsage{},
		}
		must.Eq(t, expected, result)
	})

	t.Run("ok", func(t *testing.T) {
		tru := &TaskResourceUsage{
			ResourceUsage: &ResourceUsage{
				MemoryStats: &MemoryStats{
					RSS:   42,
					Usage: 42,
				},
				CpuStats: &CpuStats{
					ThrottledTime: 42,
				},
			},
			Pids: map[string]*ResourceUsage{
				"42": {
					MemoryStats: &MemoryStats{
						RSS:   42,
						Usage: 42,
					},
					CpuStats: &CpuStats{
						ThrottledTime: 42,
					},
				},
			},
		}
		duration := 42 * time.Second
		mock := &MockDriverPlugin{
			TaskStatsFn: func(ctx context.Context, tid string, dur time.Duration) (<-chan *TaskResourceUsage, error) {
				must.Eq(t, taskID, tid)
				must.Eq(t, duration, dur)
				outCh := make(chan *TaskResourceUsage, 1)
				outCh <- tru
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		tCh, err := impl.TaskStats(ctx, taskID, duration)
		must.NoError(t, err)

		var result *TaskResourceUsage
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-tCh:
		}

		expected := &TaskResourceUsage{
			ResourceUsage: &ResourceUsage{
				MemoryStats: &MemoryStats{
					RSS:      42,
					Usage:    42,
					Measured: []string{},
				},
				CpuStats: &CpuStats{
					ThrottledTime: 42,
					Measured:      []string{},
				},
			},
			Pids: map[string]*ResourceUsage{
				"42": {
					MemoryStats: &MemoryStats{
						RSS:      42,
						Usage:    42,
						Measured: []string{},
					},
					CpuStats: &CpuStats{
						ThrottledTime: 42,
						Measured:      []string{},
					},
				},
			},
		}

		must.Eq(t, expected, result)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			TaskStatsFn: func(context.Context, string, time.Duration) (<-chan *TaskResourceUsage, error) {
				return nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		tCh, err := impl.TaskStats(ctx, taskID, 0)
		must.NoError(t, err)

		var result *TaskResourceUsage
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-tCh:
		}

		must.Nil(t, result)
	})

	t.Run("channel closed", func(t *testing.T) {
		mock := &MockDriverPlugin{
			TaskStatsFn: func(context.Context, string, time.Duration) (<-chan *TaskResourceUsage, error) {
				outCh := make(chan *TaskResourceUsage, 1)
				close(outCh)
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		tCh, err := impl.TaskStats(ctx, taskID, 0)
		must.NoError(t, err)

		var result *TaskResourceUsage
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-tCh:
		}

		must.Nil(t, result)

		select {
		case _, ok := <-tCh:
			must.False(t, ok)
		case <-time.After(10 * time.Millisecond):
			t.Fatal("channel not closed")
		}
	})
}

func TestDriverPlugin_TaskEvents(t *testing.T) {
	ci.Parallel(t)
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		te := &TaskEvent{
			TaskID:   "test-task-id",
			TaskName: "test-task-name",
		}
		mock := &MockDriverPlugin{
			TaskEventsFn: func(context.Context) (<-chan *TaskEvent, error) {
				outCh := make(chan *TaskEvent, 1)
				outCh <- te
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		tCh, err := impl.TaskEvents(ctx)
		must.NoError(t, err)

		var result *TaskEvent
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-tCh:
		}

		must.Eq(t, te, result)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			TaskEventsFn: func(context.Context) (<-chan *TaskEvent, error) {
				return nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		tCh, err := impl.TaskEvents(ctx)
		must.NoError(t, err)

		var result *TaskEvent
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-tCh:
		}

		must.ErrorContains(t, result.Err, errTest.Error())
	})

	t.Run("closed channel", func(t *testing.T) {
		mock := &MockDriverPlugin{
			TaskEventsFn: func(context.Context) (<-chan *TaskEvent, error) {
				outCh := make(chan *TaskEvent, 1)
				close(outCh)
				return outCh, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		tCh, err := impl.TaskEvents(ctx)
		must.NoError(t, err)

		var result *TaskEvent
		select {
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout")
		case result = <-tCh:
		}

		must.Nil(t, result)
		select {
		case _, ok := <-tCh:
			if ok {
				t.Fatal("channel not closed")
			}
		default:
		}
	})
}

func TestDriverPlugin_SignalTask(t *testing.T) {
	ci.Parallel(t)
	signal := "test-signal"

	t.Run("ok", func(t *testing.T) {
		mock := &MockDriverPlugin{
			SignalTaskFn: func(tid string, sig string) error {
				must.Eq(t, taskID, tid)
				must.Eq(t, signal, sig)
				return nil
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.SignalTask(taskID, signal)
		must.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			SignalTaskFn: func(string, string) error {
				return errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		err := impl.SignalTask(taskID, signal)
		must.ErrorContains(t, err, errTest.Error())
	})
}

func TestDriverPlugin_ExecTask(t *testing.T) {
	ci.Parallel(t)

	t.Run("ok", func(t *testing.T) {
		commands := []string{"first-cmd", "second-cmd"}
		timeout := 42 * time.Second
		etr := &ExecTaskResult{
			Stdout: []byte("stdout content"),
			Stderr: []byte("stderr content"),
			ExitResult: &ExitResult{
				ExitCode: 42,
			},
		}
		mock := &MockDriverPlugin{
			ExecTaskFn: func(tid string, cmds []string, to time.Duration) (*ExecTaskResult, error) {
				must.Eq(t, taskID, tid)
				must.Eq(t, commands, cmds)
				must.Eq(t, timeout, to)
				return etr, nil
			},
		}

		impl := makeTestPlugin(t, mock)
		result, err := impl.ExecTask(taskID, commands, timeout)
		must.NoError(t, err)
		must.Eq(t, etr, result)
	})

	t.Run("error", func(t *testing.T) {
		mock := &MockDriverPlugin{
			ExecTaskFn: func(string, []string, time.Duration) (*ExecTaskResult, error) {
				return nil, errTest
			},
		}

		impl := makeTestPlugin(t, mock)
		_, err := impl.ExecTask(taskID, []string{}, 0)
		must.ErrorContains(t, err, err.Error())
	})
}
