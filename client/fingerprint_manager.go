package client

import (
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

	// UpdateHealthCheck is a callback to the client to update the state of the
	// node for resources that require a health check
	updateHealthCheck func(*cstructs.HealthCheckResponse) *structs.Node
	logger            *log.Logger
}

// NewFingerprintManager is a constructor that creates and returns an instance
// of FingerprintManager
func NewFingerprintManager(getConfig func() *config.Config,
	node *structs.Node,
	shutdownCh chan struct{},
	updateNodeAttributes func(*cstructs.FingerprintResponse) *structs.Node,
	updateHealthCheck func(*cstructs.HealthCheckResponse) *structs.Node,
	logger *log.Logger) *FingerprintManager {
	return &FingerprintManager{
		getConfig:            getConfig,
		updateNodeAttributes: updateNodeAttributes,
		updateHealthCheck:    updateHealthCheck,
		node:                 node,
		shutdownCh:           shutdownCh,
		logger:               logger,
	}
}

// runFingerprint runs each fingerprinter individually on an ongoing basis
func (fm *FingerprintManager) runFingerprint(f fingerprint.Fingerprint, period time.Duration, name string) {
	fm.logger.Printf("[DEBUG] client.fingerprint_manager: fingerprinting %s every %v", name, period)

	for {
		select {
		case <-time.After(period):
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

// runHealthCheck runs each health check individually on an ongoing basis
func (fm *FingerprintManager) runHealthCheck(hc fingerprint.HealthCheck, period time.Duration, name string) {
	fm.logger.Printf("[DEBUG] client.fingerprint_manager: healthchecking %s every %v", name, period)

	for {
		select {
		case <-time.After(period):
			err := fm.healthCheck(name, hc)
			if err != nil {
				fm.logger.Printf("[DEBUG] client.fingerprint_manager: health checking for %v failed: %+v", name, err)
				continue
			}

		case <-fm.shutdownCh:
			return
		}
	}
}

// setupDrivers is used to fingerprint the node to see if these drivers are
// supported
func (fm *FingerprintManager) setupDrivers(drivers []string) error {
	var availDrivers []string
	driverCtx := driver.NewDriverContext("", "", fm.getConfig(), fm.node, fm.logger, nil)
	for _, name := range drivers {

		d, err := driver.NewDriver(name, driverCtx)
		if err != nil {
			return err
		}

		detected, err := fm.fingerprintDriver(name, d)
		if err != nil {
			fm.logger.Printf("[DEBUG] client.fingerprint_manager: fingerprinting for %v failed: %+v", name, err)
			return err
		}

		// log the fingerprinters which have been applied
		if detected {
			availDrivers = append(availDrivers, name)
		}

		p, period := d.Periodic()
		if p {
			go fm.runFingerprint(d, period, name)
		}

		// We should only run the health check task if the driver is detected
		// Note that if the driver is detected later in a periodic health check,
		// this won't automateically trigger the periodic health check.
		if hc, ok := d.(fingerprint.HealthCheck); ok && detected {
			req := &cstructs.HealthCheckIntervalRequest{}
			resp := &cstructs.HealthCheckIntervalResponse{}
			hc.GetHealthCheckInterval(req, resp)
			if resp.Eligible {
				go fm.runHealthCheck(hc, resp.Period, name)
			}
		}
	}

	fm.logger.Printf("[DEBUG] client.fingerprint_manager: detected drivers %v", availDrivers)
	return nil
}

// fingerprintDriver is a temporary solution to move towards DriverInfo and
// away from annotating a node's attributes to demonstrate support for a
// particular driver. Takes the FingerprintResponse and converts it to the
// proper DriverInfo update and then sets the prefix attributes as well
func (fm *FingerprintManager) fingerprintDriver(name string, f fingerprint.Fingerprint) (bool, error) {
	request := &cstructs.FingerprintRequest{Config: fm.getConfig(), Node: fm.node}
	var response cstructs.FingerprintResponse
	if err := f.Fingerprint(request, &response); err != nil {
		return false, err
	}

	// TODO This is a temporary measure, as eventually all drivers will need to
	// support this. Doing this so that we can enable this iteratively and also
	// in a backwards compatible way, where node attributes for drivers will
	// eventually be phased out.
	di := &structs.DriverInfo{
		Attributes: response.Attributes,
		Detected:   response.Detected,
	}
	response.AddDriver(name, di)

	if node := fm.updateNodeAttributes(&response); node != nil {
		fm.nodeLock.Lock()
		fm.node = node
		fm.nodeLock.Unlock()
	}

	if hc, ok := f.(fingerprint.HealthCheck); ok {
		fm.healthCheck(name, hc)
	} else {

		resp := &cstructs.HealthCheckResponse{
			Drivers: map[string]*structs.DriverInfo{
				name: di,
			},
		}

		if node := fm.updateHealthCheck(resp); node != nil {
			fm.nodeLock.Lock()
			fm.node = node
			fm.nodeLock.Unlock()
		}
	}

	return response.Detected, nil
}

// fingerprint does an initial fingerprint of the client. If the fingerprinter
// is meant to be run continuously, a process is launched to perform this
// fingerprint on an ongoing basis in the background.
func (fm *FingerprintManager) fingerprint(name string, f fingerprint.Fingerprint) (bool, error) {
	request := &cstructs.FingerprintRequest{Config: fm.getConfig(), Node: fm.node}
	var response cstructs.FingerprintResponse

	fm.nodeLock.Lock()
	err := f.Fingerprint(request, &response)
	fm.nodeLock.Unlock()

	if err != nil {
		return false, err
	}

	if node := fm.updateNodeAttributes(&response); node != nil {
		fm.nodeLock.Lock()
		fm.node = node
		fm.nodeLock.Unlock()
	}

	return response.Detected, nil
}

// healthcheck checks the health of the specified resource.
func (fm *FingerprintManager) healthCheck(name string, hc fingerprint.HealthCheck) error {
	request := &cstructs.HealthCheckRequest{}
	var response cstructs.HealthCheckResponse
	err := hc.HealthCheck(request, &response)

	if node := fm.updateHealthCheck(&response); node != nil {
		fm.nodeLock.Lock()
		fm.node = node
		fm.nodeLock.Unlock()
	}

	return err
}

// setupFingerprints is used to fingerprint the node to see if these attributes are
// supported
func (fm *FingerprintManager) setupFingerprinters(fingerprints []string) error {
	var appliedFingerprints []string

	for _, name := range fingerprints {
		f, err := fingerprint.NewFingerprint(name, fm.logger)

		if err != nil {
			fm.logger.Printf("[DEBUG] client.fingerprint_manager: fingerprinting for %v failed: %+v", name, err)
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

		if hc, ok := f.(fingerprint.HealthCheck); ok {
			req := &cstructs.HealthCheckIntervalRequest{}
			var resp cstructs.HealthCheckIntervalResponse
			if err := hc.GetHealthCheckInterval(req, &resp); err != nil {
				if resp.Eligible {
					go fm.runHealthCheck(hc, resp.Period, name)
				}
			}
		}
	}

	fm.logger.Printf("[DEBUG] client.fingerprint_manager: detected fingerprints %v", appliedFingerprints)
	return nil
}

// Run starts the process of fingerprinting the node. It does an initial pass,
// identifying whitelisted and blacklisted fingerprints/drivers. Then, for
// those which require periotic checking, it starts a periodic process for
// each.
func (fp *FingerprintManager) Run() error {
	// first, set up all fingerprints
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

	// next, set up drivers
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
