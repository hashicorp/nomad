// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package devicemanager is used to manage device plugins
package devicemanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/pluginmanager"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

// Manager is the interface used to manage device plugins
type Manager interface {
	pluginmanager.PluginManager

	// Reserve is used to reserve a set of devices
	Reserve(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error)

	// AllStats is used to retrieve all the latest statistics for all devices.
	AllStats() []*device.DeviceGroupStats

	// DeviceStats returns the device statistics for the given device.
	DeviceStats(d *structs.AllocatedDeviceResource) (*device.DeviceGroupStats, error)
}

// StateStorage is used to persist the device managers state across
// agent restarts.
type StateStorage interface {
	// GetDevicePluginState is used to retrieve the device manager's plugin
	// state.
	GetDevicePluginState() (*state.PluginState, error)

	// PutDevicePluginState is used to store the device manager's plugin
	// state.
	PutDevicePluginState(state *state.PluginState) error
}

// UpdateNodeDevices is a callback for updating the set of devices on a node.
type UpdateNodeDevicesFn func(devices []*structs.NodeDeviceResource)

// StorePluginReattachFn is used to store plugin reattachment configurations.
type StorePluginReattachFn func(*plugin.ReattachConfig) error

// Config is used to configure a device manager
type Config struct {
	// Logger is the logger used by the device manager
	Logger log.Logger

	// Loader is the plugin loader
	Loader loader.PluginCatalog

	// PluginConfig is the config passed to the launched plugins
	PluginConfig *base.AgentConfig

	// Updater is used to update the node when device information changes
	Updater UpdateNodeDevicesFn

	// StatsInterval is the interval at which to collect statistics
	StatsInterval time.Duration

	// State is used to manage the device managers state
	State StateStorage
}

// manager is used to manage a set of device plugins
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
	updater UpdateNodeDevicesFn

	// statsInterval is the duration at which to collect statistics
	statsInterval time.Duration

	// fingerprintResCh is used to be triggered that there are new devices
	fingerprintResCh chan struct{}

	// instances is the list of managed devices
	instances map[loader.PluginID]*instanceManager

	// reattachConfigs stores the plugin reattach configs
	reattachConfigs    map[loader.PluginID]*pstructs.ReattachConfig
	reattachConfigLock sync.Mutex
}

// New returns a new device manager
func New(c *Config) *manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		logger:           c.Logger.Named("device_mgr"),
		state:            c.State,
		ctx:              ctx,
		cancel:           cancel,
		loader:           c.Loader,
		pluginConfig:     c.PluginConfig,
		updater:          c.Updater,
		statsInterval:    c.StatsInterval,
		instances:        make(map[loader.PluginID]*instanceManager),
		reattachConfigs:  make(map[loader.PluginID]*pstructs.ReattachConfig),
		fingerprintResCh: make(chan struct{}, 1),
	}
}

// PluginType identifies this manager to the plugin manager and satisfies the PluginManager interface.
func (*manager) PluginType() string { return base.PluginTypeDevice }

// Run starts the device manager. The manager will shutdown any previously
// launched plugin and then begin fingerprinting and stats collection on all new
// device plugins.
func (m *manager) Run() {
	// Check if there are any plugins that didn't get cleanly shutdown before
	// and if there are shut them down.
	m.cleanupStalePlugins()

	// Get device plugins
	devices := m.loader.Catalog()[base.PluginTypeDevice]
	if len(devices) == 0 {
		m.logger.Debug("exiting since there are no device plugins")
		m.cancel()
		return
	}

	for _, d := range devices {
		id := loader.PluginInfoID(d)
		storeFn := func(c *plugin.ReattachConfig) error {
			id := id
			return m.storePluginReattachConfig(id, c)
		}
		m.instances[id] = newInstanceManager(&instanceManagerConfig{
			Logger:           m.logger,
			Ctx:              m.ctx,
			Loader:           m.loader,
			StoreReattach:    storeFn,
			PluginConfig:     m.pluginConfig,
			Id:               &id,
			FingerprintOutCh: m.fingerprintResCh,
			StatsInterval:    m.statsInterval,
		})
	}

	// Now start the fingerprint handler
	go m.fingerprint()
}

// fingerprint is the main fingerprint loop
func (m *manager) fingerprint() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.fingerprintResCh:
		}

		// Collect the data
		var fingerprinted []*device.DeviceGroup
		for _, i := range m.instances {
			fingerprinted = append(fingerprinted, i.Devices()...)
		}

		// Convert and update
		out := make([]*structs.NodeDeviceResource, len(fingerprinted))
		for i, f := range fingerprinted {
			out[i] = convertDeviceGroup(f)
		}

		// Call the updater
		m.updater(out)
	}
}

// Shutdown cleans up all the plugins
func (m *manager) Shutdown() {
	// Cancel the context to stop any requests
	m.cancel()

	// Go through and shut everything down
	for _, i := range m.instances {
		i.cleanup()
	}
}

func (m *manager) WaitForFirstFingerprint(ctx context.Context) <-chan struct{} {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		var wg sync.WaitGroup
		for i := range m.instances {
			wg.Add(1)
			go func(instance *instanceManager) {
				instance.WaitForFirstFingerprint(ctx)
				wg.Done()
			}(m.instances[i])
		}
		wg.Wait()
		cancel()
	}()
	return ctx.Done()
}

// Reserve reserves the given allocated device. If the device is unknown, an
// UnknownDeviceErr is returned.
func (m *manager) Reserve(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error) {
	// Go through each plugin and see if it can reserve the resources
	for _, i := range m.instances {
		if !i.HasDevices(d) {
			continue
		}

		// We found a match so reserve
		return i.Reserve(d)
	}

	return nil, UnknownDeviceErrFromAllocated("failed to reserve devices", d)
}

// AllStats returns statistics for all the devices
func (m *manager) AllStats() []*device.DeviceGroupStats {
	// Go through each plugin and collect stats
	var stats []*device.DeviceGroupStats
	for _, i := range m.instances {
		stats = append(stats, i.AllStats()...)
	}

	return stats
}

// DeviceStats returns the statistics for the passed devices. If the device is unknown, an
// UnknownDeviceErr is returned.
func (m *manager) DeviceStats(d *structs.AllocatedDeviceResource) (*device.DeviceGroupStats, error) {
	// Go through each plugin and see if it has the requested devices
	for _, i := range m.instances {
		if !i.HasDevices(d) {
			continue
		}

		// We found a match so reserve
		return i.DeviceStats(d), nil
	}

	return nil, UnknownDeviceErrFromAllocated("failed to collect statistics", d)
}

// cleanupStalePlugins reads the device managers state and shuts down any
// previously launched plugin.
func (m *manager) cleanupStalePlugins() error {

	// Read the old plugin state
	s, err := m.state.GetDevicePluginState()
	if err != nil {
		return fmt.Errorf("failed to read plugin state: %v", err)
	}

	// No state was stored so there is nothing to do.
	if s == nil {
		return nil
	}

	// For each plugin go through and try to shut it down
	var mErr multierror.Error
	for name, c := range s.ReattachConfigs {
		rc, err := pstructs.ReattachConfigToGoPlugin(c)
		if err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("failed to convert reattach config: %v", err))
			continue
		}

		instance, err := m.loader.Reattach(name, base.PluginTypeDevice, rc)
		if err != nil {
			_ = multierror.Append(&mErr, fmt.Errorf("failed to reattach to plugin %q: %v", name, err))
			continue
		}

		// Kill the instance
		instance.Kill()
	}

	return mErr.ErrorOrNil()
}

// storePluginReattachConfig is used as a callback to the instance managers and
// persists thhe plugin reattach configurations.
func (m *manager) storePluginReattachConfig(id loader.PluginID, c *plugin.ReattachConfig) error {
	m.reattachConfigLock.Lock()
	defer m.reattachConfigLock.Unlock()

	// Store the new reattach config
	m.reattachConfigs[id] = pstructs.ReattachConfigFromGoPlugin(c)

	// Persist the state
	s := &state.PluginState{
		ReattachConfigs: make(map[string]*pstructs.ReattachConfig, len(m.reattachConfigs)),
	}

	for id, c := range m.reattachConfigs {
		s.ReattachConfigs[id.Name] = c
	}

	return m.state.PutDevicePluginState(s)
}
