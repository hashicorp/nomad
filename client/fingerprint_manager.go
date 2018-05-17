package client

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// FingerprintManager runs a client fingerprinters on a continuous basis, and
// updates the client when the node has changed
type FingerprintManager struct {
	getConfig  func() *config.Config
	node       *structs.Node
	nodeLock   sync.Mutex
	shutdownCh chan struct{}

	// updateNodeAttributes is a callback to the client to update the state of its
	// associated node
	updateNodeAttributes func(*cstructs.FingerprintResponse) *structs.Node

	// updateNodeFromDriver is a callback to the client to update the state of a
	// specific driver for the node
	updateNodeFromDriver func(string, *structs.DriverInfo, *structs.DriverInfo) *structs.Node
	logger               *log.Logger
}

// NewFingerprintManager is a constructor that creates and returns an instance
// of FingerprintManager
func NewFingerprintManager(getConfig func() *config.Config,
	node *structs.Node,
	shutdownCh chan struct{},
	updateNodeAttributes func(*cstructs.FingerprintResponse) *structs.Node,
	updateNodeFromDriver func(string, *structs.DriverInfo, *structs.DriverInfo) *structs.Node,
	logger *log.Logger) *FingerprintManager {
	return &FingerprintManager{
		getConfig:            getConfig,
		updateNodeAttributes: updateNodeAttributes,
		updateNodeFromDriver: updateNodeFromDriver,
		node:                 node,
		shutdownCh:           shutdownCh,
		logger:               logger,
	}
}

// setNode updates the current client node
func (fm *FingerprintManager) setNode(node *structs.Node) {
	fm.nodeLock.Lock()
	defer fm.nodeLock.Unlock()
	fm.node = node
}

// getNode returns the current client node
func (fm *FingerprintManager) getNode() *structs.Node {
	fm.nodeLock.Lock()
	defer fm.nodeLock.Unlock()
	return fm.node
}

// Run starts the process of fingerprinting the node. It does an initial pass,
// identifying whitelisted and blacklisted fingerprints/drivers. Then, for
// those which require periotic checking, it starts a periodic process for
// each.
func (fp *FingerprintManager) Run() error {
	// First, set up all fingerprints
	cfg := fp.getConfig()
	whitelistFingerprints := cfg.ReadStringListToMap("fingerprint.whitelist")
	whitelistFingerprintsEnabled := len(whitelistFingerprints) > 0
	blacklistFingerprints := cfg.ReadStringListToMap("fingerprint.blacklist")

	fp.logger.Printf("[DEBUG] client.fingerprint_manager: built-in fingerprints: %v", fingerprint.BuiltinFingerprints())

	var availableFingerprints []string
	var skippedFingerprints []string
	for _, name := range fingerprint.BuiltinFingerprints() {
		// Skip modules that are not in the whitelist if it is enabled.
		if _, ok := whitelistFingerprints[name]; whitelistFingerprintsEnabled && !ok {
			skippedFingerprints = append(skippedFingerprints, name)
			continue
		}
		// Skip modules that are in the blacklist
		if _, ok := blacklistFingerprints[name]; ok {
			skippedFingerprints = append(skippedFingerprints, name)
			continue
		}

		availableFingerprints = append(availableFingerprints, name)
	}

	if err := fp.setupFingerprinters(availableFingerprints); err != nil {
		return err
	}

	if len(skippedFingerprints) != 0 {
		fp.logger.Printf("[DEBUG] client.fingerprint_manager: fingerprint modules skipped due to white/blacklist: %v", skippedFingerprints)
	}

	// Next, set up drivers
	// Build the white/blacklists of drivers.
	whitelistDrivers := cfg.ReadStringListToMap("driver.whitelist")
	whitelistDriversEnabled := len(whitelistDrivers) > 0
	blacklistDrivers := cfg.ReadStringListToMap("driver.blacklist")

	var availDrivers []string
	var skippedDrivers []string

	for name := range driver.BuiltinDrivers {
		// Skip fingerprinting drivers that are not in the whitelist if it is
		// enabled.
		if _, ok := whitelistDrivers[name]; whitelistDriversEnabled && !ok {
			skippedDrivers = append(skippedDrivers, name)
			continue
		}
		// Skip fingerprinting drivers that are in the blacklist
		if _, ok := blacklistDrivers[name]; ok {
			skippedDrivers = append(skippedDrivers, name)
			continue
		}

		availDrivers = append(availDrivers, name)
	}

	if err := fp.setupDrivers(availDrivers); err != nil {
		return err
	}

	if len(skippedDrivers) > 0 {
		fp.logger.Printf("[DEBUG] client.fingerprint_manager: drivers skipped due to white/blacklist: %v", skippedDrivers)
	}
	return nil
}

// setupFingerprints is used to fingerprint the node to see if these attributes are
// supported
func (fm *FingerprintManager) setupFingerprinters(fingerprints []string) error {
	var appliedFingerprints []string

	for _, name := range fingerprints {
		f, err := fingerprint.NewFingerprint(name, fm.logger)

		if err != nil {
			fm.logger.Printf("[ERR] client.fingerprint_manager: fingerprinting for %v failed: %+v", name, err)
			return err
		}

		detected, err := fm.fingerprint(name, f)
		if err != nil {
			return err
		}

		// log the fingerprinters which have been applied
		if detected {
			appliedFingerprints = append(appliedFingerprints, name)
		}

		p, period := f.Periodic()
		if p {
			go fm.runFingerprint(f, period, name)
		}
	}

	fm.logger.Printf("[DEBUG] client.fingerprint_manager: detected fingerprints %v", appliedFingerprints)
	return nil
}

// setupDrivers is used to fingerprint the node to see if these drivers are
// supported
func (fm *FingerprintManager) setupDrivers(drivers []string) error {
	var availDrivers []string
	driverCtx := driver.NewDriverContext("", "", "", "", fm.getConfig(), fm.getNode(), fm.logger, nil)
	for _, name := range drivers {

		d, err := driver.NewDriver(name, driverCtx)
		if err != nil {
			return err
		}

		// Pass true for whether the health check is periodic here, so that the
		// fingerprinter will not set the initial health check status (this is set
		// below, with an empty health status so that a node event is not
		// triggered)
		// Later, the periodic health checker will update this value for drivers
		// where health checks are enabled.
		detected, err := fm.fingerprintDriver(name, d, true)
		if err != nil {
			fm.logger.Printf("[DEBUG] client.fingerprint_manager: fingerprinting driver %v failed: %+v", name, err)
			return err
		}

		// Start a periodic watcher to detect changes to a drivers health and
		// attributes.
		go fm.watchDriver(d, name)

		// Log the fingerprinters which have been applied
		if detected {
			availDrivers = append(availDrivers, name)
		}
	}

	fm.logger.Printf("[DEBUG] client.fingerprint_manager: detected drivers %v", availDrivers)
	return nil
}

// runFingerprint runs each fingerprinter individually on an ongoing basis
func (fm *FingerprintManager) runFingerprint(f fingerprint.Fingerprint, period time.Duration, name string) {
	fm.logger.Printf("[DEBUG] client.fingerprint_manager: fingerprinting %s every %v", name, period)

	timer := time.NewTimer(period)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			timer.Reset(period)

			_, err := fm.fingerprint(name, f)
			if err != nil {
				fm.logger.Printf("[DEBUG] client.fingerprint_manager: periodic fingerprinting for %v failed: %+v", name, err)
				continue
			}

		case <-fm.shutdownCh:
			return
		}
	}
}

// fingerprint does an initial fingerprint of the client. If the fingerprinter
// is meant to be run continuously, a process is launched to perform this
// fingerprint on an ongoing basis in the background.
func (fm *FingerprintManager) fingerprint(name string, f fingerprint.Fingerprint) (bool, error) {
	var response cstructs.FingerprintResponse

	fm.nodeLock.Lock()
	request := &cstructs.FingerprintRequest{Config: fm.getConfig(), Node: fm.node}
	err := f.Fingerprint(request, &response)
	fm.nodeLock.Unlock()

	if err != nil {
		return false, err
	}

	if node := fm.updateNodeAttributes(&response); node != nil {
		fm.setNode(node)
	}

	return response.Detected, nil
}

// watchDrivers facilitates the different periods between fingerprint and
// health checking a driver
func (fm *FingerprintManager) watchDriver(d driver.Driver, name string) {
	var fingerprintTicker, healthTicker <-chan time.Time

	// Determine whether the fingerprinter is periodic and health checking
	isPeriodic, fingerprintPeriod := d.Periodic()
	hc, isHealthCheck := d.(fingerprint.HealthCheck)

	// Nothing to do since the state of this driver will never change
	if !isPeriodic && !isHealthCheck {
		return
	}

	// Setup the required tickers
	if isPeriodic {
		ticker := time.NewTicker(fingerprintPeriod)
		fingerprintTicker = ticker.C
		defer ticker.Stop()
		fm.logger.Printf("[DEBUG] client.fingerprint_manager: fingerprinting driver %s every %v", name, fingerprintPeriod)
	}

	var isHealthCheckPeriodic bool
	if isHealthCheck {
		// Determine the interval at which to health check
		req := &cstructs.HealthCheckIntervalRequest{}
		var healthCheckResp cstructs.HealthCheckIntervalResponse

		if err := hc.GetHealthCheckInterval(req, &healthCheckResp); err != nil {
			fm.logger.Printf("[ERR] client.fingerprint_manager: error getting health check interval for driver %s: %v", name, err)
		} else if healthCheckResp.Eligible {
			isHealthCheckPeriodic = true
			ticker := time.NewTicker(healthCheckResp.Period)
			healthTicker = ticker.C
			defer ticker.Stop()
			fm.logger.Printf("[DEBUG] client.fingerprint_manager: health checking driver %s every %v", name, healthCheckResp.Period)
		}
	}

	driverEverDetected := false
	for {
		select {
		case <-fm.shutdownCh:
			return
		case <-fingerprintTicker:
			if _, err := fm.fingerprintDriver(name, d, isHealthCheckPeriodic); err != nil {
				fm.logger.Printf("[DEBUG] client.fingerprint_manager: periodic fingerprinting for driver %v failed: %+v", name, err)
			}

			fm.nodeLock.Lock()
			driver, detected := fm.node.Drivers[name]

			// Memoize the driver detected status, so that we know whether to run the
			// health check or not.
			if detected && driver != nil && driver.Detected {
				if !driverEverDetected {
					driverEverDetected = true
				}
			}
			fm.nodeLock.Unlock()
		case <-healthTicker:
			if driverEverDetected {
				if err := fm.runDriverHealthCheck(name, hc); err != nil {
					fm.logger.Printf("[DEBUG] client.fingerprint_manager: health checking for %v failed: %v", name, err)
				}
			}
		}
	}
}

// fingerprintDriver is a temporary solution to move towards DriverInfo and
// away from annotating a node's attributes to demonstrate support for a
// particular driver. Takes the FingerprintResponse and converts it to the
// proper DriverInfo update and then sets the prefix attributes as well
func (fm *FingerprintManager) fingerprintDriver(name string, f fingerprint.Fingerprint, hasPeriodicHealthCheck bool) (bool, error) {
	var response cstructs.FingerprintResponse

	fm.nodeLock.Lock()

	// Determine if the driver has been detected before.
	originalNode, haveDriver := fm.node.Drivers[name]
	firstDetection := !haveDriver

	// Determine if the driver is healthy
	var driverIsHealthy bool
	if haveDriver && originalNode.Healthy {
		driverIsHealthy = true
	}

	// Fingerprint the driver.
	request := &cstructs.FingerprintRequest{Config: fm.getConfig(), Node: fm.node}
	err := f.Fingerprint(request, &response)
	fm.nodeLock.Unlock()

	if err != nil {
		return false, err
	}

	// Remove the health check attribute indicating the status of the driver,
	// as the overall driver info object should indicate this.
	delete(response.Attributes, fmt.Sprintf("driver.%s", name))

	fingerprintInfo := &structs.DriverInfo{
		Attributes: response.Attributes,
		Detected:   response.Detected,
	}

	// We set the health status based on the detection state of the driver if:
	// * It is the first time we are fingerprinting the driver. This gives all
	// drivers an initial health.
	// * If the driver becomes undetected. This gives us an immediate unhealthy
	// state and description when it transistions from detected and healthy to
	// undetected.
	// * If the driver does not have its own health checks. Then we always
	// couple the states.
	var healthInfo *structs.DriverInfo
	if firstDetection || !hasPeriodicHealthCheck || !response.Detected && driverIsHealthy {
		state := " "
		if !response.Detected {
			state = " not "
		}

		healthInfo = &structs.DriverInfo{
			Healthy:           response.Detected,
			HealthDescription: fmt.Sprintf("Driver %s is%sdetected", name, state),
			UpdateTime:        time.Now(),
		}
	}

	if node := fm.updateNodeFromDriver(name, fingerprintInfo, healthInfo); node != nil {
		fm.setNode(node)
	}

	return response.Detected, nil
}

// runDriverHealthCheck checks the health of the specified resource.
func (fm *FingerprintManager) runDriverHealthCheck(name string, hc fingerprint.HealthCheck) error {
	request := &cstructs.HealthCheckRequest{}
	var response cstructs.HealthCheckResponse
	if err := hc.HealthCheck(request, &response); err != nil {
		return err
	}

	// Update the status of the node irregardless if there was an error- in the
	// case of periodic health checks, an error will occur if a health check
	// fails
	if node := fm.updateNodeFromDriver(name, nil, response.Drivers[name]); node != nil {
		fm.setNode(node)
	}

	return nil
}
