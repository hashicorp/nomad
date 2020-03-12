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

type Config struct {
	Logger                hclog.Logger
	DynamicRegistry       dynamicplugins.Registry
	UpdateNodeCSIInfoFunc UpdateNodeCSIInfoFunc
	PluginResyncPeriod    time.Duration
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
	// Ensure we have at least one full sync before starting
	c.resyncPluginsFromRegistry("csi-controller")
	c.resyncPluginsFromRegistry("csi-node")
	go c.runLoop()
}

func (c *csiManager) runLoop() {
	// TODO: Subscribe to the events channel from the registry to receive dynamic
	//       updates without a full resync
	timer := time.NewTimer(0)
	for {
		select {
		case <-c.shutdownCtx.Done():
			close(c.shutdownCh)
			return
		case <-timer.C:
			c.resyncPluginsFromRegistry("csi-controller")
			c.resyncPluginsFromRegistry("csi-node")
			timer.Reset(c.pluginResyncPeriod)
		}
	}
}

// resyncPluginsFromRegistry does a full sync of the running instance managers
// against those in the registry. Eventually we should primarily be using
// update events from the registry, but this is an ok fallback for now.
func (c *csiManager) resyncPluginsFromRegistry(ptype string) {
	plugins := c.registry.ListPlugins(ptype)
	seen := make(map[string]struct{}, len(plugins))

	pluginMap, ok := c.instances[ptype]
	if !ok {
		pluginMap = make(map[string]*instanceManager)
		c.instances[ptype] = pluginMap
	}

	// For every plugin in the registry, ensure that we have an existing plugin
	// running. Also build the map of valid plugin names.
	for _, plugin := range plugins {
		seen[plugin.Name] = struct{}{}
		if _, ok := pluginMap[plugin.Name]; !ok {
			c.logger.Debug("detected new CSI plugin", "name", plugin.Name, "type", ptype)
			mgr := newInstanceManager(c.logger, c.updateNodeCSIInfoFunc, plugin)
			pluginMap[plugin.Name] = mgr
			mgr.run()
		}
	}

	// For every instance manager, if we did not find it during the plugin
	// iterator, shut it down and remove it from the table.
	for name, mgr := range pluginMap {
		if _, ok := seen[name]; !ok {
			c.logger.Info("shutting down CSI plugin", "name", name, "type", ptype)
			mgr.shutdown()
			delete(pluginMap, name)
		}
	}
}

// Shutdown should gracefully shutdown all plugins managed by the manager.
// It must block until shutdown is complete
func (c *csiManager) Shutdown() {
	// Shut down the run loop
	c.shutdownCtxCancelFn()

	// Wait for plugin manager shutdown to complete
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
