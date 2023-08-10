// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drivermanager

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

// ErrDriverNotFound is returned during Dispense when the requested driver
// plugin is not found in the plugin catalog
var ErrDriverNotFound = fmt.Errorf("driver not found")

// Manager is the interface used to manage driver plugins
type Manager interface {
	pluginmanager.PluginManager

	// Dispense returns a drivers.DriverPlugin for the given driver plugin name
	// handling reattaching to an existing driver if available
	Dispense(driver string) (drivers.DriverPlugin, error)
}

// TaskExecHandler is function to be called for executing commands in a task
type TaskExecHandler func(
	ctx context.Context,
	command []string,
	tty bool,
	stream drivers.ExecTaskStream) error

// EventHandler is a callback to be called for a task.
// The handler should not block execution.
type EventHandler func(*drivers.TaskEvent)

// TaskEventHandlerFactory returns an event handler for a given allocID/task name
type TaskEventHandlerFactory func(allocID, taskName string) EventHandler

// StateStorage is used to persist the driver managers state across
// agent restarts.
type StateStorage interface {
	// GetDevicePluginState is used to retrieve the device manager's plugin
	// state.
	GetDriverPluginState() (*state.PluginState, error)

	// PutDevicePluginState is used to store the device manager's plugin
	// state.
	PutDriverPluginState(state *state.PluginState) error
}

// UpdateNodeDriverInfoFn is the callback used to update the node from
// fingerprinting
type UpdateNodeDriverInfoFn func(string, *structs.DriverInfo)

// StorePluginReattachFn is used to store plugin reattachment configurations.
type StorePluginReattachFn func(*plugin.ReattachConfig) error

// FetchPluginReattachFn is used to retrieve the stored plugin reattachment
// configuration.
type FetchPluginReattachFn func() (*plugin.ReattachConfig, bool)

// Config is used to configure a driver manager
type Config struct {
	// Logger is the logger used by the device manager
	Logger log.Logger

	// Loader is the plugin loader
	Loader loader.PluginCatalog

	// PluginConfig is the config passed to the launched plugins
	PluginConfig *base.AgentConfig

	// Updater is used to update the node when driver information changes
	Updater UpdateNodeDriverInfoFn

	// EventHandlerFactory is used to retrieve a task event handler
	EventHandlerFactory TaskEventHandlerFactory

	// State is used to manage the device managers state
	State StateStorage

	// AllowedDrivers if set will only start driver plugins for the given
	// drivers
	AllowedDrivers map[string]struct{}

	// BlockedDrivers if set will not allow the given driver plugins to start
	BlockedDrivers map[string]struct{}
}

// manager is used to manage a set of driver plugins
type manager struct {
	// logger is the logger used by the device manager
	logger log.Logger

	// state is used to manage the device managers state
	state StateStorage

	// ctx is used to shutdown the device manager
	ctx    context.Context
	cancel context.CancelFunc

	// loader is the plugin loader
	loader loader.PluginCatalog

	// pluginConfig is the config passed to the launched plugins
	pluginConfig *base.AgentConfig

	// updater is used to update the node when device information changes
	updater UpdateNodeDriverInfoFn

	// eventHandlerFactory is passed to the instance managers and used to forward
	// task events
	eventHandlerFactory TaskEventHandlerFactory

	// instances is the list of managed devices, access is serialized by instanceMu
	instances   map[string]*instanceManager
	instancesMu sync.RWMutex

	// reattachConfigs stores the plugin reattach configs
	reattachConfigs    map[loader.PluginID]*pstructs.ReattachConfig
	reattachConfigLock sync.Mutex

	// allows/block lists
	allowedDrivers map[string]struct{}
	blockedDrivers map[string]struct{}

	// readyCh is ticked once at the end of Run()
	readyCh chan struct{}
}

// New returns a new driver manager
func New(c *Config) *manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		logger:              c.Logger.Named("driver_mgr"),
		state:               c.State,
		ctx:                 ctx,
		cancel:              cancel,
		loader:              c.Loader,
		pluginConfig:        c.PluginConfig,
		updater:             c.Updater,
		eventHandlerFactory: c.EventHandlerFactory,
		instances:           make(map[string]*instanceManager),
		reattachConfigs:     make(map[loader.PluginID]*pstructs.ReattachConfig),
		allowedDrivers:      c.AllowedDrivers,
		blockedDrivers:      c.BlockedDrivers,
		readyCh:             make(chan struct{}),
	}
}

// PluginType returns the type of plugin this manager mananges
func (*manager) PluginType() string { return base.PluginTypeDriver }

// Run starts the manager, initializes driver plugins and blocks until Shutdown
// is called.
func (m *manager) Run() {
	// Load any previous plugin reattach configuration
	if err := m.loadReattachConfigs(); err != nil {
		m.logger.Warn("unable to load driver plugin reattach configs, a driver process may have been leaked",
			"error", err)
	}

	// Get driver plugins
	driversPlugins := m.loader.Catalog()[base.PluginTypeDriver]
	if len(driversPlugins) == 0 {
		m.logger.Debug("exiting since there are no driver plugins")
		m.cancel()
		return
	}

	var skippedDrivers []string
	for _, d := range driversPlugins {
		id := loader.PluginInfoID(d)
		if m.isDriverBlocked(id.Name) {
			skippedDrivers = append(skippedDrivers, id.Name)
			continue
		}

		storeFn := func(c *plugin.ReattachConfig) error {
			return m.storePluginReattachConfig(id, c)
		}
		fetchFn := func() (*plugin.ReattachConfig, bool) {
			return m.fetchPluginReattachConfig(id)
		}

		instance := newInstanceManager(&instanceManagerConfig{
			Logger:               m.logger,
			Ctx:                  m.ctx,
			Loader:               m.loader,
			StoreReattach:        storeFn,
			FetchReattach:        fetchFn,
			PluginConfig:         m.pluginConfig,
			ID:                   &id,
			UpdateNodeFromDriver: m.updater,
			EventHandlerFactory:  m.eventHandlerFactory,
		})

		m.instancesMu.Lock()
		m.instances[id.Name] = instance
		m.instancesMu.Unlock()
	}

	if len(skippedDrivers) > 0 {
		m.logger.Debug("drivers skipped due to allow/block list", "skipped_drivers", skippedDrivers)
	}

	// signal ready
	close(m.readyCh)
}

// Shutdown cleans up all the plugins
func (m *manager) Shutdown() {
	// Cancel the context to stop any requests
	m.cancel()

	m.instancesMu.RLock()
	defer m.instancesMu.RUnlock()

	// Go through and shut everything down
	for _, i := range m.instances {
		i.cleanup()
	}
}

func (m *manager) WaitForFirstFingerprint(ctx context.Context) <-chan struct{} {
	ctx, cancel := context.WithCancel(ctx)
	go m.waitForFirstFingerprint(ctx, cancel)
	return ctx.Done()
}

func (m *manager) waitForFirstFingerprint(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	// We don't want to start initial fingerprint wait until Run loop has
	// finished
	select {
	case <-m.readyCh:
	case <-ctx.Done():
		// parent context canceled or timedout
		return
	case <-m.ctx.Done():
		// shutdown called
		return
	}

	var mu sync.Mutex
	driversByStatus := map[drivers.HealthState][]string{}

	var wg sync.WaitGroup

	recordDriver := func(name string, lastHeath drivers.HealthState) {
		mu.Lock()
		defer mu.Unlock()

		updated := append(driversByStatus[lastHeath], name) //nolint:gocritic
		driversByStatus[lastHeath] = updated
	}

	// loop through instances and wait for each to finish initial fingerprint
	m.instancesMu.RLock()
	for n, i := range m.instances {
		wg.Add(1)
		go func(name string, instance *instanceManager) {
			defer wg.Done()
			instance.WaitForFirstFingerprint(ctx)
			recordDriver(name, instance.getLastHealth())
		}(n, i)
	}
	m.instancesMu.RUnlock()
	wg.Wait()

	m.logger.Debug("detected drivers", "drivers", driversByStatus)
}

func (m *manager) loadReattachConfigs() error {
	m.reattachConfigLock.Lock()
	defer m.reattachConfigLock.Unlock()

	s, err := m.state.GetDriverPluginState()
	if err != nil {
		return err
	}

	if s != nil {
		for name, c := range s.ReattachConfigs {
			if m.isDriverBlocked(name) {
				m.logger.Warn("reattach config for driver plugin found but driver is blocked due to allow/block list, killing plugin",
					"driver", name)
				m.shutdownBlockedDriver(name, c)
				continue
			}

			id := loader.PluginID{
				PluginType: base.PluginTypeDriver,
				Name:       name,
			}

			m.reattachConfigs[id] = c
		}
	}
	return nil
}

// shutdownBlockedDriver is used to forcefully shutdown a running driver plugin
// when it has been blocked due to allow/block lists
func (m *manager) shutdownBlockedDriver(name string, reattach *pstructs.ReattachConfig) {
	c, err := pstructs.ReattachConfigToGoPlugin(reattach)
	if err != nil {
		m.logger.Warn("failed to reattach and kill blocked driver plugin",
			"driver", name, "error", err)
		return

	}
	pluginInstance, err := m.loader.Reattach(name, base.PluginTypeDriver, c)
	if err != nil {
		m.logger.Warn("failed to reattach and kill blocked driver plugin",
			"driver", name, "error", err)
		return
	}

	if !pluginInstance.Exited() {
		pluginInstance.Kill()
	}
}

// storePluginReattachConfig is used as a callback to the instance managers and
// persists thhe plugin reattach configurations.
func (m *manager) storePluginReattachConfig(id loader.PluginID, c *plugin.ReattachConfig) error {
	m.reattachConfigLock.Lock()
	defer m.reattachConfigLock.Unlock()

	if c == nil {
		delete(m.reattachConfigs, id)
	} else {
		// Store the new reattach config
		m.reattachConfigs[id] = pstructs.ReattachConfigFromGoPlugin(c)
	}
	// Persist the state
	s := &state.PluginState{
		ReattachConfigs: make(map[string]*pstructs.ReattachConfig, len(m.reattachConfigs)),
	}

	for id, c := range m.reattachConfigs {
		s.ReattachConfigs[id.Name] = c
	}

	return m.state.PutDriverPluginState(s)
}

// fetchPluginReattachConfig is used as a callback to the instance managers and
// retrieves the plugin reattach config. If it has not been stored it will
// return nil
func (m *manager) fetchPluginReattachConfig(id loader.PluginID) (*plugin.ReattachConfig, bool) {
	m.reattachConfigLock.Lock()
	defer m.reattachConfigLock.Unlock()

	if cfg, ok := m.reattachConfigs[id]; ok {
		c, err := pstructs.ReattachConfigToGoPlugin(cfg)
		if err != nil {
			m.logger.Warn("failed to read plugin reattach config", "config", cfg, "error", err)
			delete(m.reattachConfigs, id)
			return nil, false
		}
		return c, true
	}
	return nil, false
}

func (m *manager) Dispense(d string) (drivers.DriverPlugin, error) {
	m.instancesMu.RLock()
	defer m.instancesMu.RUnlock()
	if instance, ok := m.instances[d]; ok {
		return instance.dispense()
	}

	return nil, ErrDriverNotFound
}

func (m *manager) isDriverBlocked(name string) bool {
	// Block drivers that are not in the allowed list if it is set.
	if _, ok := m.allowedDrivers[name]; len(m.allowedDrivers) > 0 && !ok {
		return true
	}

	// Block drivers that are in the blocked list
	if _, ok := m.blockedDrivers[name]; ok {
		return true
	}
	return false
}
