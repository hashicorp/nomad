package dynamicplugins

import (
	"context"
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
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	err = r.DeregisterPlugin("csi", "my-plugin")
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
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	err = r.RegisterPlugin(&PluginInfo{
		Type:           PluginTypeCSINode,
		Name:           "my-plugin",
		ConnectionInfo: &PluginConnectionInfo{},
	})
	require.NoError(t, err)

	err = r.DeregisterPlugin(PluginTypeCSIController, "my-plugin")
	require.NoError(t, err)
	require.Equal(t, len(r.ListPlugins(PluginTypeCSINode)), 1)
	require.Equal(t, len(r.ListPlugins(PluginTypeCSIController)), 0)
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
