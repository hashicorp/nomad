package csimanager

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ pluginmanager.PluginManager = (*csiManager)(nil)

func fakePlugin(idx int, pluginType string) *dynamicplugins.PluginInfo {
	id := fmt.Sprintf("alloc-%d", idx)
	return &dynamicplugins.PluginInfo{
		Name:    "my-plugin",
		Type:    pluginType,
		Version: fmt.Sprintf("v%d", idx),
		ConnectionInfo: &dynamicplugins.PluginConnectionInfo{
			SocketPath: "/var/data/alloc/" + id + "/csi.sock"},
		AllocID: id,
	}
}

func testManager(t *testing.T, registry dynamicplugins.Registry, resyncPeriod time.Duration) *csiManager {
	return New(&Config{
		Logger:                testlog.HCLogger(t),
		DynamicRegistry:       registry,
		UpdateNodeCSIInfoFunc: func(string, *structs.CSIInfo) {},
		PluginResyncPeriod:    resyncPeriod,
	}).(*csiManager)
}

func setupRegistry(reg *MemDB) dynamicplugins.Registry {
	return dynamicplugins.NewRegistry(
		reg,
		map[string]dynamicplugins.PluginDispenser{
			"csi-controller": func(i *dynamicplugins.PluginInfo) (interface{}, error) {
				return i, nil
			},
			"csi-node": func(i *dynamicplugins.PluginInfo) (interface{}, error) {
				return i, nil
			},
		})
}

func TestManager_RegisterPlugin(t *testing.T) {
	registry := setupRegistry(nil)
	defer registry.Shutdown()
	pm := testManager(t, registry, time.Hour)
	defer pm.Shutdown()

	plugin := fakePlugin(0, dynamicplugins.PluginTypeCSIController)
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)

	pm.Run()

	require.Eventually(t, func() bool {
		pmap, ok := pm.instances[plugin.Type]
		if !ok {
			return false
		}

		_, ok = pmap[plugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)
}

func TestManager_DeregisterPlugin(t *testing.T) {
	registry := setupRegistry(nil)
	defer registry.Shutdown()
	pm := testManager(t, registry, 500*time.Millisecond)
	defer pm.Shutdown()

	plugin := fakePlugin(0, dynamicplugins.PluginTypeCSIController)
	err := registry.RegisterPlugin(plugin)
	require.NoError(t, err)

	pm.Run()

	require.Eventually(t, func() bool {
		_, ok := pm.instances[plugin.Type][plugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	err = registry.DeregisterPlugin(plugin.Type, plugin.Name, "alloc-0")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, ok := pm.instances[plugin.Type][plugin.Name]
		return !ok
	}, 5*time.Second, 10*time.Millisecond)
}

// TestManager_MultiplePlugins ensures that multiple plugins with the same
// name but different types (as found with monolith plugins) don't interfere
// with each other.
func TestManager_MultiplePlugins(t *testing.T) {
	registry := setupRegistry(nil)
	defer registry.Shutdown()

	pm := testManager(t, registry, 500*time.Millisecond)
	defer pm.Shutdown()

	controllerPlugin := fakePlugin(0, dynamicplugins.PluginTypeCSIController)
	err := registry.RegisterPlugin(controllerPlugin)
	require.NoError(t, err)

	nodePlugin := fakePlugin(0, dynamicplugins.PluginTypeCSINode)
	err = registry.RegisterPlugin(nodePlugin)
	require.NoError(t, err)

	pm.Run()

	require.Eventually(t, func() bool {
		_, ok := pm.instances[controllerPlugin.Type][controllerPlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := pm.instances[nodePlugin.Type][nodePlugin.Name]
		return ok
	}, 5*time.Second, 10*time.Millisecond)

	err = registry.DeregisterPlugin(controllerPlugin.Type, controllerPlugin.Name, "alloc-0")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, ok := pm.instances[controllerPlugin.Type][controllerPlugin.Name]
		return !ok
	}, 5*time.Second, 10*time.Millisecond)
}

// TestManager_ConcurrentPlugins exercises the behavior when multiple
// allocations for the same plugin interact
func TestManager_ConcurrentPlugins(t *testing.T) {

	t.Run("replacement races on host restart", func(t *testing.T) {
		plugin0 := fakePlugin(0, dynamicplugins.PluginTypeCSINode)
		plugin1 := fakePlugin(1, dynamicplugins.PluginTypeCSINode)
		plugin2 := fakePlugin(2, dynamicplugins.PluginTypeCSINode)

		db := &MemDB{}
		registry := setupRegistry(db)
		pm := testManager(t, registry, time.Hour) // no resync except from events
		pm.Run()

		require.NoError(t, registry.RegisterPlugin(plugin0))
		require.NoError(t, registry.RegisterPlugin(plugin1))
		require.Eventuallyf(t, func() bool {
			im, _ := pm.instances[plugin0.Type][plugin0.Name]
			return im.info.ConnectionInfo.SocketPath == "/var/data/alloc/alloc-1/csi.sock" &&
				im.allocID == "alloc-1"
		}, 5*time.Second, 10*time.Millisecond, "alloc-1 plugin did not become active plugin")

		pm.Shutdown()
		registry.Shutdown()

		// client restarts and we load state from disk.
		// most recently inserted plugin is current

		registry = setupRegistry(db)
		defer registry.Shutdown()
		pm = testManager(t, registry, time.Hour)
		defer pm.Shutdown()
		pm.Run()

		require.Eventuallyf(t, func() bool {
			im, _ := pm.instances[plugin0.Type][plugin0.Name]
			return im.info.ConnectionInfo.SocketPath == "/var/data/alloc/alloc-1/csi.sock" &&
				im.allocID == "alloc-1"
		}, 5*time.Second, 10*time.Millisecond, "alloc-1 plugin was not active after state reload")

		// RestoreTask fires for all allocations but none of them are
		// running because we restarted the whole host. Server gives
		// us a replacement alloc

		require.NoError(t, registry.RegisterPlugin(plugin2))
		require.Eventuallyf(t, func() bool {
			im, _ := pm.instances[plugin0.Type][plugin0.Name]
			return im.info.ConnectionInfo.SocketPath == "/var/data/alloc/alloc-2/csi.sock" &&
				im.allocID == "alloc-2"
		}, 5*time.Second, 10*time.Millisecond, "alloc-2 plugin was not active after replacement")

	})

	t.Run("interleaved register and deregister", func(t *testing.T) {
		plugin0 := fakePlugin(0, dynamicplugins.PluginTypeCSINode)
		plugin1 := fakePlugin(1, dynamicplugins.PluginTypeCSINode)

		db := &MemDB{}
		registry := setupRegistry(db)
		defer registry.Shutdown()

		pm := testManager(t, registry, time.Hour) // no resync except from events
		defer pm.Shutdown()
		pm.Run()

		require.NoError(t, registry.RegisterPlugin(plugin0))
		require.NoError(t, registry.RegisterPlugin(plugin1))

		require.Eventuallyf(t, func() bool {
			im, _ := pm.instances[plugin0.Type][plugin0.Name]
			return im.info.ConnectionInfo.SocketPath == "/var/data/alloc/alloc-1/csi.sock" &&
				im.allocID == "alloc-1"
		}, 5*time.Second, 10*time.Millisecond, "alloc-1 plugin did not become active plugin")

		registry.DeregisterPlugin(dynamicplugins.PluginTypeCSINode, "my-plugin", "alloc-0")

		require.Eventuallyf(t, func() bool {
			im, _ := pm.instances[plugin0.Type][plugin0.Name]
			return im != nil &&
				im.info.ConnectionInfo.SocketPath == "/var/data/alloc/alloc-1/csi.sock"
		}, 5*time.Second, 10*time.Millisecond, "alloc-1 plugin should still be active plugin")
	})
}

// MemDB implements a StateDB that stores data in memory and should only be
// used for testing. All methods are safe for concurrent use. This is a
// partial implementation of the MemDB in the client/state package, copied
// here to avoid circular dependencies.
type MemDB struct {
	dynamicManagerPs *dynamicplugins.RegistryState
	mu               sync.RWMutex
}

func (m *MemDB) GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error) {
	if m == nil {
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dynamicManagerPs, nil
}

func (m *MemDB) PutDynamicPluginRegistryState(ps *dynamicplugins.RegistryState) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dynamicManagerPs = ps
	return nil
}
