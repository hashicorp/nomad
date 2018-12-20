package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// batchFirstFingerprintsTimeout is the maximum amount of time to wait for
	// initial fingerprinting to complete before sending a batched Node update
	batchFirstFingerprintsTimeout = 5 * time.Second
)

// batchFirstFingerprints waits for the first fingerprint response from all
// plugin managers and sends a single Node update for all fingerprints. It
// should only ever be called once
func (c *Client) batchFirstFingerprints() {
	ctx, cancel := context.WithTimeout(context.Background(), batchFirstFingerprintsTimeout)
	defer cancel()

	ch, err := c.pluginManagers.WaitForFirstFingerprint(ctx)
	if err != nil {
		c.logger.Warn("failed to batch initial fingerprint updates, switching to incemental updates")
		goto SEND_BATCH
	}

	// Wait for fingerprinting to complete or timeout before processing batches
	select {
	case <-ch:
	case <-ctx.Done():
	}

SEND_BATCH:
	c.configLock.Lock()
	defer c.configLock.Unlock()

	// driver node updates
	var driverChanged bool
	c.batchNodeUpdates.batchDriverUpdates(func(driver string, info *structs.DriverInfo) {
		if c.updateNodeFromDriverLocked(driver, info) {
			c.config.Node.Drivers[driver] = info
			if c.config.Node.Drivers[driver].UpdateTime.IsZero() {
				c.config.Node.Drivers[driver].UpdateTime = time.Now()
			}
			driverChanged = true
		}
	})

	// device node updates
	var devicesChanged bool
	c.batchNodeUpdates.batchDevicesUpdates(func(devices []*structs.NodeDeviceResource) {
		if c.updateNodeFromDevicesLocked(devices) {
			devicesChanged = true
		}
	})

	// only update the node if changes occurred
	if driverChanged || devicesChanged {
		c.updateNodeLocked()
	}

	close(c.fpInitialized)
}

// updateNodeFromDriver receives a DriverInfo struct for the driver and updates
// the node accordingly
func (c *Client) updateNodeFromDriver(name string, info *structs.DriverInfo) {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	if c.updateNodeFromDriverLocked(name, info) {
		c.config.Node.Drivers[name] = info
		if c.config.Node.Drivers[name].UpdateTime.IsZero() {
			c.config.Node.Drivers[name].UpdateTime = time.Now()
		}
		c.updateNodeLocked()
	}
}

// updateNodeFromDriverLocked makes the changes to the node from a driver update
// but does not send the update to the server. c.configLock must be held before
// calling this func
func (c *Client) updateNodeFromDriverLocked(name string, info *structs.DriverInfo) bool {
	var hasChanged bool

	hadDriver := c.config.Node.Drivers[name] != nil
	if !hadDriver {
		// If the driver info has not yet been set, do that here
		hasChanged = true
		for attrName, newVal := range info.Attributes {
			c.config.Node.Attributes[attrName] = newVal
		}
	} else {
		oldVal := c.config.Node.Drivers[name]
		// The driver info has already been set, fix it up
		if oldVal.Detected != info.Detected {
			hasChanged = true
		}

		// If health state has change, trigger node event
		if oldVal.Healthy != info.Healthy || oldVal.HealthDescription != info.HealthDescription {
			hasChanged = true
			if info.HealthDescription != "" {
				event := &structs.NodeEvent{
					Subsystem: "Driver",
					Message:   info.HealthDescription,
					Timestamp: time.Now(),
					Details:   map[string]string{"driver": name},
				}
				c.triggerNodeEvent(event)
			}
		}

		for attrName, newVal := range info.Attributes {
			oldVal := c.config.Node.Drivers[name].Attributes[attrName]
			if oldVal == newVal {
				continue
			}

			hasChanged = true
			if newVal == "" {
				delete(c.config.Node.Attributes, attrName)
			} else {
				c.config.Node.Attributes[attrName] = newVal
			}
		}
	}

	// COMPAT Remove in Nomad 0.10
	// We maintain the driver enabled attribute until all drivers expose
	// their attributes as DriverInfo
	driverName := fmt.Sprintf("driver.%s", name)
	if info.Detected {
		c.config.Node.Attributes[driverName] = "1"
	} else {
		delete(c.config.Node.Attributes, driverName)
	}

	return hasChanged
}

// updateNodeFromFingerprint updates the node with the result of
// fingerprinting the node from the diff that was created
func (c *Client) updateNodeFromDevices(devices []*structs.NodeDeviceResource) {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	// Not updating node.Resources: the field is deprecated and includes
	// dispatched task resources and not appropriate for expressing
	// node available device resources
	if c.updateNodeFromDevicesLocked(devices) {
		c.updateNodeLocked()
	}
}

// updateNodeFromDevicesLocked updates the node with the results of devices,
// but does send the update to the server. c.configLock must be held before
// calling this func
func (c *Client) updateNodeFromDevicesLocked(devices []*structs.NodeDeviceResource) bool {
	if !structs.DevicesEquals(c.config.Node.NodeResources.Devices, devices) {
		c.logger.Debug("new devices detected", "devices", len(devices))
		c.config.Node.NodeResources.Devices = devices
		return true
	}

	return false
}

// batchNodeUpdates allows for batching multiple Node updates from fingerprinting.
// Once ready, the batches can be flushed and toggled to stop batching and forward
// all updates to a configured callback to be performed incrementally
type batchNodeUpdates struct {
	// access to driver fields must hold driversMu lock
	drivers        map[string]*structs.DriverInfo
	driversBatched bool
	driverCB       drivermanager.UpdateNodeDriverInfoFn
	driversMu      sync.Mutex

	// access to devices fields must hold devicesMu lock
	devices        []*structs.NodeDeviceResource
	devicesBatched bool
	devicesCB      devicemanager.UpdateNodeDevicesFn
	devicesMu      sync.Mutex
}

func newBatchNodeUpdates(
	driverCB drivermanager.UpdateNodeDriverInfoFn,
	devicesCB devicemanager.UpdateNodeDevicesFn) *batchNodeUpdates {

	return &batchNodeUpdates{
		drivers:   make(map[string]*structs.DriverInfo),
		driverCB:  driverCB,
		devices:   []*structs.NodeDeviceResource{},
		devicesCB: devicesCB,
	}
}

// updateNodeFromDriver implements drivermanager.UpdateNodeDriverInfoFn and is
// used in the driver manager to send driver fingerprints to
func (b *batchNodeUpdates) updateNodeFromDriver(driver string, info *structs.DriverInfo) {
	b.driversMu.Lock()
	defer b.driversMu.Unlock()
	if b.driversBatched {
		b.driverCB(driver, info)
		return
	}

	b.drivers[driver] = info
}

// batchDriverUpdates sends all of the batched driver node updates by calling f
// for each driver batched
func (b *batchNodeUpdates) batchDriverUpdates(f drivermanager.UpdateNodeDriverInfoFn) error {
	b.driversMu.Lock()
	defer b.driversMu.Unlock()
	if b.driversBatched {
		return fmt.Errorf("driver updates already batched")
	}

	b.driversBatched = true
	for driver, info := range b.drivers {
		f(driver, info)
	}
	return nil
}

// updateNodeFromDevices implements devicemanager.UpdateNodeDevicesFn and is
// used in the device manager to send device fingerprints to
func (b *batchNodeUpdates) updateNodeFromDevices(devices []*structs.NodeDeviceResource) {
	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()
	if b.devicesBatched {
		b.devicesCB(devices)
		return
	}

	b.devices = devices
}

// batchDevicesUpdates sends the batched device node updates by calling f with
// the devices
func (b *batchNodeUpdates) batchDevicesUpdates(f devicemanager.UpdateNodeDevicesFn) error {
	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()
	if b.devicesBatched {
		return fmt.Errorf("devices updates already batched")
	}

	b.devicesBatched = true
	f(b.devices)
	return nil
}
