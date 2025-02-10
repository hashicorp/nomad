// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"testing"

	pb "github.com/golang/protobuf/proto"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/shoenig/test/must"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
)

func TestBasePlugin_PluginInfo_GRPC(t *testing.T) {
	ci.Parallel(t)

	var (
		apiVersions = []string{"v0.1.0", "v0.1.1"}
	)

	const (
		pluginVersion = "v0.2.1"
		pluginName    = "mock"
	)

	knownType := func() (*PluginInfoResponse, error) {
		info := &PluginInfoResponse{
			Type:              PluginTypeDriver,
			PluginApiVersions: apiVersions,
			PluginVersion:     pluginVersion,
			Name:              pluginName,
		}
		return info, nil
	}
	unknownType := func() (*PluginInfoResponse, error) {
		info := &PluginInfoResponse{
			Type:              "bad",
			PluginApiVersions: apiVersions,
			PluginVersion:     pluginVersion,
			Name:              pluginName,
		}
		return info, nil
	}

	mock := &MockPlugin{
		PluginInfoF: knownType,
	}

	client, server := plugin.TestPluginGRPCConn(t, true, map[string]plugin.Plugin{
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
	must.NoError(t, err)
	must.Eq(t, apiVersions, resp.PluginApiVersions)
	must.Eq(t, pluginVersion, resp.PluginVersion)
	must.Eq(t, pluginName, resp.Name)
	must.Eq(t, PluginTypeDriver, resp.Type)

	// Swap the implementation to return an unknown type
	mock.PluginInfoF = unknownType
	_, err = impl.PluginInfo()
	must.ErrorContains(t, err, "unknown type")
}

func TestBasePlugin_ConfigSchema(t *testing.T) {
	ci.Parallel(t)

	mock := &MockPlugin{
		ConfigSchemaF: func() (*hclspec.Spec, error) {
			return TestSpec, nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, true, map[string]plugin.Plugin{
		PluginTypeBase: &PluginBase{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(PluginTypeBase)
	must.NoError(t, err)

	impl, ok := raw.(BasePlugin)
	must.True(t, ok)

	specOut, err := impl.ConfigSchema()
	must.NoError(t, err)
	must.True(t, pb.Equal(TestSpec, specOut))
}

func TestBasePlugin_SetConfig(t *testing.T) {
	ci.Parallel(t)

	var receivedData []byte
	mock := &MockPlugin{
		PluginInfoF: func() (*PluginInfoResponse, error) {
			return &PluginInfoResponse{Type: PluginTypeDriver}, nil
		},
		ConfigSchemaF: func() (*hclspec.Spec, error) {
			return TestSpec, nil
		},
		SetConfigF: func(cfg *Config) error {
			receivedData = cfg.PluginConfig
			return nil
		},
	}

	client, server := plugin.TestPluginGRPCConn(t, true, map[string]plugin.Plugin{
		PluginTypeBase: &PluginBase{Impl: mock},
	})
	defer server.Stop()
	defer client.Close()

	raw, err := client.Dispense(PluginTypeBase)
	must.NoError(t, err)
	impl, ok := raw.(BasePlugin)
	must.True(t, ok)

	config := cty.ObjectVal(map[string]cty.Value{
		"foo": cty.StringVal("v1"),
		"bar": cty.NumberIntVal(1337),
		"baz": cty.BoolVal(true),
	})

	cdata, err := msgpack.Marshal(config, config.Type())
	must.NoError(t, err)
	must.NoError(t, impl.SetConfig(&Config{PluginConfig: cdata}))
	must.Eq(t, cdata, receivedData)

	// Decode the value back
	var actual TestConfig
	must.NoError(t, structs.Decode(receivedData, &actual))
	must.Eq(t, "v1", actual.Foo)
	must.Eq(t, 1337, actual.Bar)
	must.True(t, actual.Baz)
}
