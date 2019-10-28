package pluginregistry

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Registry is an interface that allows for the dynamic registration of plugins
// that are running as Nomad Tasks.
type Registry interface {
	RegisterPlugin(info *PluginInfo) error
	DeregisterPlugin(ptype, name string) error

	ListPlugins(ptype string) []*PluginInfo
	DispensePlugin(ptype, name string) (interface{}, error)

	PluginsUpdatedCh(ctx context.Context, ptype string) <-chan *PluginUpdateEvent
}

type PluginDispenser func(info *PluginInfo) (interface{}, error)

// NewPluginRegistry takes a map of `plugintype` to PluginDispenser functions
// that should be used to vend clients for plugins to be used.
func NewPluginRegistry(dispensers map[string]PluginDispenser) Registry {
	return &dynamicRegistry{
		plugins:      make(map[string]map[string]*PluginInfo),
		broadcasters: make(map[string]*pluginEventBroadcaster),
		dispensers:   dispensers,
	}
}

type PluginInfo struct {
	Name    string
	Type    string
	Version string

	// ConnectionInfo should only be used externally during `RegisterPlugin` and
	// may not be exposed in the future.
	ConnectionInfo *PluginConnectionInfo
}

// PluginConnectionInfo is the data required to connect to the plugin.
// note: We currently only support Unix Domain Sockets, but this may be expanded
//       to support other connection modes in the future.
type PluginConnectionInfo struct {
	// SocketPath is the path to the plugins api socket.
	SocketPath string
}

type EventType string

const (
	EventTypeRegistered   EventType = "registered"
	EventTypeDeregistered EventType = "deregistered"
)

// PluginUpdateEvent is a struct that is sent over a PluginsUpdatedCh when
type PluginUpdateEvent struct {
	EventType EventType
	Info      *PluginInfo
}

type dynamicRegistry struct {
	plugins     map[string]map[string]*PluginInfo
	pluginsLock sync.RWMutex

	broadcasters     map[string]*pluginEventBroadcaster
	broadcastersLock sync.Mutex

	dispensers map[string]PluginDispenser
}

func (d *dynamicRegistry) RegisterPlugin(info *PluginInfo) error {
	if info.Type == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return errors.New("Plugin.Type must not be empty")
	}

	if info.ConnectionInfo == nil {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return errors.New("Plugin.ConnectionInfo must not be nil")
	}

	if info.Name == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return errors.New("Plugin.Name must not be empty")
	}

	d.pluginsLock.Lock()
	defer d.pluginsLock.Unlock()

	pmap, ok := d.plugins[info.Type]
	if !ok {
		pmap = make(map[string]*PluginInfo, 1)
		d.plugins[info.Type] = pmap
	}

	pmap[info.Name] = info

	broadcaster := d.broadcasterForPluginType(info.Type)
	event := &PluginUpdateEvent{
		EventType: EventTypeRegistered,
		Info:      info,
	}
	broadcaster.broadcast(event)

	return nil
}

func (d *dynamicRegistry) broadcasterForPluginType(ptype string) *pluginEventBroadcaster {
	d.broadcastersLock.Lock()
	defer d.broadcastersLock.Unlock()

	broadcaster, ok := d.broadcasters[ptype]
	if !ok {
		broadcaster = newPluginEventBroadcaster()
		d.broadcasters[ptype] = broadcaster
	}

	return broadcaster
}

func (d *dynamicRegistry) DeregisterPlugin(ptype, name string) error {
	d.pluginsLock.Lock()
	defer d.pluginsLock.Unlock()

	if ptype == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return errors.New("must specify plugin type to deregister")
	}
	if name == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return errors.New("must specify plugin name to deregister")
	}

	pmap, ok := d.plugins[ptype]
	if !ok {
		// If this occurs there's a bug in the registration handler.
		return fmt.Errorf("no plugins registered for type: %s", ptype)
	}

	info, ok := pmap[name]
	if !ok {
		// plugin already deregistered, don't send events or try re-deleting.
		return nil
	}
	delete(pmap, name)

	broadcaster := d.broadcasterForPluginType(ptype)
	event := &PluginUpdateEvent{
		EventType: EventTypeDeregistered,
		Info:      info,
	}
	broadcaster.broadcast(event)

	return nil
}

func (d *dynamicRegistry) ListPlugins(ptype string) []*PluginInfo {
	return nil
}

func (d *dynamicRegistry) DispensePlugin(ptype string, name string) (interface{}, error) {
	d.pluginsLock.Lock()
	defer d.pluginsLock.Unlock()

	if ptype == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return nil, errors.New("must specify plugin type to deregister")
	}
	if name == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return nil, errors.New("must specify plugin name to deregister")
	}

	dispenseFunc, ok := d.dispensers[ptype]
	if !ok {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return nil, fmt.Errorf("no plugin dispenser found for type: %s", ptype)
	}

	pmap, ok := d.plugins[ptype]
	if !ok {
		return nil, fmt.Errorf("no plugins registered for type: %s", ptype)
	}

	info, ok := pmap[name]
	if !ok {
		return nil, fmt.Errorf("plugin %s for type %s not found", name, ptype)
	}

	return dispenseFunc(info)
}

func (d *dynamicRegistry) PluginsUpdatedCh(ctx context.Context, ptype string) <-chan *PluginUpdateEvent {
	b := d.broadcasterForPluginType(ptype)
	ch := b.subscribe()
	go func() {
		select {
		case <-ctx.Done():
			b.unsubscribe(ch)
		}
	}()

	return ch
}

type pluginEventBroadcaster struct {
	stopCh    chan struct{}
	publishCh chan *PluginUpdateEvent

	subscriptions     map[chan *PluginUpdateEvent]struct{}
	subscriptionsLock sync.RWMutex
}

func newPluginEventBroadcaster() *pluginEventBroadcaster {
	b := &pluginEventBroadcaster{
		stopCh:        make(chan struct{}),
		publishCh:     make(chan *PluginUpdateEvent, 1),
		subscriptions: make(map[chan *PluginUpdateEvent]struct{}),
	}
	go b.run()
	return b
}

func (p *pluginEventBroadcaster) run() {
	for {
		select {
		case <-p.stopCh:
			p.subscriptionsLock.Lock()
			for sub := range p.subscriptions {
				delete(p.subscriptions, sub)
				close(sub)
			}
			p.subscriptionsLock.Unlock()
			return
		case msg := <-p.publishCh:
			p.subscriptionsLock.RLock()
			for msgCh := range p.subscriptions {
				// We block on sends as we do not want any subscribers to miss messages.
				msgCh <- msg
			}
			p.subscriptionsLock.RUnlock()
		}
	}
}

func (p *pluginEventBroadcaster) broadcast(e *PluginUpdateEvent) {
	p.publishCh <- e
}

func (p *pluginEventBroadcaster) subscribe() chan *PluginUpdateEvent {
	p.subscriptionsLock.Lock()
	defer p.subscriptionsLock.Unlock()

	ch := make(chan *PluginUpdateEvent, 1)
	p.subscriptions[ch] = struct{}{}
	return ch
}

func (p *pluginEventBroadcaster) unsubscribe(ch chan *PluginUpdateEvent) {
	p.subscriptionsLock.Lock()
	defer p.subscriptionsLock.Unlock()

	delete(p.subscriptions, ch)
	close(ch)
}
