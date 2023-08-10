// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
)

type stringSliceFlags []string

func (i *stringSliceFlags) String() string {
	return "my string representation"
}

func (i *stringSliceFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// TestMain runs either the tests or runs a mock plugin based on the passed
// flags
func TestMain(m *testing.M) {
	var plugin, configSchema bool
	var name, pluginType, pluginVersion string
	var pluginApiVersions stringSliceFlags
	flag.BoolVar(&plugin, "plugin", false, "run binary as a plugin")
	flag.BoolVar(&configSchema, "config-schema", true, "return a config schema")
	flag.StringVar(&name, "name", "", "plugin name")
	flag.StringVar(&pluginType, "type", "", "plugin type")
	flag.StringVar(&pluginVersion, "version", "", "plugin version")
	flag.Var(&pluginApiVersions, "api-version", "supported plugin API version")
	flag.Parse()

	if plugin {
		if err := pluginMain(name, pluginType, pluginVersion, pluginApiVersions, configSchema); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	} else {
		os.Exit(m.Run())
	}
}

// pluginMain starts a mock plugin using the passed parameters
func pluginMain(name, pluginType, version string, apiVersions []string, config bool) error {
	// Validate passed parameters
	if name == "" || pluginType == "" {
		return fmt.Errorf("name and plugin type must be specified")
	}

	switch pluginType {
	case base.PluginTypeDevice:
	default:
		return fmt.Errorf("unsupported plugin type %q", pluginType)
	}

	// Create the mock plugin
	m := &mockPlugin{
		name:         name,
		ptype:        pluginType,
		version:      version,
		apiVersions:  apiVersions,
		configSchema: config,
	}

	// Build the plugin map
	pmap := map[string]plugin.Plugin{
		base.PluginTypeBase: &base.PluginBase{Impl: m},
	}
	switch pluginType {
	case base.PluginTypeDevice:
		pmap[base.PluginTypeDevice] = &device.PluginDevice{Impl: m}
	}

	// Serve the plugin
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: base.Handshake,
		Plugins:         pmap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})

	return nil
}

// mockFactory returns a PluginFactory method which creates the mock plugin with
// the passed parameters
func mockFactory(name, ptype, version string, apiVersions []string, configSchema bool) func(context.Context, log.Logger) interface{} {
	return func(ctx context.Context, log log.Logger) interface{} {
		return &mockPlugin{
			name:         name,
			ptype:        ptype,
			version:      version,
			apiVersions:  apiVersions,
			configSchema: configSchema,
		}
	}
}

// mockPlugin is a plugin that meets various plugin interfaces but is only
// useful for testing.
type mockPlugin struct {
	name         string
	ptype        string
	version      string
	apiVersions  []string
	configSchema bool

	// config is built on SetConfig
	config *mockPluginConfig

	// nomadconfig is set on SetConfig
	nomadConfig *base.AgentConfig

	// negotiatedApiVersion is the version of the api to use and is set on
	// SetConfig
	negotiatedApiVersion string
}

// mockPluginConfig is the configuration for the mock plugin
type mockPluginConfig struct {
	Foo string `codec:"foo"`
	Bar int    `codec:"bar"`

	// ResKey is a key that is populated in the Env map when a device is
	// reserved.
	ResKey string `codec:"res_key"`
}

// PluginInfo returns the plugin information based on the passed fields when
// building the mock plugin
func (m *mockPlugin) PluginInfo() (*base.PluginInfoResponse, error) {
	return &base.PluginInfoResponse{
		Type:              m.ptype,
		PluginApiVersions: m.apiVersions,
		PluginVersion:     m.version,
		Name:              m.name,
	}, nil
}

func (m *mockPlugin) ConfigSchema() (*hclspec.Spec, error) {
	if !m.configSchema {
		return nil, nil
	}

	// configSpec is the hclspec for parsing the mock's configuration
	configSpec := hclspec.NewObject(map[string]*hclspec.Spec{
		"foo":     hclspec.NewAttr("foo", "string", false),
		"bar":     hclspec.NewAttr("bar", "number", false),
		"res_key": hclspec.NewAttr("res_key", "string", false),
	})

	return configSpec, nil
}

// SetConfig decodes the configuration and stores it
func (m *mockPlugin) SetConfig(c *base.Config) error {
	var config mockPluginConfig
	if len(c.PluginConfig) != 0 {
		if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
			return err
		}
	}

	m.config = &config
	m.nomadConfig = c.AgentConfig
	m.negotiatedApiVersion = c.ApiVersion
	return nil
}

func (m *mockPlugin) Fingerprint(ctx context.Context) (<-chan *device.FingerprintResponse, error) {
	return make(chan *device.FingerprintResponse), nil
}

func (m *mockPlugin) Reserve(deviceIDs []string) (*device.ContainerReservation, error) {
	if m.config == nil || m.config.ResKey == "" {
		return nil, nil
	}

	return &device.ContainerReservation{
		Envs: map[string]string{m.config.ResKey: "config-set"},
	}, nil
}

func (m *mockPlugin) Stats(ctx context.Context, interval time.Duration) (<-chan *device.StatsResponse, error) {
	return make(chan *device.StatsResponse), nil
}
