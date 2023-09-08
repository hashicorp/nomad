// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// dynamicplugins is a package that manages dynamic plugins in Nomad.
// It exposes a registry that allows for plugins to be registered/deregistered
// and also allows subscribers to receive real time updates of these events.
package dynamicplugins

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/nomad/helper"
)

const (
	PluginTypeCSIController = "csi-controller"
	PluginTypeCSINode       = "csi-node"
)

// Registry is an interface that allows for the dynamic registration of plugins
// that are running as Nomad Tasks.
type Registry interface {
	RegisterPlugin(info *PluginInfo) error
	DeregisterPlugin(ptype, name, allocID string) error

	WaitForPlugin(ctx context.Context, ptype, pname string) (*PluginInfo, error)
	ListPlugins(ptype string) []*PluginInfo
	DispensePlugin(ptype, name string) (interface{}, error)
	PluginForAlloc(ptype, name, allocID string) (*PluginInfo, error)

	PluginsUpdatedCh(ctx context.Context, ptype string) <-chan *PluginUpdateEvent

	Shutdown()

	StubDispenserForType(ptype string, dispenser PluginDispenser)
}

// RegistryState is what we persist in the client state
// store. It contains a map of plugin types to maps of plugin name ->
// list of *PluginInfo, sorted by recency of registration
type RegistryState struct {
	Plugins map[string]map[string]*list.List
}

type PluginDispenser func(info *PluginInfo) (interface{}, error)

// NewRegistry takes a map of `plugintype` to PluginDispenser functions
// that should be used to vend clients for plugins to be used.
func NewRegistry(state StateStorage, dispensers map[string]PluginDispenser) Registry {

	registry := &dynamicRegistry{
		plugins:      make(map[string]map[string]*list.List),
		broadcasters: make(map[string]*pluginEventBroadcaster),
		dispensers:   dispensers,
		state:        state,
	}

	// populate the state and initial broadcasters if we have an
	// existing state DB to restore
	if state != nil {
		storedState, err := state.GetDynamicPluginRegistryState()
		if err == nil && storedState != nil {
			registry.plugins = storedState.Plugins
			for ptype := range registry.plugins {
				registry.broadcasterForPluginType(ptype)
			}
		}
	}

	return registry
}

// StateStorage is used to persist the dynamic plugin registry's state
// across agent restarts.
type StateStorage interface {
	// GetDynamicPluginRegistryState is used to restore the registry state
	GetDynamicPluginRegistryState() (*RegistryState, error)

	// PutDynamicPluginRegistryState is used to store the registry state
	PutDynamicPluginRegistryState(state *RegistryState) error
}

// PluginInfo is the metadata that is stored by the registry for a given plugin.
type PluginInfo struct {
	Name    string
	Type    string
	Version string

	// ConnectionInfo should only be used externally during `RegisterPlugin` and
	// may not be exposed in the future.
	ConnectionInfo *PluginConnectionInfo

	// AllocID tracks the allocation running the plugin
	AllocID string

	// Options is used for plugin registrations to pass further metadata along to
	// other subsystems
	Options map[string]string
}

// PluginConnectionInfo is the data required to connect to the plugin.
// note: We currently only support Unix Domain Sockets, but this may be expanded
//
//	to support other connection modes in the future.
type PluginConnectionInfo struct {
	// SocketPath is the path to the plugins api socket.
	SocketPath string
}

// EventType is the enum of events that will be emitted by a Registry's
// PluginsUpdatedCh.
type EventType string

const (
	// EventTypeRegistered is emitted by the Registry when a new plugin has been
	// registered.
	EventTypeRegistered EventType = "registered"
	// EventTypeDeregistered is emitted by the Registry when a plugin has been
	// removed.
	EventTypeDeregistered EventType = "deregistered"
)

// PluginUpdateEvent is a struct that is sent over a PluginsUpdatedCh when
// plugins are added or removed from the registry.
type PluginUpdateEvent struct {
	EventType EventType
	Info      *PluginInfo
}

type dynamicRegistry struct {
	plugins     map[string]map[string]*list.List
	pluginsLock sync.RWMutex

	broadcasters     map[string]*pluginEventBroadcaster
	broadcastersLock sync.Mutex

	dispensers     map[string]PluginDispenser
	stubDispensers map[string]PluginDispenser

	state StateStorage
}

// StubDispenserForType allows test functions to provide alternative plugin
// dispensers to simplify writing tests for higher level Nomad features.
// This function should not be called from production code.
func (d *dynamicRegistry) StubDispenserForType(ptype string, dispenser PluginDispenser) {
	// delete from stubs
	if dispenser == nil && d.stubDispensers != nil {
		delete(d.stubDispensers, ptype)
		if len(d.stubDispensers) == 0 {
			d.stubDispensers = nil
		}

		return
	}

	// setup stubs
	if d.stubDispensers == nil {
		d.stubDispensers = make(map[string]PluginDispenser, 1)
	}

	d.stubDispensers[ptype] = dispenser
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
		pmap = make(map[string]*list.List)
		d.plugins[info.Type] = pmap
	}
	infos, ok := pmap[info.Name]
	if !ok {
		infos = list.New()
		pmap[info.Name] = infos
	}

	// TODO(tgross): https://github.com/hashicorp/nomad/issues/11786
	// If we're already registered, we should update the definition
	// and send a broadcast of any update so the instanceManager can
	// be restarted if there's been a change
	var alreadyRegistered bool
	for e := infos.Front(); e != nil; e = e.Next() {
		if e.Value.(*PluginInfo).AllocID == info.AllocID {
			alreadyRegistered = true
			break
		}
	}
	if !alreadyRegistered {
		infos.PushFront(info)
		broadcaster := d.broadcasterForPluginType(info.Type)
		event := &PluginUpdateEvent{
			EventType: EventTypeRegistered,
			Info:      info,
		}
		broadcaster.broadcast(event)
	}

	return d.sync()
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

func (d *dynamicRegistry) DeregisterPlugin(ptype, name, allocID string) error {
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
	if allocID == "" {
		return errors.New("must specify plugin allocation ID to deregister")
	}

	pmap, ok := d.plugins[ptype]
	if !ok {
		// If this occurs there's a bug in the registration handler.
		return fmt.Errorf("no plugins registered for type: %s", ptype)
	}

	infos, ok := pmap[name]
	if !ok {
		// plugin already deregistered, don't send events or try re-deleting.
		return nil
	}

	var info *PluginInfo
	for e := infos.Front(); e != nil; e = e.Next() {
		info = e.Value.(*PluginInfo)
		if info.AllocID == allocID {
			infos.Remove(e)
			break
		}
	}

	if info != nil {
		broadcaster := d.broadcasterForPluginType(ptype)
		event := &PluginUpdateEvent{
			EventType: EventTypeDeregistered,
			Info:      info,
		}
		broadcaster.broadcast(event)
	}

	return d.sync()
}

func (d *dynamicRegistry) ListPlugins(ptype string) []*PluginInfo {
	d.pluginsLock.RLock()
	defer d.pluginsLock.RUnlock()

	pmap, ok := d.plugins[ptype]
	if !ok {
		return nil
	}

	plugins := make([]*PluginInfo, 0, len(pmap))

	for _, info := range pmap {
		if info.Front() != nil {
			plugins = append(plugins, info.Front().Value.(*PluginInfo))
		}
	}

	return plugins
}

// WaitForPlugin repeatedly checks until a plugin with a given type and name
// becomes available or its context is canceled or times out.
// Callers should pass in a context with a sensible timeout
// for the plugin they're expecting to find.
func (d *dynamicRegistry) WaitForPlugin(ctx context.Context, ptype, name string) (*PluginInfo, error) {
	// this is our actual goal, which may be run repeatedly
	findPlugin := func() *PluginInfo {
		for _, p := range d.ListPlugins(ptype) {
			if p.Name == name {
				return p
			}
		}
		return nil
	}

	// try immediately first, before any timers get involved
	if p := findPlugin(); p != nil {
		return p, nil
	}

	// next, loop until found or context is done

	// these numbers are almost arbitrary...
	delay := 200     // milliseconds between checks, will backoff
	maxDelay := 5000 // up to 5 seconds between each check

	// put a long upper bound on total time,
	// just in case callers don't follow directions.
	ctx, cancel := context.WithTimeout(ctx, 24*time.Hour)
	defer cancel()

	timer, stop := helper.NewSafeTimer(time.Duration(delay) * time.Millisecond)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			// an externally-defined timeout wins the day
			return nil, ctx.Err()
		case <-timer.C:
			// continue after our internal delay
		}

		if p := findPlugin(); p != nil {
			return p, nil
		}

		if delay < maxDelay {
			delay += delay
		}
		if delay > maxDelay {
			delay = maxDelay
		}
		timer.Reset(time.Duration(delay) * time.Millisecond)
	}
}

func (d *dynamicRegistry) DispensePlugin(ptype string, name string) (interface{}, error) {
	d.pluginsLock.Lock()
	defer d.pluginsLock.Unlock()

	if ptype == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return nil, errors.New("must specify plugin type to dispense")
	}
	if name == "" {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return nil, errors.New("must specify plugin name to dispense")
	}

	dispenseFunc, ok := d.dispensers[ptype]
	if !ok {
		// This error shouldn't make it to a production cluster and is to aid
		// developers during the development of new plugin types.
		return nil, fmt.Errorf("no plugin dispenser found for type: %s", ptype)
	}

	// After initially loading the dispenser (to avoid masking missing setup in
	// client/client.go), we then check to see if we have any stub dispensers for
	// this plugin type. If we do, then replace the dispenser fn with the stub.
	if d.stubDispensers != nil {
		if stub, ok := d.stubDispensers[ptype]; ok {
			dispenseFunc = stub
		}
	}

	pmap, ok := d.plugins[ptype]
	if !ok {
		return nil, fmt.Errorf("no plugins registered for type: %s", ptype)
	}

	info, ok := pmap[name]
	if !ok || info.Front() == nil {
		return nil, fmt.Errorf("plugin %s for type %s not found", name, ptype)
	}

	return dispenseFunc(info.Front().Value.(*PluginInfo))
}

func (d *dynamicRegistry) PluginForAlloc(ptype, name, allocID string) (*PluginInfo, error) {
	d.pluginsLock.Lock()
	defer d.pluginsLock.Unlock()

	pmap, ok := d.plugins[ptype]
	if !ok {
		return nil, fmt.Errorf("no plugins registered for type: %s", ptype)
	}

	infos, ok := pmap[name]
	if ok {
		for e := infos.Front(); e != nil; e = e.Next() {
			plugin := e.Value.(*PluginInfo)
			if plugin.AllocID == allocID {
				return plugin, nil
			}
		}
	}
	return nil, fmt.Errorf("no plugin for that allocation")
}

// PluginsUpdatedCh returns a channel over which plugin events for the requested
// plugin type will be emitted. These events are strongly ordered and will never
// be dropped.
//
// The receiving channel _must not_ be closed before the provided context is
// cancelled.
func (d *dynamicRegistry) PluginsUpdatedCh(ctx context.Context, ptype string) <-chan *PluginUpdateEvent {
	b := d.broadcasterForPluginType(ptype)
	ch := b.subscribe()
	go func() {
		select {
		case <-b.shutdownCh:
			return
		case <-ctx.Done():
			b.unsubscribe(ch)
		}
	}()

	return ch
}

func (d *dynamicRegistry) sync() error {
	if d.state != nil {
		storedState := &RegistryState{Plugins: d.plugins}
		return d.state.PutDynamicPluginRegistryState(storedState)
	}
	return nil
}

func (d *dynamicRegistry) Shutdown() {
	for _, b := range d.broadcasters {
		b.shutdown()
	}
}

type pluginEventBroadcaster struct {
	stopCh     chan struct{}
	shutdownCh chan struct{}
	publishCh  chan *PluginUpdateEvent

	subscriptions     map[chan *PluginUpdateEvent]struct{}
	subscriptionsLock sync.RWMutex
}

func newPluginEventBroadcaster() *pluginEventBroadcaster {
	b := &pluginEventBroadcaster{
		stopCh:        make(chan struct{}),
		shutdownCh:    make(chan struct{}),
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
			close(p.shutdownCh)
			return
		case msg := <-p.publishCh:
			p.subscriptionsLock.RLock()
			for msgCh := range p.subscriptions {
				msgCh <- msg
			}
			p.subscriptionsLock.RUnlock()
		}
	}
}

func (p *pluginEventBroadcaster) shutdown() {
	close(p.stopCh)

	// Wait for loop to exit before closing subscriptions
	<-p.shutdownCh

	p.subscriptionsLock.Lock()
	for sub := range p.subscriptions {
		delete(p.subscriptions, sub)
		close(sub)
	}
	p.subscriptionsLock.Unlock()
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

	_, ok := p.subscriptions[ch]
	if ok {
		delete(p.subscriptions, ch)
		close(ch)
	}
}
