package base

import (
	"testing"

	pb "github.com/golang/protobuf/proto"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
)

func TestBasePlugin_PluginInfo_GRPC(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	const (
		apiVersion    = "v0.1.0"
		pluginVersion = "v0.2.1"
		pluginName    = "mock"
	)

	knownType := func() (*PluginInfoResponse, error) {
		info := &PluginInfoResponse{
			Type:             PluginTypeDriver,
			PluginApiVersion: apiVersion,
			PluginVersion:    pluginVersion,
			Name:             pluginName,
		}
		return info, nil
	}
	unknownType := func() (*PluginInfoResponse, error) {
		info := &PluginInfoResponse{
			Type:             "bad",
			PluginApiVersion: apiVersion,
			PluginVersion:    pluginVersion,
			Name:             pluginName,
		}
		return info, nil
	}

	mock := &MockPlugin{
		PluginInfoF: knownType,
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		PluginTypeBase: &PluginBase{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(PluginTypeBase)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(BasePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	resp, err := impl.PluginInfo()
	require.NoError(err)
	require.Equal(apiVersion, resp.PluginApiVersion)
	require.Equal(pluginVersion, resp.PluginVersion)
	require.Equal(pluginName, resp.Name)
	require.Equal(PluginTypeDriver, resp.Type)

	// Swap the implementation to return an unknown type
	mock.PluginInfoF = unknownType
	_, err = impl.PluginInfo()
	require.Error(err)
	require.Contains(err.Error(), "unknown type")
}

func TestBasePlugin_ConfigSchema(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	mock := &MockPlugin{
		ConfigSchemaF: func() (*hclspec.Spec, error) {
			return TestSpec, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		PluginTypeBase: &PluginBase{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(PluginTypeBase)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(BasePlugin)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	specOut, err := impl.ConfigSchema()
	require.NoError(err)
	require.True(pb.Equal(TestSpec, specOut))
}

func TestBasePlugin_SetConfig(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	var receivedData []byte
	mock := &MockPlugin{
		ConfigSchemaF: func() (*hclspec.Spec, error) {
			return TestSpec, nil
		},
		SetConfigF: func(data []byte) error {
			receivedData = data
			return nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		PluginTypeBase: &PluginBase{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(PluginTypeBase)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	impl, ok := raw.(BasePlugin)
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
	var actual TestConfig
	require.NoError(structs.Decode(receivedData, &actual))
	require.Equal("v1", actual.Foo)
	require.EqualValues(1337, actual.Bar)
	require.True(actual.Baz)
}
