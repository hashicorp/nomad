package drivermanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared"
	"github.com/hashicorp/nomad/plugins/shared/loader"
)

// Manager is the interface used to manage driver plugins
type Manager interface {
	RegisterEventHandler(driver, taskID string, handler EventHandler)
	DeregisterEventHandler(driver, taskID string)

	Dispense(driver string) (drivers.DriverPlugin, error)
}

// EventHandler can be registered with a Manager to be called for a matching task.
// The handler should not block execution.
type EventHandler func(*drivers.TaskEvent)

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
type UpdateNodeDriverInfoFn func(string, *structs.DriverInfo) *structs.Node

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
	PluginConfig *base.ClientAgentConfig

	// Updater is used to update the node when driver information changes
	Updater UpdateNodeDriverInfoFn

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
	pluginConfig *base.ClientAgentConfig

	// updater is used to update the node when device information changes
	updater UpdateNodeDriverInfoFn

	// instances is the list of managed devices, access is serialized by instanceMu
	instances   map[string]*instanceManager
	instancesMu sync.Mutex

	// reattachConfigs stores the plugin reattach configs
	reattachConfigs    map[loader.PluginID]*shared.ReattachConfig
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
		logger:          c.Logger.Named("driver_mgr"),
		state:           c.State,
		ctx:             ctx,
		cancel:          cancel,
		loader:          c.Loader,
		pluginConfig:    c.PluginConfig,
		updater:         c.Updater,
		instances:       make(map[string]*instanceManager),
		reattachConfigs: make(map[loader.PluginID]*shared.ReattachConfig),
		allowedDrivers:  c.AllowedDrivers,
		blockedDrivers:  c.BlockedDrivers,
		readyCh:         make(chan struct{}),
	}
}

// PluginType returns the type of plugin this mananger mananges
func (*manager) PluginType() string { return base.PluginTypeDriver }

// Run starts the mananger, initializes driver plugins and blocks until Shutdown
// is called.
func (m *manager) Run() {
	// Load any previous plugin reattach configuration
	m.loadReattachConfigs()

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
		// Skip drivers that are not in the allowed list if it is set.
		if _, ok := m.allowedDrivers[id.Name]; len(m.allowedDrivers) > 0 && !ok {
			skippedDrivers = append(skippedDrivers, id.Name)
			continue
		}
		// Skip fingerprinting drivers that are in the blocked list
		if _, ok := m.blockedDrivers[id.Name]; ok {
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

	// wait for shutdown
	<-m.ctx.Done()
}

// Shutdown cleans up all the plugins
func (m *manager) Shutdown() {
	// Cancel the context to stop any requests
	m.cancel()

	m.instancesMu.Lock()
	defer m.instancesMu.Unlock()

	// Go through and shut everything down
	for _, i := range m.instances {
		i.cleanup()
	}
}

func (m *manager) Ready() <-chan struct{} {
	ctx, cancel := context.WithTimeout(m.ctx, time.Second*10)
	go func() {
		defer cancel()
		// We don't want to start initial fingerprint wait until Run loop has
		// finished
		select {
		case <-m.readyCh:
		case <-m.ctx.Done():
			return
		}

		var availDrivers []string
		for name, instance := range m.instances {
			instance.WaitForFirstFingerprint(ctx)
			if instance.lastHealthState != drivers.HealthStateUndetected {
				availDrivers = append(availDrivers, name)
			}
		}
		m.logger.Debug("detected drivers", "drivers", availDrivers)
	}()
	return ctx.Done()
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
			id := loader.PluginID{
				PluginType: base.PluginTypeDriver,
				Name:       name,
			}
			m.reattachConfigs[id] = c
		}
	}
	return nil
}

// storePluginReattachConfig is used as a callback to the instance managers and
// persists thhe plugin reattach configurations.
func (m *manager) storePluginReattachConfig(id loader.PluginID, c *plugin.ReattachConfig) error {
	m.reattachConfigLock.Lock()
	defer m.reattachConfigLock.Unlock()

	// Store the new reattach config
	m.reattachConfigs[id] = shared.ReattachConfigFromGoPlugin(c)

	// Persist the state
	s := &state.PluginState{
		ReattachConfigs: make(map[string]*shared.ReattachConfig, len(m.reattachConfigs)),
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
		c, err := shared.ReattachConfigToGoPlugin(cfg)
		if err != nil {
			m.logger.Warn("failed to read plugin reattach config", "config", cfg, "error", err)
			return nil, false
		}
		return c, true
	}
	return nil, false
}

func (m *manager) RegisterEventHandler(driver, taskID string, handler EventHandler) {
	m.instancesMu.Lock()
	if d, ok := m.instances[driver]; ok {
		d.registerEventHandler(taskID, handler)
	}
	m.instancesMu.Unlock()
}

func (m *manager) DeregisterEventHandler(driver, taskID string) {
	m.instancesMu.Lock()
	if d, ok := m.instances[driver]; ok {
		d.deregisterEventHandler(taskID)
	}
	m.instancesMu.Unlock()
}

func (m *manager) Dispense(d string) (drivers.DriverPlugin, error) {
	m.instancesMu.Lock()
	defer m.instancesMu.Unlock()
	if instance, ok := m.instances[d]; ok {
		return instance.dispense()
	}

	return nil, fmt.Errorf("driver not found")
}
