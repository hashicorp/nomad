package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// batchFirstFingerprintsTimeout is the maximum amount of time to wait for
	// initial fingerprinting to complete before sending a batched Node update
	batchFirstFingerprintsTimeout = 50 * time.Second
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

	// csi updates
	var csiChanged bool
	c.batchNodeUpdates.batchCSIUpdates(func(name string, info *structs.CSIInfo) {
		if c.updateNodeFromCSILocked(name, info) {
			c.config.Node.CSIControllerPlugins[name] = info
			if c.config.Node.CSIControllerPlugins[name].UpdateTime.IsZero() {
				c.config.Node.CSIControllerPlugins[name].UpdateTime = time.Now()
			}
			c.config.Node.CSINodePlugins[name] = info
			if c.config.Node.CSINodePlugins[name].UpdateTime.IsZero() {
				c.config.Node.CSINodePlugins[name].UpdateTime = time.Now()
			}
			csiChanged = true
		}
	})

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
	if driverChanged || devicesChanged || csiChanged {
		c.updateNodeLocked()
	}

	close(c.fpInitialized)
}

// updateNodeFromCSI receives a CSIInfo struct for the plugin and updates the
// node accordingly
func (c *Client) updateNodeFromCSI(name string, info *structs.CSIInfo) {
	c.configLock.Lock()
	defer c.configLock.Unlock()

	if c.updateNodeFromCSILocked(name, info) {
		if info.UpdateTime.IsZero() {
			info.UpdateTime = time.Now()
		}

		c.config.Node.CSIControllerPlugins[name] = info.Copy()
		c.config.Node.CSINodePlugins[name] = info.Copy()
		c.updateNodeLocked()
	}
}

// updateNodeFromCSILocked makes the changes to the node from a csi update
// but does not send the update to the server. c.configLock must be held before
// calling this func
func (c *Client) updateNodeFromCSILocked(name string, info *structs.CSIInfo) bool {
	var changed bool

	oldController, hadController := c.config.Node.CSIControllerPlugins[name]
	if !hadController {
		// If the controller info has not yet been set, do that here
		changed = true
		c.config.Node.CSIControllerPlugins[name] = info
	} else {
		// The controller info has already been set, fix it up
		if !oldController.IsEqual(info) {
			c.config.Node.CSIControllerPlugins[name] = info
			changed = true
		}

		// If health state has changed, trigger node event
		if oldController.Healthy != info.Healthy || oldController.HealthDescription != info.HealthDescription {
			changed = true
			if info.HealthDescription != "" {
				event := &structs.NodeEvent{
					Subsystem: "CSI",
					Message:   info.HealthDescription,
					Timestamp: time.Now(),
					Details:   map[string]string{"plugin": name},
				}
				c.triggerNodeEvent(event)
			}
		}
	}

	oldNode, hadNode := c.config.Node.CSINodePlugins[name]
	if !hadNode {
		// If the Node info has not yet been set, do that here
		changed = true
		c.config.Node.CSINodePlugins[name] = info
	} else {
		// The node info has already been set, fix it up
		if !oldNode.IsEqual(info) {
			c.config.Node.CSINodePlugins[name] = info
			changed = true
		}

		// If health state has changed, trigger node event
		if oldNode.Healthy != info.Healthy || oldNode.HealthDescription != info.HealthDescription {
			changed = true
			if info.HealthDescription != "" {
				event := &structs.NodeEvent{
					Subsystem: "CSI",
					Message:   info.HealthDescription,
					Timestamp: time.Now(),
					Details:   map[string]string{"plugin": name},
				}
				c.triggerNodeEvent(event)
			}
		}
	}

	return changed
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

	// access to csi fields must hold csiMu lock
	csiNodePlugins       map[string]*structs.CSIInfo
	csiControllerPlugins map[string]*structs.CSIInfo
	csiBatched           bool
	csiCB                csimanager.UpdateNodeCSIInfoFunc
	csiMu                sync.Mutex
}

func newBatchNodeUpdates(
	driverCB drivermanager.UpdateNodeDriverInfoFn,
	devicesCB devicemanager.UpdateNodeDevicesFn,
	csiCB csimanager.UpdateNodeCSIInfoFunc) *batchNodeUpdates {

	return &batchNodeUpdates{
		drivers:              make(map[string]*structs.DriverInfo),
		driverCB:             driverCB,
		devices:              []*structs.NodeDeviceResource{},
		devicesCB:            devicesCB,
		csiNodePlugins:       make(map[string]*structs.CSIInfo),
		csiControllerPlugins: make(map[string]*structs.CSIInfo),
		csiCB:                csiCB,
	}
}

// updateNodeFromCSI implements csimanager.UpdateNodeCSIInfoFunc and is used in
// the csi manager to send csi fingerprints to the server. Currently it registers
// all plugins as both controller and node plugins.
// TODO: separate node and controller plugin handling.
func (b *batchNodeUpdates) updateNodeFromCSI(plugin string, info *structs.CSIInfo) {
	b.csiMu.Lock()
	defer b.csiMu.Unlock()
	if b.csiBatched {
		b.csiCB(plugin, info)
		return
	}

	b.csiNodePlugins[plugin] = info
	b.csiControllerPlugins[plugin] = info
}

// batchCSIUpdates sends all of the batched CSI updates by calling f  for each
// plugin batched
func (b *batchNodeUpdates) batchCSIUpdates(f csimanager.UpdateNodeCSIInfoFunc) error {
	b.csiMu.Lock()
	defer b.csiMu.Unlock()
	if b.csiBatched {
		return fmt.Errorf("csi updates already batched")
	}

	b.csiBatched = true
	for plugin, info := range b.csiNodePlugins {
		f(plugin, info)
	}
	for plugin, info := range b.csiControllerPlugins {
		f(plugin, info)
	}
	return nil
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
