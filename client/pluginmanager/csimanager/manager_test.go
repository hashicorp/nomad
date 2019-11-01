package csimanager

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginregistry"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ pluginmanager.PluginManager = (*csiManager)(nil)

var fakePlugin = &pluginregistry.PluginInfo{
	Name:           "my-plugin",
	Type:           "csi",
	ConnectionInfo: &pluginregistry.PluginConnectionInfo{},
}

func setupRegistry() pluginregistry.Registry {
	return pluginregistry.NewPluginRegistry(
		map[string]pluginregistry.PluginDispenser{
			"csi": func(*pluginregistry.PluginInfo) (interface{}, error) {
				return nil, nil
			},
		})
}

func TestCSIManager_Setup_Shutdown(t *testing.T) {
	r := setupRegistry()
	defer r.Shutdown()

	cfg := &Config{
		Logger:                testlog.HCLogger(t),
		PluginRegistry:        r,
		UpdateNodeCSIInfoFunc: func(string, *structs.CSIInfo) {},
	}
	pm := New(cfg).(*csiManager)
	pm.Run()
	pm.Shutdown()
}

func TestCSIManager_RegisterPlugin(t *testing.T) {
	registry := setupRegistry()
	defer registry.Shutdown()

	require.NotNil(t, registry)

	cfg := &Config{
		Logger:                testlog.HCLogger(t),
		PluginRegistry:        registry,
		UpdateNodeCSIInfoFunc: func(string, *structs.CSIInfo) {},
	}
	pm := New(cfg).(*csiManager)
	defer pm.Shutdown()

	require.NotNil(t, pm.registry)

	err := registry.RegisterPlugin(fakePlugin)
	require.Nil(t, err)

	pm.Run()

	require.Eventually(t, func() bool {
		_, ok := pm.instances[fakePlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)
}

func TestCSIManager_DeregisterPlugin(t *testing.T) {
	registry := setupRegistry()
	defer registry.Shutdown()

	require.NotNil(t, registry)

	cfg := &Config{
		Logger:                testlog.HCLogger(t),
		PluginRegistry:        registry,
		UpdateNodeCSIInfoFunc: func(string, *structs.CSIInfo) {},
		PluginResyncPeriod:    500 * time.Millisecond,
	}
	pm := New(cfg).(*csiManager)
	defer pm.Shutdown()

	require.NotNil(t, pm.registry)

	err := registry.RegisterPlugin(fakePlugin)
	require.Nil(t, err)

	pm.Run()

	require.Eventually(t, func() bool {
		_, ok := pm.instances[fakePlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	err = registry.DeregisterPlugin(fakePlugin.Type, fakePlugin.Name)
	require.Nil(t, err)

	require.Eventually(t, func() bool {
		_, ok := pm.instances[fakePlugin.Name]
		return !ok
	}, 5*time.Second, 10*time.Millisecond)
}
