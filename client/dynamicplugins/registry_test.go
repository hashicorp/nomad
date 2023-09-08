// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dynamicplugins

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestPluginEventBroadcaster_SendsMessagesToAllClients(t *testing.T) {
	ci.Parallel(t)

	b := newPluginEventBroadcaster()
	defer close(b.stopCh)

	var rcv1, rcv2 bool
	ch1 := b.subscribe()
	ch2 := b.subscribe()

	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				t.Errorf("did not receive event on both subscriptions before timeout")
				return
			case <-ch1:
				rcv1 = true
			case <-ch2:
				rcv2 = true
			}
			if rcv1 && rcv2 {
				return
			}
		}
	}()

	b.broadcast(&PluginUpdateEvent{})
	wg.Wait()
}

func TestPluginEventBroadcaster_UnsubscribeWorks(t *testing.T) {
	ci.Parallel(t)

	b := newPluginEventBroadcaster()
	defer close(b.stopCh)

	ch1 := b.subscribe()

	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				t.Errorf("did not receive unsubscribe event on subscription before timeout")
				return
			case <-ch1:
				return // done!
			}
		}
	}()

	b.unsubscribe(ch1)
	b.broadcast(&PluginUpdateEvent{})
	wg.Wait()
}

func TestDynamicRegistry_RegisterPlugin_SendsUpdateEvents(t *testing.T) {
	ci.Parallel(t)

	r := NewRegistry(nil, nil)

	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ch := r.PluginsUpdatedCh(ctx, "csi")

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				t.Errorf("did not receive registration event on subscription before timeout")
				return
			case e := <-ch:
				if e != nil && e.EventType == EventTypeRegistered {
					return
				}
			}
		}
	}()

	err := r.RegisterPlugin(&PluginInfo{
		Type:           "csi",
		Name:           "my-plugin",
		ConnectionInfo: &PluginConnectionInfo{},
	})

	require.NoError(t, err)
	wg.Wait()
}

func TestDynamicRegistry_DeregisterPlugin_SendsUpdateEvents(t *testing.T) {
	ci.Parallel(t)

	r := NewRegistry(nil, nil)

	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ch := r.PluginsUpdatedCh(ctx, "csi")

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				t.Errorf("did not receive deregistration event on subscription before timeout")
				return
			case e := <-ch:
				if e != nil && e.EventType == EventTypeDeregistered {
					return
				}
			}
		}
	}()

	err := r.RegisterPlugin(&PluginInfo{
		Type:           "csi",
		Name:           "my-plugin",
		AllocID:        "alloc-0",
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	err = r.DeregisterPlugin("csi", "my-plugin", "alloc-0")
	require.NoError(t, err)
	wg.Wait()
}

func TestDynamicRegistry_DispensePlugin_Works(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

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
