package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/loader"
)

const (
	// driverFPBackoffBaseline is the baseline time for exponential backoff while
	// fingerprinting a driver.
	driverFPBackoffBaseline = 5 * time.Second

	// driverFPBackoffLimit is the limit of the exponential backoff for fingerprinting
	// a driver.
	driverFPBackoffLimit = 2 * time.Minute
)

// FingerprintManager runs a client fingerprinters on a continuous basis, and
// updates the client when the node has changed
type FingerprintManager struct {
	singletonLoader loader.PluginCatalog
	getConfig       func() *config.Config
	node            *structs.Node
	nodeLock        sync.Mutex
	shutdownCh      chan struct{}

	// updateNodeAttributes is a callback to the client to update the state of its
	// associated node
	updateNodeAttributes func(*cstructs.FingerprintResponse) *structs.Node

	// updateNodeFromDriver is a callback to the client to update the state of a
	// specific driver for the node
	updateNodeFromDriver func(string, *structs.DriverInfo) *structs.Node
	logger               log.Logger
}

// NewFingerprintManager is a constructor that creates and returns an instance
// of FingerprintManager
func NewFingerprintManager(
	singletonLoader loader.PluginCatalog,
	getConfig func() *config.Config,
	node *structs.Node,
	shutdownCh chan struct{},
	updateNodeAttributes func(*cstructs.FingerprintResponse) *structs.Node,
	updateNodeFromDriver func(string, *structs.DriverInfo) *structs.Node,
	logger log.Logger) *FingerprintManager {

	return &FingerprintManager{
		singletonLoader:      singletonLoader,
		getConfig:            getConfig,
		updateNodeAttributes: updateNodeAttributes,
		updateNodeFromDriver: updateNodeFromDriver,
		node:                 node,
		shutdownCh:           shutdownCh,
		logger:               logger.Named("fingerprint_mgr"),
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

	fp.logger.Debug("built-in fingerprints", "fingerprinters", fingerprint.BuiltinFingerprints())

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
		fp.logger.Debug("fingerprint modules skipped due to white/blacklist",
			"skipped_fingerprinters", skippedFingerprints)
	}

	// Next, set up drivers
	// Build the white/blacklists of drivers.
	whitelistDrivers := cfg.ReadStringListToMap("driver.whitelist")
	whitelistDriversEnabled := len(whitelistDrivers) > 0
	blacklistDrivers := cfg.ReadStringListToMap("driver.blacklist")

	var availDrivers []string
	var skippedDrivers []string

	for _, pl := range fp.singletonLoader.Catalog()[base.PluginTypeDriver] {
		name := pl.Name
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
		fp.logger.Debug("drivers skipped due to white/blacklist", "skipped_drivers", skippedDrivers)
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
			fm.logger.Error("error fingerprinting", "error", err, "fingerprinter", name)
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

	fm.logger.Debug("detected fingerprints", "node_attrs", appliedFingerprints)
	return nil
}

// setupDrivers is used to fingerprint the node to see if these drivers are
// supported
func (fm *FingerprintManager) setupDrivers(driverNames []string) error {
	//TODO(alex,hclog) Update fingerprinters to hclog
	var availDrivers []string
	for _, name := range driverNames {
		// TODO: driver reattach
		fingerCh, cancel, err := fm.dispenseDriverFingerprint(name)
		if err != nil {
			return err
		}

		finger := <-fingerCh

		// Start a periodic watcher to detect changes to a drivers health and
		// attributes.
		go fm.watchDriverFingerprint(fingerCh, name, cancel)

		if fm.logger.IsTrace() {
			fm.logger.Trace("initial driver fingerprint", "driver", name, "fingerprint", finger)
		}
		// Log the fingerprinters which have been applied
		if finger.Health != drivers.HealthStateUndetected {
			availDrivers = append(availDrivers, name)
		}
		fm.processDriverFingerprint(finger, name)
	}

	fm.logger.Debug("detected drivers", "drivers", availDrivers)
	return nil
}

// runFingerprint runs each fingerprinter individually on an ongoing basis
func (fm *FingerprintManager) runFingerprint(f fingerprint.Fingerprint, period time.Duration, name string) {
	fm.logger.Debug("fingerprinting periodically", "fingerprinter", name, "period", period)

	timer := time.NewTimer(period)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			timer.Reset(period)

			_, err := fm.fingerprint(name, f)
			if err != nil {
				fm.logger.Debug("error periodic fingerprinting", "error", err, "fingerprinter", name)
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
func (fm *FingerprintManager) watchDriverFingerprint(fpChan <-chan *drivers.Fingerprint, name string, cancel context.CancelFunc) {
	var backoff time.Duration
	var retry int
	for {
		if backoff > 0 {
			time.Sleep(backoff)
		}
		select {
		case <-fm.shutdownCh:
			cancel()
			return
		case fp, ok := <-fpChan:
			if ok && fp.Err == nil {
				fm.processDriverFingerprint(fp, name)
				continue
			}
			// if the channel is closed attempt to open a new one
			newFpChan, newCancel, err := fm.dispenseDriverFingerprint(name)
			if err != nil {
				fm.logger.Warn("failed to fingerprint driver, retrying in 30s", "error", err, "retry", retry)
				di := &structs.DriverInfo{
					Healthy:           false,
					HealthDescription: "failed to fingerprint driver",
					UpdateTime:        time.Now(),
				}
				if n := fm.updateNodeFromDriver(name, di); n != nil {
					fm.setNode(n)
				}

				// Calculate the new backoff
				backoff = (1 << (2 * uint64(retry))) * driverFPBackoffBaseline
				if backoff > driverFPBackoffLimit {
					backoff = driverFPBackoffLimit
				}
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

// processDriverFringerprint converts a Fingerprint from a driver into a DriverInfo
// struct and updates the Node with it
func (fm *FingerprintManager) processDriverFingerprint(fp *drivers.Fingerprint, driverName string) {
	di := &structs.DriverInfo{
		Attributes:        fp.Attributes,
		Detected:          fp.Health != drivers.HealthStateUndetected,
		Healthy:           fp.Health == drivers.HealthStateHealthy,
		HealthDescription: fp.HealthDescription,
		UpdateTime:        time.Now(),
	}
	if n := fm.updateNodeFromDriver(driverName, di); n != nil {
		fm.setNode(n)
	}
}

// dispenseDriverFingerprint dispenses a driver plugin for the given driver name
// and requests a fingerprint channel. The channel and a context cancel function
// is returned to the caller
func (fm *FingerprintManager) dispenseDriverFingerprint(driverName string) (<-chan *drivers.Fingerprint, context.CancelFunc, error) {
	plug, err := fm.singletonLoader.Dispense(driverName, base.PluginTypeDriver, fm.getConfig().NomadPluginConfig(), fm.logger)
	if err != nil {
		return nil, nil, err
	}

	driver, ok := plug.Plugin().(drivers.DriverPlugin)
	if !ok {
		return nil, nil, fmt.Errorf("registered driver plugin %q does not implement DriverPlugin interface", driverName)
	}

	ctx, cancel := context.WithCancel(context.Background())
	fingerCh, err := driver.Fingerprint(ctx)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return fingerCh, cancel, nil
}
