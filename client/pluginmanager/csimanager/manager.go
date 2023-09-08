// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package csimanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

// defaultPluginResyncPeriod is the time interval used to do a full resync
// against the dynamicplugins, to account for missed updates.
const defaultPluginResyncPeriod = 30 * time.Second

// UpdateNodeCSIInfoFunc is the callback used to update the node from
// fingerprinting
type UpdateNodeCSIInfoFunc func(string, *structs.CSIInfo)
type TriggerNodeEvent func(*structs.NodeEvent)

type Config struct {
	Logger                hclog.Logger
	DynamicRegistry       dynamicplugins.Registry
	UpdateNodeCSIInfoFunc UpdateNodeCSIInfoFunc
	PluginResyncPeriod    time.Duration
	TriggerNodeEvent      TriggerNodeEvent
}

// New returns a new PluginManager that will handle managing CSI plugins from
// the dynamicRegistry from the provided Config.
func New(config *Config) Manager {
	// Use a dedicated internal context for managing plugin shutdown.
	ctx, cancelFn := context.WithCancel(context.Background())
	if config.PluginResyncPeriod == 0 {
		config.PluginResyncPeriod = defaultPluginResyncPeriod
	}

	return &csiManager{
		logger:    config.Logger.Named("csi_manager"),
		eventer:   config.TriggerNodeEvent,
		registry:  config.DynamicRegistry,
		instances: make(map[string]map[string]*instanceManager),

		updateNodeCSIInfoFunc: config.UpdateNodeCSIInfoFunc,
		pluginResyncPeriod:    config.PluginResyncPeriod,

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

type csiManager struct {
	// instances should only be accessed after locking with instancesLock.
	// It is a map of PluginType : [PluginName : *instanceManager]
	instances     map[string]map[string]*instanceManager
	instancesLock sync.RWMutex

	registry           dynamicplugins.Registry
	logger             hclog.Logger
	eventer            TriggerNodeEvent
	pluginResyncPeriod time.Duration

	updateNodeCSIInfoFunc UpdateNodeCSIInfoFunc

	shutdownCtx         context.Context
	shutdownCtxCancelFn context.CancelFunc
	shutdownCh          chan struct{}
}

func (c *csiManager) PluginManager() pluginmanager.PluginManager {
	return c
}

// WaitForPlugin waits for a specific plugin to be registered and available,
// unless the context is canceled, or it takes longer than a minute.
func (c *csiManager) WaitForPlugin(ctx context.Context, pType, pID string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	p, err := c.registry.WaitForPlugin(ctx, pType, pID)
	if err != nil {
		return fmt.Errorf("%s plugin '%s' did not become ready: %w", pType, pID, err)
	}
	c.instancesLock.Lock()
	defer c.instancesLock.Unlock()
	c.ensureInstance(p)
	return nil
}

func (c *csiManager) MounterForPlugin(ctx context.Context, pluginID string) (VolumeMounter, error) {
	c.instancesLock.RLock()
	defer c.instancesLock.RUnlock()
	nodePlugins, hasAnyNodePlugins := c.instances["csi-node"]
	if !hasAnyNodePlugins {
		return nil, fmt.Errorf("no storage node plugins found")
	}

	mgr, hasPlugin := nodePlugins[pluginID]
	if !hasPlugin {
		return nil, fmt.Errorf("plugin %s for type csi-node not found", pluginID)
	}

	return mgr.VolumeMounter(ctx)
}

// Run starts a plugin manager and should return early
func (c *csiManager) Run() {
	go c.runLoop()
}

func (c *csiManager) runLoop() {
	timer := time.NewTimer(0) // ensure we sync immediately in first pass
	controllerUpdates := c.registry.PluginsUpdatedCh(c.shutdownCtx, "csi-controller")
	nodeUpdates := c.registry.PluginsUpdatedCh(c.shutdownCtx, "csi-node")
	for {
		select {
		case <-timer.C:
			c.resyncPluginsFromRegistry("csi-controller")
			c.resyncPluginsFromRegistry("csi-node")
			timer.Reset(c.pluginResyncPeriod)
		case event := <-controllerUpdates:
			c.handlePluginEvent(event)
		case event := <-nodeUpdates:
			c.handlePluginEvent(event)
		case <-c.shutdownCtx.Done():
			close(c.shutdownCh)
			return
		}
	}
}

// resyncPluginsFromRegistry does a full sync of the running instance
// managers against those in the registry. we primarily will use update
// events from the registry.
func (c *csiManager) resyncPluginsFromRegistry(ptype string) {

	c.instancesLock.Lock()
	defer c.instancesLock.Unlock()

	plugins := c.registry.ListPlugins(ptype)
	seen := make(map[string]struct{}, len(plugins))

	// For every plugin in the registry, ensure that we have an existing plugin
	// running. Also build the map of valid plugin names.
	// Note: monolith plugins that run as both controllers and nodes get a
	// separate instance manager for both modes.
	for _, plugin := range plugins {
		seen[plugin.Name] = struct{}{}
		c.ensureInstance(plugin)
	}

	// For every instance manager, if we did not find it during the plugin
	// iterator, shut it down and remove it from the table.
	instances := c.instancesForType(ptype)
	for name, mgr := range instances {
		if _, ok := seen[name]; !ok {
			c.ensureNoInstance(mgr.info)
		}
	}
}

// handlePluginEvent syncs a single event against the plugin registry
func (c *csiManager) handlePluginEvent(event *dynamicplugins.PluginUpdateEvent) {
	if event == nil || event.Info == nil {
		return
	}
	c.logger.Trace("dynamic plugin event",
		"event", event.EventType,
		"plugin_id", event.Info.Name,
		"plugin_alloc_id", event.Info.AllocID)

	c.instancesLock.Lock()
	defer c.instancesLock.Unlock()

	switch event.EventType {
	case dynamicplugins.EventTypeRegistered:
		c.ensureInstance(event.Info)
	case dynamicplugins.EventTypeDeregistered:
		c.ensureNoInstance(event.Info)
	default:
		c.logger.Error("received unknown dynamic plugin event type",
			"type", event.EventType)
	}
}

// Ensure we have an instance manager for the plugin and add it to
// the CSI manager's tracking table for that plugin type.
// Assumes that c.instances has been locked.
func (c *csiManager) ensureInstance(plugin *dynamicplugins.PluginInfo) {
	name := plugin.Name
	ptype := plugin.Type
	instances := c.instancesForType(ptype)
	mgr, ok := instances[name]
	if !ok {
		c.logger.Debug("detected new CSI plugin", "name", name, "type", ptype, "alloc", plugin.AllocID)
		mgr := newInstanceManager(c.logger, c.eventer, c.updateNodeCSIInfoFunc, plugin)
		instances[name] = mgr
		mgr.run()
	} else if mgr.allocID != plugin.AllocID {
		mgr.shutdown()
		c.logger.Debug("detected update for CSI plugin", "name", name, "type", ptype, "alloc", plugin.AllocID)
		mgr := newInstanceManager(c.logger, c.eventer, c.updateNodeCSIInfoFunc, plugin)
		instances[name] = mgr
		mgr.run()

	}
}

// Shut down the instance manager for a plugin and remove it from
// the CSI manager's tracking table for that plugin type.
// Assumes that c.instances has been locked.
func (c *csiManager) ensureNoInstance(plugin *dynamicplugins.PluginInfo) {
	name := plugin.Name
	ptype := plugin.Type
	instances := c.instancesForType(ptype)
	if mgr, ok := instances[name]; ok {
		if mgr.allocID == plugin.AllocID {
			c.logger.Debug("shutting down CSI plugin", "name", name, "type", ptype, "alloc", plugin.AllocID)
			mgr.shutdown()
			delete(instances, name)
		}
	}
}

// Get the instance managers table for a specific plugin type,
// ensuring it's been initialized if it doesn't exist.
// Assumes that c.instances has been locked.
func (c *csiManager) instancesForType(ptype string) map[string]*instanceManager {
	pluginMap, ok := c.instances[ptype]
	if !ok {
		pluginMap = make(map[string]*instanceManager)
		c.instances[ptype] = pluginMap
	}
	return pluginMap
}

// Shutdown should gracefully shutdown all plugins managed by the manager.
// It must block until shutdown is complete
func (c *csiManager) Shutdown() {
	// Shut down the run loop
	c.shutdownCtxCancelFn()

	// Wait for plugin manager shutdown to complete so that we
	// don't try to shutdown instance managers while runLoop is
	// doing a resync
	<-c.shutdownCh

	// Shutdown all the instance managers in parallel
	var wg sync.WaitGroup
	for _, pluginMap := range c.instances {
		for _, mgr := range pluginMap {
			wg.Add(1)
			go func(mgr *instanceManager) {
				mgr.shutdown()
				wg.Done()
			}(mgr)
		}
	}
	wg.Wait()
}

// PluginType is the type of plugin which the manager manages
func (c *csiManager) PluginType() string {
	return "csi"
}
