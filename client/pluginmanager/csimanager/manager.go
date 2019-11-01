package csimanager

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginregistry"
	"github.com/hashicorp/nomad/nomad/structs"
)

// defaultPluginResyncPeriod is the time interval used to do a full resync
// against the pluginregistry, to account for missed updates.
const defaultPluginResyncPeriod = 30 * time.Second

// UpdateNodeCSIInfoFunc is the callback used to update the node from
// fingerprinting
type UpdateNodeCSIInfoFunc func(string, *structs.CSIInfo)

type Config struct {
	Logger                hclog.Logger
	PluginRegistry        pluginregistry.Registry
	UpdateNodeCSIInfoFunc UpdateNodeCSIInfoFunc
	PluginResyncPeriod    time.Duration
}

// New returns a new PluginManager that will handle managing CSI plugins from
// the PluginRegistry from the provided Config.
func New(config *Config) pluginmanager.PluginManager {
	// Use a dedicated internal context for managing plugin shutdown.
	ctx, cancelFn := context.WithCancel(context.Background())

	if config.PluginResyncPeriod == 0 {
		config.PluginResyncPeriod = defaultPluginResyncPeriod
	}

	return &csiManager{
		logger:    config.Logger,
		registry:  config.PluginRegistry,
		instances: make(map[string]*instanceManager),

		updateNodeCSIInfoFunc: config.UpdateNodeCSIInfoFunc,
		pluginResyncPeriod:    config.PluginResyncPeriod,

		shutdownCtx:         ctx,
		shutdownCtxCancelFn: cancelFn,
		shutdownCh:          make(chan struct{}),
	}
}

type csiManager struct {
	// instances should only be accessed from the run() goroutine and the shutdown
	// fn.
	instances          map[string]*instanceManager
	registry           pluginregistry.Registry
	logger             hclog.Logger
	pluginResyncPeriod time.Duration

	updateNodeCSIInfoFunc UpdateNodeCSIInfoFunc

	shutdownCtx         context.Context
	shutdownCtxCancelFn context.CancelFunc
	shutdownCh          chan struct{}
}

// Run starts a plugin manager and should return early
func (c *csiManager) Run() {
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
			c.resyncPluginsFromRegistry()
			timer.Reset(c.pluginResyncPeriod)
		}
	}
}

// resyncPluginsFromRegistry does a full sync of the running instance managers
// against those in the registry. Eventually we should primarily be using
// update events from the registry, but this is an ok fallback for now.
func (c *csiManager) resyncPluginsFromRegistry() {
	plugins := c.registry.ListPlugins("csi")
	seen := make(map[string]struct{}, len(plugins))

	// For every plugin in the registry, ensure that we have an existing plugin
	// running. Also build the map of valid plugin names.
	for _, plugin := range plugins {
		seen[plugin.Name] = struct{}{}
		if _, ok := c.instances[plugin.Name]; !ok {
			c.logger.Debug("detected new CSI plugin", "name", plugin.Name)
			mgr := newInstanceManager(c.logger, c.updateNodeCSIInfoFunc, plugin)
			c.instances[plugin.Name] = mgr
			mgr.run()
		}
	}

	// For every instance manager, if we did not find it during the plugin
	// iterator, shut it down and remove it from the table.
	for name, mgr := range c.instances {
		if _, ok := seen[name]; !ok {
			c.logger.Info("shutting down CSI plugin", "name", name)
			mgr.shutdown()
			delete(c.instances, name)
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
	for _, mgr := range c.instances {
		wg.Add(1)
		go func(mgr *instanceManager) {
			mgr.shutdown()
			wg.Done()
		}(mgr)
	}
	wg.Wait()
}

// PluginType is the type of plugin which the manager manages
func (c *csiManager) PluginType() string {
	return "csi"
}
