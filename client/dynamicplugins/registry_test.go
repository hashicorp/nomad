package dynamicplugins

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPluginEventBroadcaster_SendsMessagesToAllClients(t *testing.T) {
	t.Parallel()
	b := newPluginEventBroadcaster()
	defer close(b.stopCh)
	var rcv1, rcv2 bool

	ch1 := b.subscribe()
	ch2 := b.subscribe()

	listenFunc := func(ch chan *PluginUpdateEvent, updateBool *bool) {
		select {
		case <-ch:
			*updateBool = true
		}
	}

	go listenFunc(ch1, &rcv1)
	go listenFunc(ch2, &rcv2)

	b.broadcast(&PluginUpdateEvent{})

	require.Eventually(t, func() bool {
		return rcv1 == true && rcv2 == true
	}, 1*time.Second, 200*time.Millisecond)
}

func TestPluginEventBroadcaster_UnsubscribeWorks(t *testing.T) {
	t.Parallel()

	b := newPluginEventBroadcaster()
	defer close(b.stopCh)
	var rcv1 bool

	ch1 := b.subscribe()

	listenFunc := func(ch chan *PluginUpdateEvent, updateBool *bool) {
		select {
		case e := <-ch:
			if e == nil {
				*updateBool = true
			}
		}
	}

	go listenFunc(ch1, &rcv1)

	b.unsubscribe(ch1)

	b.broadcast(&PluginUpdateEvent{})

	require.Eventually(t, func() bool {
		return rcv1 == true
	}, 1*time.Second, 200*time.Millisecond)
}

func TestDynamicRegistry_RegisterPlugin_SendsUpdateEvents(t *testing.T) {
	t.Parallel()
	r := NewRegistry(nil, nil)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	ch := r.PluginsUpdatedCh(ctx, "csi")
	receivedRegistrationEvent := false

	listenFunc := func(ch <-chan *PluginUpdateEvent, updateBool *bool) {
		select {
		case e := <-ch:
			if e == nil {
				return
			}

			if e.EventType == EventTypeRegistered {
				*updateBool = true
			}
		}
	}

	go listenFunc(ch, &receivedRegistrationEvent)

	err := r.RegisterPlugin(&PluginInfo{
		Type:           "csi",
		Name:           "my-plugin",
		ConnectionInfo: &PluginConnectionInfo{},
	})

	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return receivedRegistrationEvent == true
	}, 1*time.Second, 200*time.Millisecond)
}

func TestDynamicRegistry_DeregisterPlugin_SendsUpdateEvents(t *testing.T) {
	t.Parallel()
	r := NewRegistry(nil, nil)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	ch := r.PluginsUpdatedCh(ctx, "csi")
	receivedDeregistrationEvent := false

	listenFunc := func(ch <-chan *PluginUpdateEvent, updateBool *bool) {
		for {
			select {
			case e := <-ch:
				if e == nil {
					return
				}

				if e.EventType == EventTypeDeregistered {
					*updateBool = true
				}
			}
		}
	}

	go listenFunc(ch, &receivedDeregistrationEvent)

	err := r.RegisterPlugin(&PluginInfo{
		Type:           "csi",
		Name:           "my-plugin",
		AllocID:        "alloc-0",
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	err = r.DeregisterPlugin("csi", "my-plugin", "alloc-0")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return receivedDeregistrationEvent == true
	}, 1*time.Second, 200*time.Millisecond)
}

func TestDynamicRegistry_DispensePlugin_Works(t *testing.T) {
	dispenseFn := func(i *PluginInfo) (interface{}, error) {
		return struct{}{}, nil
	}

	registry := NewRegistry(nil, map[string]PluginDispenser{"csi": dispenseFn})

	err := registry.RegisterPlugin(&PluginInfo{
		Type:           "csi",
		Name:           "my-plugin",
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	result, err := registry.DispensePlugin("unknown-type", "unknown-name")
	require.Nil(t, result)
	require.EqualError(t, err, "no plugin dispenser found for type: unknown-type")

	result, err = registry.DispensePlugin("csi", "unknown-name")
	require.Nil(t, result)
	require.EqualError(t, err, "plugin unknown-name for type csi not found")

	result, err = registry.DispensePlugin("csi", "my-plugin")
	require.NotNil(t, result)
	require.NoError(t, err)
}

func TestDynamicRegistry_IsolatePluginTypes(t *testing.T) {
	t.Parallel()
	r := NewRegistry(nil, nil)

	err := r.RegisterPlugin(&PluginInfo{
		Type:           PluginTypeCSIController,
		Name:           "my-plugin",
		AllocID:        "alloc-0",
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	err = r.RegisterPlugin(&PluginInfo{
		Type:           PluginTypeCSINode,
		Name:           "my-plugin",
		AllocID:        "alloc-1",
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	err = r.DeregisterPlugin(PluginTypeCSIController, "my-plugin", "alloc-0")
	require.NoError(t, err)
	require.Equal(t, 1, len(r.ListPlugins(PluginTypeCSINode)))
	require.Equal(t, 0, len(r.ListPlugins(PluginTypeCSIController)))
}

func TestDynamicRegistry_StateStore(t *testing.T) {
	t.Parallel()
	dispenseFn := func(i *PluginInfo) (interface{}, error) {
		return i, nil
	}

	memdb := &MemDB{}
	oldR := NewRegistry(memdb, map[string]PluginDispenser{"csi": dispenseFn})

	err := oldR.RegisterPlugin(&PluginInfo{
		Type:           "csi",
		Name:           "my-plugin",
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)
	result, err := oldR.DispensePlugin("csi", "my-plugin")
	require.NotNil(t, result)
	require.NoError(t, err)

	// recreate the registry from the state store and query again
	newR := NewRegistry(memdb, map[string]PluginDispenser{"csi": dispenseFn})
	result, err = newR.DispensePlugin("csi", "my-plugin")
	require.NotNil(t, result)
	require.NoError(t, err)
}

func TestDynamicRegistry_ConcurrentAllocs(t *testing.T) {

	t.Parallel()
	dispenseFn := func(i *PluginInfo) (interface{}, error) {
		return i, nil
	}

	newPlugin := func(idx int) *PluginInfo {
		id := fmt.Sprintf("alloc-%d", idx)
		return &PluginInfo{
			Name:    "my-plugin",
			Type:    PluginTypeCSINode,
			Version: fmt.Sprintf("v%d", idx),
			ConnectionInfo: &PluginConnectionInfo{
				SocketPath: "/var/data/alloc/" + id + "/csi.sock"},
			AllocID: id,
		}
	}

	dispensePlugin := func(t *testing.T, reg Registry) *PluginInfo {
		result, err := reg.DispensePlugin(PluginTypeCSINode, "my-plugin")
		require.NotNil(t, result)
		require.NoError(t, err)
		plugin := result.(*PluginInfo)
		return plugin
	}

	t.Run("restore races on client restart", func(t *testing.T) {
		plugin0 := newPlugin(0)
		plugin1 := newPlugin(1)

		memdb := &MemDB{}
		oldR := NewRegistry(memdb, map[string]PluginDispenser{PluginTypeCSINode: dispenseFn})

		// add a plugin and a new alloc running the same plugin
		// (without stopping the old one)
		require.NoError(t, oldR.RegisterPlugin(plugin0))
		require.NoError(t, oldR.RegisterPlugin(plugin1))
		plugin := dispensePlugin(t, oldR)
		require.Equal(t, "alloc-1", plugin.AllocID)

		// client restarts and we load state from disk.
		// most recently inserted plugin is current

		newR := NewRegistry(memdb, map[string]PluginDispenser{PluginTypeCSINode: dispenseFn})
		plugin = dispensePlugin(t, oldR)
		require.Equal(t, "/var/data/alloc/alloc-1/csi.sock", plugin.ConnectionInfo.SocketPath)
		require.Equal(t, "alloc-1", plugin.AllocID)

		// RestoreTask fires for all allocations, which runs the
		// plugin_supervisor_hook. But there's a race and the allocations
		// in this scenario are Restored in the opposite order they were
		// created

		require.NoError(t, newR.RegisterPlugin(plugin0))
		plugin = dispensePlugin(t, newR)
		require.Equal(t, "/var/data/alloc/alloc-1/csi.sock", plugin.ConnectionInfo.SocketPath)
		require.Equal(t, "alloc-1", plugin.AllocID)
	})

	t.Run("replacement races on host restart", func(t *testing.T) {
		plugin0 := newPlugin(0)
		plugin1 := newPlugin(1)
		plugin2 := newPlugin(2)

		memdb := &MemDB{}
		oldR := NewRegistry(memdb, map[string]PluginDispenser{PluginTypeCSINode: dispenseFn})

		// add a plugin and a new alloc running the same plugin
		// (without stopping the old one)
		require.NoError(t, oldR.RegisterPlugin(plugin0))
		require.NoError(t, oldR.RegisterPlugin(plugin1))
		plugin := dispensePlugin(t, oldR)
		require.Equal(t, "alloc-1", plugin.AllocID)

		// client restarts and we load state from disk.
		// most recently inserted plugin is current

		newR := NewRegistry(memdb, map[string]PluginDispenser{PluginTypeCSINode: dispenseFn})
		plugin = dispensePlugin(t, oldR)
		require.Equal(t, "/var/data/alloc/alloc-1/csi.sock", plugin.ConnectionInfo.SocketPath)
		require.Equal(t, "alloc-1", plugin.AllocID)

		// RestoreTask fires for all allocations but none of them are
		// running because we restarted the whole host. Server gives
		// us a replacement alloc

		require.NoError(t, newR.RegisterPlugin(plugin2))
		plugin = dispensePlugin(t, newR)
		require.Equal(t, "/var/data/alloc/alloc-2/csi.sock", plugin.ConnectionInfo.SocketPath)
		require.Equal(t, "alloc-2", plugin.AllocID)
	})

	t.Run("interleaved register and deregister", func(t *testing.T) {
		plugin0 := newPlugin(0)
		plugin1 := newPlugin(1)

		memdb := &MemDB{}
		reg := NewRegistry(memdb, map[string]PluginDispenser{PluginTypeCSINode: dispenseFn})

		require.NoError(t, reg.RegisterPlugin(plugin0))

		// replacement is registered before old plugin deregisters
		require.NoError(t, reg.RegisterPlugin(plugin1))
		plugin := dispensePlugin(t, reg)
		require.Equal(t, "alloc-1", plugin.AllocID)

		reg.DeregisterPlugin(PluginTypeCSINode, "my-plugin", "alloc-0")
		plugin = dispensePlugin(t, reg)
		require.Equal(t, "alloc-1", plugin.AllocID)
	})

}

// MemDB implements a StateDB that stores data in memory and should only be
// used for testing. All methods are safe for concurrent use. This is a
// partial implementation of the MemDB in the client/state package, copied
// here to avoid circular dependencies.
type MemDB struct {
	dynamicManagerPs *RegistryState
	mu               sync.RWMutex
}

func (m *MemDB) GetDynamicPluginRegistryState() (*RegistryState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dynamicManagerPs, nil
}

func (m *MemDB) PutDynamicPluginRegistryState(ps *RegistryState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dynamicManagerPs = ps
	return nil
}
