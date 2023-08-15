// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drivermanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/pluginutils/singleton"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	// driverFPBackoffBaseline is the baseline time for exponential backoff while
	// fingerprinting a driver.
	driverFPBackoffBaseline = 5 * time.Second

	// driverFPBackoffLimit is the limit of the exponential backoff for fingerprinting
	// a driver.
	driverFPBackoffLimit = 2 * time.Minute
)

// instanceManagerConfig configures a driver instance manager
type instanceManagerConfig struct {
	// Logger is the logger used by the driver instance manager
	Logger log.Logger

	// Ctx is used to shutdown the driver instance manager
	Ctx context.Context

	// Loader is the plugin loader
	Loader loader.PluginCatalog

	// StoreReattach is used to store a plugins reattach config
	StoreReattach StorePluginReattachFn

	// FetchReattach is used to retrieve a plugin's reattach config
	FetchReattach FetchPluginReattachFn

	// PluginConfig is the config passed to the launched plugins
	PluginConfig *base.AgentConfig

	// ID is the ID of the plugin being managed
	ID *loader.PluginID

	// updateNodeFromDriver is the callback used to update the node from fingerprinting
	UpdateNodeFromDriver UpdateNodeDriverInfoFn

	// EventHandlerFactory is used to fetch a task event handler
	EventHandlerFactory TaskEventHandlerFactory
}

// instanceManager is used to manage a single driver plugin
type instanceManager struct {
	// logger is the logger used by the driver instance manager
	logger log.Logger

	// ctx is used to shutdown the driver manager
	ctx context.Context

	// cancel is used to shutdown management of this driver plugin
	cancel context.CancelFunc

	// loader is the plugin loader
	loader loader.PluginCatalog

	// storeReattach is used to store a plugins reattach config
	storeReattach StorePluginReattachFn

	// fetchReattach is used to retrieve a plugin's reattach config
	fetchReattach FetchPluginReattachFn

	// pluginConfig is the config passed to the launched plugins
	pluginConfig *base.AgentConfig

	// id is the ID of the plugin being managed
	id *loader.PluginID

	// plugin is the plugin instance being managed
	plugin loader.PluginInstance

	// driver is the driver plugin being managed
	driver drivers.DriverPlugin

	// pluginLock locks access to the driver and plugin
	pluginLock sync.Mutex

	// shutdownLock is used to serialize attempts to shutdown
	shutdownLock sync.Mutex

	// updateNodeFromDriver is the callback used to update the node from fingerprinting
	updateNodeFromDriver UpdateNodeDriverInfoFn

	// eventHandlerFactory is used to fetch a handler for a task event
	eventHandlerFactory TaskEventHandlerFactory

	// firstFingerprintCh is used to trigger that we have successfully
	// fingerprinted once. It is used to gate launching the stats collection.
	firstFingerprintCh chan struct{}
	hasFingerprinted   bool

	// lastHealthState is the last known health fingerprinted by the manager
	lastHealthState   drivers.HealthState
	lastHealthStateMu sync.Mutex
}

// newInstanceManager returns a new driver instance manager. It is expected that
// the context passed in the configuration is cancelled in order to shutdown
// launched goroutines.
func newInstanceManager(c *instanceManagerConfig) *instanceManager {

	ctx, cancel := context.WithCancel(c.Ctx)
	i := &instanceManager{
		logger:               c.Logger.With("driver", c.ID.Name),
		ctx:                  ctx,
		cancel:               cancel,
		loader:               c.Loader,
		storeReattach:        c.StoreReattach,
		fetchReattach:        c.FetchReattach,
		pluginConfig:         c.PluginConfig,
		id:                   c.ID,
		updateNodeFromDriver: c.UpdateNodeFromDriver,
		eventHandlerFactory:  c.EventHandlerFactory,
		firstFingerprintCh:   make(chan struct{}),
	}

	go i.run()
	return i
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

	// Start event handler
	wg.Add(1)
	go func() {
		i.handleEvents()
		wg.Done()
	}()

	// Do a final cleanup
	wg.Wait()
	i.cleanup()
}

// dispense is used to dispense a plugin.
func (i *instanceManager) dispense() (plugin drivers.DriverPlugin, err error) {
	i.pluginLock.Lock()
	defer i.pluginLock.Unlock()

	// See if we already have a running instance
	if i.plugin != nil && !i.plugin.Exited() {
		return i.driver, nil
	}

	var pluginInstance loader.PluginInstance
	dispenseFn := func() (loader.PluginInstance, error) {
		return i.loader.Dispense(i.id.Name, i.id.PluginType, i.pluginConfig, i.logger)
	}

	if reattach, ok := i.fetchReattach(); ok {
		// Reattach to existing plugin
		pluginInstance, err = i.loader.Reattach(i.id.Name, i.id.PluginType, reattach)

		// If reattachment fails, get a new plugin instance
		if err != nil {
			i.logger.Warn("failed to reattach to plugin, starting new instance", "error", err)
			pluginInstance, err = dispenseFn()
		}
	} else {
		// Get an instance of the plugin
		pluginInstance, err = dispenseFn()
	}

	if err != nil {
		// Retry as the error just indicates the singleton has exited
		if err == singleton.SingletonPluginExited {
			pluginInstance, err = dispenseFn()
		}

		// If we still have an error there is a real problem
		if err != nil {
			return nil, fmt.Errorf("failed to start plugin: %v", err)
		}
	}

	// Convert to a driver plugin
	driver, ok := pluginInstance.Plugin().(drivers.DriverPlugin)
	if !ok {
		pluginInstance.Kill()
		return nil, fmt.Errorf("plugin loaded does not implement the driver interface")
	}

	// Store the plugin and driver
	i.plugin = pluginInstance
	i.driver = driver

	// Store the reattach config
	if c, ok := pluginInstance.ReattachConfig(); ok {
		if err := i.storeReattach(c); err != nil {
			i.logger.Error("error storing driver plugin reattach config", "error", err)
		}
	}

	return driver, nil
}

// cleanup shutsdown the plugin
func (i *instanceManager) cleanup() {
	i.shutdownLock.Lock()
	i.pluginLock.Lock()
	defer i.pluginLock.Unlock()
	defer i.shutdownLock.Unlock()

	if i.plugin == nil {
		return
	}

	if !i.plugin.Exited() {
		i.plugin.Kill()
		if err := i.storeReattach(nil); err != nil {
			i.logger.Warn("error clearing plugin reattach config from state store", "error", err)
		}
	}

	i.cancel()
}

// dispenseFingerprintCh dispenses a driver and makes a Fingerprint RPC call
// to the driver. The fingerprint chan is returned along with the cancel func
// for the context used in the RPC. This cancel func should always be called
// when the caller is finished with the channel.
func (i *instanceManager) dispenseFingerprintCh() (<-chan *drivers.Fingerprint, context.CancelFunc, error) {
	driver, err := i.dispense()
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithCancel(i.ctx)
	fingerCh, err := driver.Fingerprint(ctx)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return fingerCh, cancel, nil
}

// fingerprint is the main loop for fingerprinting.
func (i *instanceManager) fingerprint() {
	fpChan, cancel, err := i.dispenseFingerprintCh()
	if err != nil {
		i.logger.Error("failed to dispense driver plugin", "error", err)
	}

	// backoff and retry used if the RPC is closed by the other end
	var backoff time.Duration
	var retry uint64
	for {
		if backoff > 0 {
			select {
			case <-time.After(backoff):
			case <-i.ctx.Done():
				cancel()
				return
			}
		}

		select {
		case <-i.ctx.Done():
			cancel()
			return
		case fp, ok := <-fpChan:
			if ok {
				if fp.Err == nil {
					i.handleFingerprint(fp)
				} else {
					i.logger.Warn("received fingerprint error from driver", "error", fp.Err)
					i.handleFingerprintError()
				}
				continue
			}

			// avoid fingerprinting again if ctx and fpChan both close
			if i.ctx.Err() != nil {
				cancel()
				return
			}

			// if the channel is closed attempt to open a new one
			newFpChan, newCancel, err := i.dispenseFingerprintCh()
			if err != nil {
				i.logger.Warn("error fingerprinting driver", "error", err, "retry", retry)
				i.handleFingerprintError()

				// Calculate the new backoff
				backoff = helper.Backoff(driverFPBackoffBaseline, driverFPBackoffLimit, retry)
				retry++
				continue
			}
			cancel()
			fpChan = newFpChan
			cancel = newCancel

			// Reset backoff
			backoff = 0
			retry = 0
		}
	}
}

// handleFingerprintError is called when an error occurred while fingerprinting
// and will set the driver to unhealthy
func (i *instanceManager) handleFingerprintError() {
	di := &structs.DriverInfo{
		Healthy:           false,
		HealthDescription: "failed to fingerprint driver",
		UpdateTime:        time.Now(),
	}
	i.updateNodeFromDriver(i.id.Name, di)
}

// handleFingerprint updates the node with the current fingerprint status
func (i *instanceManager) handleFingerprint(fp *drivers.Fingerprint) {
	attrs := make(map[string]string, len(fp.Attributes))
	for key, attr := range fp.Attributes {
		attrs[key] = attr.GoString()
	}
	di := &structs.DriverInfo{
		Attributes:        attrs,
		Detected:          fp.Health != drivers.HealthStateUndetected,
		Healthy:           fp.Health == drivers.HealthStateHealthy,
		HealthDescription: fp.HealthDescription,
		UpdateTime:        time.Now(),
	}
	i.updateNodeFromDriver(i.id.Name, di)

	// log detected/undetected state changes after the initial fingerprint
	i.lastHealthStateMu.Lock()
	if i.hasFingerprinted {
		if i.lastHealthState != fp.Health {
			i.logger.Info("driver health state has changed", "previous", i.lastHealthState, "current", fp.Health, "description", fp.HealthDescription)
		}
	}
	i.lastHealthState = fp.Health
	i.lastHealthStateMu.Unlock()

	// if this is the first fingerprint, mark that we have received it
	if !i.hasFingerprinted {
		i.logger.Debug("initial driver fingerprint", "health", fp.Health, "description", fp.HealthDescription)
		close(i.firstFingerprintCh)
		i.hasFingerprinted = true
	}
}

// getLastHealth returns the most recent HealthState from fingerprinting
func (i *instanceManager) getLastHealth() drivers.HealthState {
	i.lastHealthStateMu.Lock()
	defer i.lastHealthStateMu.Unlock()
	return i.lastHealthState
}

// dispenseTaskEventsCh dispenses a driver plugin and makes a TaskEvents RPC.
// The TaskEvent chan and cancel func for the RPC is return. The cancel func must
// be called by the caller to properly cleanup the context
func (i *instanceManager) dispenseTaskEventsCh() (<-chan *drivers.TaskEvent, context.CancelFunc, error) {
	driver, err := i.dispense()
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithCancel(i.ctx)
	eventsCh, err := driver.TaskEvents(ctx)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return eventsCh, cancel, nil
}

// handleEvents is the main loop that receives task events from the driver
func (i *instanceManager) handleEvents() {
	eventsCh, cancel, err := i.dispenseTaskEventsCh()
	if err != nil {
		i.logger.Error("failed to dispense driver", "error", err)
	}

	var backoff time.Duration
	var retry uint64
	for {
		if backoff > 0 {
			select {
			case <-time.After(backoff):
			case <-i.ctx.Done():
				cancel()
				return
			}
		}

		select {
		case <-i.ctx.Done():
			cancel()
			return
		case ev, ok := <-eventsCh:
			if ok {
				i.handleEvent(ev)
				continue
			}

			// if the channel is closed attempt to open a new one
			newEventsChan, newCancel, err := i.dispenseTaskEventsCh()
			if err != nil {
				i.logger.Warn("failed to receive task events, retrying", "error", err, "retry", retry)

				// Calculate the new backoff
				backoff = helper.Backoff(driverFPBackoffBaseline, driverFPBackoffLimit, retry)
				retry++
				continue
			}
			cancel()
			eventsCh = newEventsChan
			cancel = newCancel

			// Reset backoff
			backoff = 0
			retry = 0
		}
	}
}

// handleEvent looks up the event handler(s) for the event and runs them
func (i *instanceManager) handleEvent(ev *drivers.TaskEvent) {
	// Do not emit that the plugin is shutdown
	if ev.Err != nil && ev.Err == bstructs.ErrPluginShutdown {
		return
	}

	if handler := i.eventHandlerFactory(ev.AllocID, ev.TaskName); handler != nil {
		i.logger.Trace("task event received", "event", ev)
		handler(ev)
		return
	}

	i.logger.Warn("no handler registered for event", "event", ev, "error", ev.Err)

}
