// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package devicemanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	"github.com/hashicorp/nomad/plugins/device"
)

const (
	// statsBackoffBaseline is the baseline time for exponential backoff while
	// collecting device stats.
	statsBackoffBaseline = 5 * time.Second

	// statsBackoffLimit is the limit of the exponential backoff for collecting
	// device statistics.
	statsBackoffLimit = 30 * time.Minute
)

// instanceManagerConfig configures a device instance manager
type instanceManagerConfig struct {
	// Logger is the logger used by the device instance manager
	Logger log.Logger

	// Ctx is used to shutdown the device instance manager
	Ctx context.Context

	// Loader is the plugin loader
	Loader loader.PluginCatalog

	// StoreReattach is used to store a plugins reattach config
	StoreReattach StorePluginReattachFn

	// PluginConfig is the config passed to the launched plugins
	PluginConfig *base.AgentConfig

	// Id is the ID of the plugin being managed
	Id *loader.PluginID

	// FingerprintOutCh is used to emit new fingerprinted devices
	FingerprintOutCh chan<- struct{}

	// StatsInterval is the interval at which we collect statistics.
	StatsInterval time.Duration
}

// instanceManager is used to manage a single device plugin
type instanceManager struct {
	// logger is the logger used by the device instance manager
	logger log.Logger

	// ctx is used to shutdown the device manager
	ctx context.Context

	// cancel is used to shutdown management of this device plugin
	cancel context.CancelFunc

	// loader is the plugin loader
	loader loader.PluginCatalog

	// storeReattach is used to store a plugins reattach config
	storeReattach StorePluginReattachFn

	// pluginConfig is the config passed to the launched plugins
	pluginConfig *base.AgentConfig

	// id is the ID of the plugin being managed
	id *loader.PluginID

	// fingerprintOutCh is used to emit new fingerprinted devices
	fingerprintOutCh chan<- struct{}

	// plugin is the plugin instance being managed
	plugin loader.PluginInstance

	// device is the device plugin being managed
	device device.DevicePlugin

	// pluginLock locks access to the device and plugin
	pluginLock sync.Mutex

	// shutdownLock is used to serialize attempts to shutdown
	shutdownLock sync.Mutex

	// devices is the set of fingerprinted devices
	devices    []*device.DeviceGroup
	deviceLock sync.RWMutex

	// statsInterval is the interval at which we collect statistics.
	statsInterval time.Duration

	// deviceStats is the set of statistics objects per devices
	deviceStats     []*device.DeviceGroupStats
	deviceStatsLock sync.RWMutex

	// firstFingerprintCh is used to trigger that we have successfully
	// fingerprinted once. It is used to gate launching the stats collection.
	firstFingerprintCh chan struct{}
	hasFingerprinted   bool
}

// newInstanceManager returns a new device instance manager. It is expected that
// the context passed in the configuration is cancelled in order to shutdown
// launched goroutines.
func newInstanceManager(c *instanceManagerConfig) *instanceManager {

	ctx, cancel := context.WithCancel(c.Ctx)
	i := &instanceManager{
		logger:             c.Logger.With("plugin", c.Id.Name),
		ctx:                ctx,
		cancel:             cancel,
		loader:             c.Loader,
		storeReattach:      c.StoreReattach,
		pluginConfig:       c.PluginConfig,
		id:                 c.Id,
		fingerprintOutCh:   c.FingerprintOutCh,
		statsInterval:      c.StatsInterval,
		firstFingerprintCh: make(chan struct{}),
	}

	go i.run()
	return i
}

// HasDevices returns if the instance is managing the passed devices
func (i *instanceManager) HasDevices(d *structs.AllocatedDeviceResource) bool {
	i.deviceLock.RLock()
	defer i.deviceLock.RUnlock()

OUTER:
	for _, dev := range i.devices {
		if dev.Name != d.Name || dev.Type != d.Type || dev.Vendor != d.Vendor {
			continue
		}

		// Check that we have all the requested devices
		ids := make(map[string]struct{}, len(dev.Devices))
		for _, inst := range dev.Devices {
			ids[inst.ID] = struct{}{}
		}

		for _, reqID := range d.DeviceIDs {
			if _, ok := ids[reqID]; !ok {
				continue OUTER
			}
		}

		return true
	}

	return false
}

// AllStats returns all the device statistics returned by the device plugin.
func (i *instanceManager) AllStats() []*device.DeviceGroupStats {
	i.deviceStatsLock.RLock()
	defer i.deviceStatsLock.RUnlock()
	return i.deviceStats
}

// DeviceStats returns the device statistics for the request devices.
func (i *instanceManager) DeviceStats(d *structs.AllocatedDeviceResource) *device.DeviceGroupStats {
	i.deviceStatsLock.RLock()
	defer i.deviceStatsLock.RUnlock()

	// Find the device in question and then gather the instance statistics we
	// are interested in
	for _, group := range i.deviceStats {
		if group.Vendor != d.Vendor || group.Type != d.Type || group.Name != d.Name {
			continue
		}

		// We found the group we want so now grab the instance stats
		out := &device.DeviceGroupStats{
			Vendor:        d.Vendor,
			Type:          d.Type,
			Name:          d.Name,
			InstanceStats: make(map[string]*device.DeviceStats, len(d.DeviceIDs)),
		}

		for _, id := range d.DeviceIDs {
			out.InstanceStats[id] = group.InstanceStats[id]
		}

		return out
	}

	return nil
}

// Reserve reserves the given devices
func (i *instanceManager) Reserve(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error) {
	// Get a device plugin
	devicePlugin, err := i.dispense()
	if err != nil {
		i.logger.Error("dispensing plugin failed", "error", err)
		return nil, err
	}

	// Send the reserve request
	return devicePlugin.Reserve(d.DeviceIDs)
}

// Devices returns the detected devices.
func (i *instanceManager) Devices() []*device.DeviceGroup {
	i.deviceLock.RLock()
	defer i.deviceLock.RUnlock()
	return i.devices
}

// WaitForFirstFingerprint waits until either the plugin fingerprints, the
// passed context is done, or the plugin instance manager is shutdown.
func (i *instanceManager) WaitForFirstFingerprint(ctx context.Context) {
	select {
	case <-i.ctx.Done():
	case <-ctx.Done():
	case <-i.firstFingerprintCh:
	}
}

// run is a long lived goroutine that starts the fingerprinting and stats
// collection goroutine and then shutsdown the plugin on exit.
func (i *instanceManager) run() {
	// Dispense once to ensure we are given a valid plugin
	if _, err := i.dispense(); err != nil {
		i.logger.Error("dispensing initial plugin failed", "error", err)
		return
	}

	// Create a waitgroup to block on shutdown for all created goroutines to
	// exit
	var wg sync.WaitGroup

	// Start the fingerprinter
	wg.Add(1)
	go func() {
		i.fingerprint()
		wg.Done()
	}()

	// Wait for a valid result before starting stats collection
	select {
	case <-i.ctx.Done():
		goto DONE
	case <-i.firstFingerprintCh:
	}

	// Start stats
	wg.Add(1)
	go func() {
		i.collectStats()
		wg.Done()
	}()

	// Do a final cleanup
DONE:
	wg.Wait()
	i.cleanup()
}

// dispense is used to dispense a plugin.
func (i *instanceManager) dispense() (plugin device.DevicePlugin, err error) {
	i.pluginLock.Lock()
	defer i.pluginLock.Unlock()

	// See if we already have a running instance
	if i.plugin != nil && !i.plugin.Exited() {
		return i.device, nil
	}

	// Get an instance of the plugin
	pluginInstance, err := i.loader.Dispense(i.id.Name, i.id.PluginType, i.pluginConfig, i.logger)
	if err != nil {
		// Retry as the error just indicates the singleton has exited
		if err == singleton.SingletonPluginExited {
			pluginInstance, err = i.loader.Dispense(i.id.Name, i.id.PluginType, i.pluginConfig, i.logger)
		}

		// If we still have an error there is a real problem
		if err != nil {
			return nil, fmt.Errorf("failed to start plugin: %v", err)
		}
	}

	// Convert to a fingerprint plugin
	device, ok := pluginInstance.Plugin().(device.DevicePlugin)
	if !ok {
		pluginInstance.Kill()
		return nil, fmt.Errorf("plugin loaded does not implement the driver interface")
	}

	// Store the plugin and device
	i.plugin = pluginInstance
	i.device = device

	// Store the reattach config
	if c, ok := pluginInstance.ReattachConfig(); ok {
		i.storeReattach(c)
	}

	return device, nil
}

// cleanup shutsdown the plugin
func (i *instanceManager) cleanup() {
	i.shutdownLock.Lock()
	i.pluginLock.Lock()
	defer i.pluginLock.Unlock()
	defer i.shutdownLock.Unlock()

	if i.plugin != nil && !i.plugin.Exited() {
		i.plugin.Kill()
		i.storeReattach(nil)
	}
}

// fingerprint is a long lived routine used to fingerprint the device
func (i *instanceManager) fingerprint() {
START:
	// Get a device plugin
	devicePlugin, err := i.dispense()
	if err != nil {
		i.logger.Error("dispensing plugin failed", "error", err)
		i.cancel()
		return
	}

	// Start fingerprinting
	fingerprintCh, err := devicePlugin.Fingerprint(i.ctx)
	if err == device.ErrPluginDisabled {
		i.logger.Info("fingerprinting failed: plugin is not enabled")
		i.handleFingerprintError()
		return
	} else if err != nil {
		i.logger.Error("fingerprinting failed", "error", err)
		i.handleFingerprintError()
		return
	}

	var fresp *device.FingerprintResponse
	var ok bool
	for {
		select {
		case <-i.ctx.Done():
			return
		case fresp, ok = <-fingerprintCh:
		}

		if !ok {
			i.logger.Trace("exiting since fingerprinting gracefully shutdown")
			i.handleFingerprintError()
			return
		}

		// Guard against error by the plugin
		if fresp == nil {
			continue
		}

		// Handle any errors
		if fresp.Error != nil {
			if fresp.Error == bstructs.ErrPluginShutdown {
				i.logger.Error("plugin exited unexpectedly")
				goto START
			}

			i.logger.Error("fingerprinting returned an error", "error", fresp.Error)
			i.handleFingerprintError()
			return
		}

		if err := i.handleFingerprint(fresp); err != nil {
			// Cancel the context so we cleanup all goroutines
			i.logger.Error("returned devices failed fingerprinting", "error", err)
			i.handleFingerprintError()
		}
	}
}

// handleFingerprintError exits the manager and shutsdown the plugin.
func (i *instanceManager) handleFingerprintError() {
	// Clear out the devices and trigger a node update
	i.deviceLock.Lock()
	defer i.deviceLock.Unlock()

	// If we have fingerprinted before clear it out
	if i.hasFingerprinted {
		// Store the new devices
		i.devices = nil

		// Trigger that the we have new devices
		select {
		case i.fingerprintOutCh <- struct{}{}:
		default:
		}
	}

	// Cancel the context so we cleanup all goroutines
	i.cancel()
}

// handleFingerprint stores the new devices and triggers the fingerprint output
// channel. An error is returned if the passed devices don't pass validation.
func (i *instanceManager) handleFingerprint(f *device.FingerprintResponse) error {
	// Validate the received devices
	var validationErr multierror.Error
	for i, d := range f.Devices {
		if err := d.Validate(); err != nil {
			multierror.Append(&validationErr, multierror.Prefix(err, fmt.Sprintf("device group %d: ", i)))
		}
	}

	if err := validationErr.ErrorOrNil(); err != nil {
		return err
	}

	i.deviceLock.Lock()
	defer i.deviceLock.Unlock()

	// Store the new devices
	i.devices = f.Devices

	// Mark that we have received data
	if !i.hasFingerprinted {
		close(i.firstFingerprintCh)
		i.hasFingerprinted = true
	}

	// Trigger that we have data to pull
	select {
	case i.fingerprintOutCh <- struct{}{}:
	default:
	}

	return nil
}

// collectStats is a long lived goroutine for collecting device statistics. It
// handles errors by backing off exponentially and retrying.
func (i *instanceManager) collectStats() {
	var attempt uint64
	var backoff time.Duration

START:
	// Get a device plugin
	devicePlugin, err := i.dispense()
	if err != nil {
		i.logger.Error("dispensing plugin failed", "error", err)
		i.cancel()
		return
	}

	// Start stats collection
	statsCh, err := devicePlugin.Stats(i.ctx, i.statsInterval)
	if err != nil {
		i.logger.Error("stats collection failed", "error", err)
		return
	}

	var sresp *device.StatsResponse
	var ok bool
	for {
		select {
		case <-i.ctx.Done():
			return
		case sresp, ok = <-statsCh:
		}

		if !ok {
			i.logger.Trace("exiting since stats gracefully shutdown")
			return
		}

		// Guard against error by the plugin
		if sresp == nil {
			continue
		}

		// Handle any errors
		if sresp.Error != nil {
			if sresp.Error == bstructs.ErrPluginShutdown {
				i.logger.Error("plugin exited unexpectedly")
				goto START
			}

			// Retry with an exponential backoff
			backoff = helper.Backoff(statsBackoffBaseline, statsBackoffLimit, attempt)
			attempt++

			i.logger.Error("stats returned an error", "error", err, "retry", backoff)

			select {
			case <-i.ctx.Done():
				return
			case <-time.After(backoff):
				goto START
			}
		}

		// Reset the attempts since we got statistics
		attempt = 0

		// Store the new stats
		if sresp.Groups != nil {
			i.deviceStatsLock.Lock()
			i.deviceStats = sresp.Groups
			i.deviceStatsLock.Unlock()
		}
	}
}
