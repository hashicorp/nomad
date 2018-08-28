package device

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "github.com/golang/protobuf/proto"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
	"google.golang.org/grpc/status"
)

func TestDevicePlugin_PluginInfo(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	const (
		apiVersion    = "v0.1.0"
		pluginVersion = "v0.2.1"
		pluginName    = "mock"
	)

	knownType := func() (*base.PluginInfoResponse, error) {
		info := &base.PluginInfoResponse{
			Type:             base.PluginTypeDevice,
			PluginApiVersion: apiVersion,
			PluginVersion:    pluginVersion,
			Name:             pluginName,
		}
		return info, nil
	}
	unknownType := func() (*base.PluginInfoResponse, error) {
		info := &base.PluginInfoResponse{
			Type:             "bad",
			PluginApiVersion: apiVersion,
			PluginVersion:    pluginVersion,
			Name:             pluginName,
		}
		return info, nil
	}

	mock := &MockDevicePlugin{
		MockPlugin: &base.MockPlugin{
			PluginInfoF: knownType,
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	resp, err := impl.PluginInfo()
	require.NoError(err)
	require.Equal(apiVersion, resp.PluginApiVersion)
	require.Equal(pluginVersion, resp.PluginVersion)
	require.Equal(pluginName, resp.Name)
	require.Equal(base.PluginTypeDevice, resp.Type)

	// Swap the implementation to return an unknown type
	mock.PluginInfoF = unknownType
	_, err = impl.PluginInfo()
	require.Error(err)
	require.Contains(err.Error(), "unknown type")
}

func TestDevicePlugin_ConfigSchema(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	mock := &MockDevicePlugin{
		MockPlugin: &base.MockPlugin{
			ConfigSchemaF: func() (*hclspec.Spec, error) {
				return base.TestSpec, nil
			},
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	specOut, err := impl.ConfigSchema()
	require.NoError(err)
	require.True(pb.Equal(base.TestSpec, specOut))
}

func TestDevicePlugin_SetConfig(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	var receivedData []byte
	mock := &MockDevicePlugin{
		MockPlugin: &base.MockPlugin{
			ConfigSchemaF: func() (*hclspec.Spec, error) {
				return base.TestSpec, nil
			},
			SetConfigF: func(data []byte) error {
				receivedData = data
				return nil
			},
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	config := cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("v1"),
		"bar": cty.NumberIntVal(1337),
		"baz": cty.BoolVal(true),
	})
	cdata, err := msgpack.Marshal(config, config.Type())
	require.NoError(err)
	require.NoError(impl.SetConfig(cdata))
	require.Equal(cdata, receivedData)

	// Decode the value back
	var actual base.TestConfig
	require.NoError(structs.Decode(receivedData, &actual))
	require.Equal("v1", actual.Foo)
	require.EqualValues(1337, actual.Bar)
	require.True(actual.Baz)
}

func TestDevicePlugin_Fingerprint(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	devices1 := []*DeviceGroup{
		{
			Vendor: "nvidia",
			Type:   DeviceTypeGPU,
			Name:   "foo",
		},
	}
	devices2 := []*DeviceGroup{
		{
			Vendor: "nvidia",
			Type:   DeviceTypeGPU,
			Name:   "foo",
		},
		{
			Vendor: "nvidia",
			Type:   DeviceTypeGPU,
			Name:   "bar",
		},
	}

	mock := &MockDevicePlugin{
		FingerprintF: func(ctx context.Context) (<-chan *FingerprintResponse, error) {
			outCh := make(chan *FingerprintResponse, 1)
			go func() {
				// Send two messages
				for _, devs := range [][]*DeviceGroup{devices1, devices2} {
					select {
					case <-ctx.Done():
						return
					case outCh <- &FingerprintResponse{Devices: devs}:
					}
				}
				close(outCh)
				return
			}()
			return outCh, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the stream
	stream, err := impl.Fingerprint(ctx)
	require.NoError(err)

	// Get the first message
	var first *FingerprintResponse
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case first = <-stream:
	}

	require.NoError(first.Error)
	require.EqualValues(devices1, first.Devices)

	// Get the second message
	var second *FingerprintResponse
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case second = <-stream:
	}

	require.NoError(second.Error)
	require.EqualValues(devices2, second.Devices)

	select {
	case _, ok := <-stream:
		require.False(ok)
	case <-time.After(1 * time.Second):
		t.Fatal("stream should be closed")
	}
}

func TestDevicePlugin_Fingerprint_StreamErr(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ferr := fmt.Errorf("mock fingerprinting failed")
	mock := &MockDevicePlugin{
		FingerprintF: func(ctx context.Context) (<-chan *FingerprintResponse, error) {
			outCh := make(chan *FingerprintResponse, 1)
			go func() {
				// Send the error
				select {
				case <-ctx.Done():
					return
				case outCh <- &FingerprintResponse{Error: ferr}:
				}

				close(outCh)
				return
			}()
			return outCh, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the stream
	stream, err := impl.Fingerprint(ctx)
	require.NoError(err)

	// Get the first message
	var first *FingerprintResponse
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case first = <-stream:
	}

	errStatus := status.Convert(ferr)
	require.EqualError(first.Error, errStatus.Err().Error())
}

func TestDevicePlugin_Fingerprint_CancelCtx(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	mock := &MockDevicePlugin{
		FingerprintF: func(ctx context.Context) (<-chan *FingerprintResponse, error) {
			outCh := make(chan *FingerprintResponse, 1)
			go func() {
				<-ctx.Done()
				close(outCh)
				return
			}()
			return outCh, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())

	// Get the stream
	stream, err := impl.Fingerprint(ctx)
	require.NoError(err)

	// Get the first message
	select {
	case <-time.After(testutil.Timeout(10 * time.Millisecond)):
	case _ = <-stream:
		t.Fatal("bad value")
	}

	// Cancel the context
	cancel()

	// Make sure we are done
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	case v := <-stream:
		require.Error(v.Error)
		require.EqualError(v.Error, context.Canceled.Error())
	}
}

func TestDevicePlugin_Reserve(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	reservation := &ContainerReservation{
		Envs: map[string]string{
			"foo": "bar",
		},
		Mounts: []*Mount{
			{
				TaskPath: "foo",
				HostPath: "bar",
				ReadOnly: true,
			},
		},
		Devices: []*DeviceSpec{
			{
				TaskPath:    "foo",
				HostPath:    "bar",
				CgroupPerms: "rx",
			},
		},
	}

	var received []string
	mock := &MockDevicePlugin{
		ReserveF: func(devices []string) (*ContainerReservation, error) {
			received = devices
			return reservation, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	req := []string{"a", "b"}
	containerRes, err := impl.Reserve(req)
	require.NoError(err)
	require.EqualValues(req, received)
	require.EqualValues(reservation, containerRes)
}

func TestDevicePlugin_Stats(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	devices1 := []*DeviceGroupStats{
		{
			Vendor: "nvidia",
			Type:   DeviceTypeGPU,
			Name:   "foo",
			InstanceStats: map[string]*DeviceStats{
				"1": {
					Summary: &StatValue{
						IntNumeratorVal:   10,
						IntDenominatorVal: 20,
						Unit:              "MB",
						Desc:              "Unit test",
					},
				},
			},
		},
	}
	devices2 := []*DeviceGroupStats{
		{
			Vendor: "nvidia",
			Type:   DeviceTypeGPU,
			Name:   "foo",
			InstanceStats: map[string]*DeviceStats{
				"1": {
					Summary: &StatValue{
						FloatNumeratorVal:   10.0,
						FloatDenominatorVal: 20.0,
						Unit:                "MB",
						Desc:                "Unit test",
					},
				},
			},
		},
		{
			Vendor: "nvidia",
			Type:   DeviceTypeGPU,
			Name:   "bar",
			InstanceStats: map[string]*DeviceStats{
				"1": {
					Summary: &StatValue{
						StringVal: "foo",
						Unit:      "MB",
						Desc:      "Unit test",
					},
				},
			},
		},
		{
			Vendor: "nvidia",
			Type:   DeviceTypeGPU,
			Name:   "baz",
			InstanceStats: map[string]*DeviceStats{
				"1": {
					Summary: &StatValue{
						BoolVal: true,
						Unit:    "MB",
						Desc:    "Unit test",
					},
				},
			},
		},
	}

	mock := &MockDevicePlugin{
		StatsF: func(ctx context.Context) (<-chan *StatsResponse, error) {
			outCh := make(chan *StatsResponse, 1)
			go func() {
				// Send two messages
				for _, devs := range [][]*DeviceGroupStats{devices1, devices2} {
					select {
					case <-ctx.Done():
						return
					case outCh <- &StatsResponse{Groups: devs}:
					}
				}
				close(outCh)
				return
			}()
			return outCh, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the stream
	stream, err := impl.Stats(ctx)
	require.NoError(err)

	// Get the first message
	var first *StatsResponse
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case first = <-stream:
	}

	require.NoError(first.Error)
	require.EqualValues(devices1, first.Groups)

	// Get the second message
	var second *StatsResponse
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case second = <-stream:
	}

	require.NoError(second.Error)
	require.EqualValues(devices2, second.Groups)

	select {
	case _, ok := <-stream:
		require.False(ok)
	case <-time.After(1 * time.Second):
		t.Fatal("stream should be closed")
	}
}

func TestDevicePlugin_Stats_StreamErr(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ferr := fmt.Errorf("mock stats failed")
	mock := &MockDevicePlugin{
		StatsF: func(ctx context.Context) (<-chan *StatsResponse, error) {
			outCh := make(chan *StatsResponse, 1)
			go func() {
				// Send the error
				select {
				case <-ctx.Done():
					return
				case outCh <- &StatsResponse{Error: ferr}:
				}

				close(outCh)
				return
			}()
			return outCh, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the stream
	stream, err := impl.Stats(ctx)
	require.NoError(err)

	// Get the first message
	var first *StatsResponse
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case first = <-stream:
	}

	errStatus := status.Convert(ferr)
	require.EqualError(first.Error, errStatus.Err().Error())
}

func TestDevicePlugin_Stats_CancelCtx(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	mock := &MockDevicePlugin{
		StatsF: func(ctx context.Context) (<-chan *StatsResponse, error) {
			outCh := make(chan *StatsResponse, 1)
			go func() {
				<-ctx.Done()
				close(outCh)
				return
			}()
			return outCh, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		base.PluginTypeBase:   &base.PluginBase{Impl: mock},
		base.PluginTypeDevice: &PluginDevice{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(base.PluginTypeDevice)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(DevicePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())

	// Get the stream
	stream, err := impl.Stats(ctx)
	require.NoError(err)

	// Get the first message
	select {
	case <-time.After(testutil.Timeout(10 * time.Millisecond)):
	case _ = <-stream:
		t.Fatal("bad value")
	}

	// Cancel the context
	cancel()

	// Make sure we are done
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timeout")
	case v := <-stream:
		require.Error(v.Error)
		require.EqualError(v.Error, context.Canceled.Error())
	}
}
