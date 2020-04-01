package csimanager

import (
	"context"
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
		logger:    config.Logger,
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
	// instances should only be accessed from the run() goroutine and the shutdown
	// fn. It is a map of PluginType : [PluginName : instanceManager]
	instances map[string]map[string]*instanceManager

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

func (c *csiManager) MounterForVolume(ctx context.Context, vol *structs.CSIVolume) (VolumeMounter, error) {
	nodePlugins, hasAnyNodePlugins := c.instances["csi-node"]
	if !hasAnyNodePlugins {
		return nil, PluginNotFoundErr
	}

	mgr, hasPlugin := nodePlugins[vol.PluginID]
	if !hasPlugin {
		return nil, PluginNotFoundErr
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
	if event == nil {
		return
	}
	c.logger.Trace("dynamic plugin event",
		"event", event.EventType,
		"plugin_id", event.Info.Name,
		"plugin_alloc_id", event.Info.AllocID)

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
func (c *csiManager) ensureInstance(plugin *dynamicplugins.PluginInfo) {
	name := plugin.Name
	ptype := plugin.Type
	instances := c.instancesForType(ptype)
	if _, ok := instances[name]; !ok {
		c.logger.Debug("detected new CSI plugin", "name", name, "type", ptype)
		mgr := newInstanceManager(c.logger, c.eventer, c.updateNodeCSIInfoFunc, plugin)
		instances[name] = mgr
		mgr.run()
	}
}

// Shut down the instance manager for a plugin and remove it from
// the CSI manager's tracking table for that plugin type.
func (c *csiManager) ensureNoInstance(plugin *dynamicplugins.PluginInfo) {
	name := plugin.Name
	ptype := plugin.Type
	instances := c.instancesForType(ptype)
	if mgr, ok := instances[name]; ok {
		c.logger.Debug("shutting down CSI plugin", "name", name, "type", ptype)
		mgr.shutdown()
		delete(instances, name)
	}
}

// Get the instance managers table for a specific plugin type,
// ensuring it's been initialized if it doesn't exist.
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
