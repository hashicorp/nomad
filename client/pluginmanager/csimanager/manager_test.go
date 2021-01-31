package csimanager

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ pluginmanager.PluginManager = (*csiManager)(nil)

var fakePlugin = &dynamicplugins.PluginInfo{
	Name:           "my-plugin",
	Type:           "csi-controller",
	ConnectionInfo: &dynamicplugins.PluginConnectionInfo{},
}

func setupRegistry() dynamicplugins.Registry {
	return dynamicplugins.NewRegistry(
		nil,
		map[string]dynamicplugins.PluginDispenser{
			"csi-controller": func(*dynamicplugins.PluginInfo) (interface{}, error) {
				return nil, nil
			},
		})
}

func TestManager_Setup_Shutdown(t *testing.T) {
	r := setupRegistry()
	defer r.Shutdown()

	cfg := &Config{
		Logger:                testlog.HCLogger(t),
		DynamicRegistry:       r,
		UpdateNodeCSIInfoFunc: func(string, *structs.CSIInfo) {},
	}
	pm := New(cfg).(*csiManager)
	pm.Run()
	pm.Shutdown()
}

func TestManager_RegisterPlugin(t *testing.T) {
	registry := setupRegistry()
	defer registry.Shutdown()

	require.NotNil(t, registry)

	cfg := &Config{
		Logger:                testlog.HCLogger(t),
		DynamicRegistry:       registry,
		UpdateNodeCSIInfoFunc: func(string, *structs.CSIInfo) {},
	}
	pm := New(cfg).(*csiManager)
	defer pm.Shutdown()

	require.NotNil(t, pm.registry)

	err := registry.RegisterPlugin(fakePlugin)
	require.Nil(t, err)

	pm.Run()

	require.Eventually(t, func() bool {
		pmap, ok := pm.instances[fakePlugin.Type]
		if !ok {
			return false
		}

		_, ok = pmap[fakePlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)
}

func TestManager_DeregisterPlugin(t *testing.T) {
	registry := setupRegistry()
	defer registry.Shutdown()

	require.NotNil(t, registry)

	cfg := &Config{
		Logger:                testlog.HCLogger(t),
		DynamicRegistry:       registry,
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
		_, ok := pm.instances[fakePlugin.Type][fakePlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	err = registry.DeregisterPlugin(fakePlugin.Type, fakePlugin.Name)
	require.Nil(t, err)

	require.Eventually(t, func() bool {
		_, ok := pm.instances[fakePlugin.Type][fakePlugin.Name]
		return !ok
	}, 5*time.Second, 10*time.Millisecond)
}

// TestManager_MultiplePlugins ensures that multiple plugins with the same
// name but different types (as found with monolith plugins) don't interfere
// with each other.
func TestManager_MultiplePlugins(t *testing.T) {
	registry := setupRegistry()
	defer registry.Shutdown()

	require.NotNil(t, registry)

	cfg := &Config{
		Logger:                testlog.HCLogger(t),
		DynamicRegistry:       registry,
		UpdateNodeCSIInfoFunc: func(string, *structs.CSIInfo) {},
		PluginResyncPeriod:    500 * time.Millisecond,
	}
	pm := New(cfg).(*csiManager)
	defer pm.Shutdown()

	require.NotNil(t, pm.registry)

	err := registry.RegisterPlugin(fakePlugin)
	require.Nil(t, err)

	fakeNodePlugin := *fakePlugin
	fakeNodePlugin.Type = "csi-node"
	err = registry.RegisterPlugin(&fakeNodePlugin)
	require.Nil(t, err)

	pm.Run()

	require.Eventually(t, func() bool {
		_, ok := pm.instances[fakePlugin.Type][fakePlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := pm.instances[fakeNodePlugin.Type][fakeNodePlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	err = registry.DeregisterPlugin(fakePlugin.Type, fakePlugin.Name)
	require.Nil(t, err)

	require.Eventually(t, func() bool {
		_, ok := pm.instances[fakePlugin.Type][fakePlugin.Name]
		return !ok
	}, 5*time.Second, 10*time.Millisecond)
}
