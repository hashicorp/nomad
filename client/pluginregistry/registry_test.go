package pluginregistry

import (
	"context"
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
	r := NewPluginRegistry(nil)

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
	r := NewPluginRegistry(nil)

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

	registry := NewPluginRegistry(map[string]PluginDispenser{"csi": dispenseFn})

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
