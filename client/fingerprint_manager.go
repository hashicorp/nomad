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

	// updateNode is a callback to the client to update the state of its
	// associated node
	updateNode func(*cstructs.FingerprintResponse) *structs.Node
	logger     *log.Logger
}

// NewFingerprintManager is a constructor that creates and returns an instance
// of FingerprintManager
func NewFingerprintManager(getConfig func() *config.Config,
	node *structs.Node,
	shutdownCh chan struct{},
	updateNode func(*cstructs.FingerprintResponse) *structs.Node,
	logger *log.Logger) *FingerprintManager {
	return &FingerprintManager{
		getConfig:  getConfig,
		updateNode: updateNode,
		node:       node,
		shutdownCh: shutdownCh,
		logger:     logger,
	}
}

// run runs each fingerprinter individually on an ongoing basis
func (fm *FingerprintManager) run(f fingerprint.Fingerprint, period time.Duration, name string) {
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

		detected, err := fm.fingerprint(name, d)
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
			go fm.run(d, period, name)
		}
	}

	fm.logger.Printf("[DEBUG] client.fingerprint_manager: detected drivers %v", availDrivers)
	return nil
}

// fingerprint does an initial fingerprint of the client. If the fingerprinter
// is meant to be run continuously, a process is launched to perform this
// fingerprint on an ongoing basis in the background.
func (fm *FingerprintManager) fingerprint(name string, f fingerprint.Fingerprint) (bool, error) {
	request := &cstructs.FingerprintRequest{Config: fm.getConfig(), Node: fm.node}
	var response cstructs.FingerprintResponse
	if err := f.Fingerprint(request, &response); err != nil {
		return false, err
	}

	fm.nodeLock.Lock()
	if node := fm.updateNode(&response); node != nil {
		fm.node = node
	}
	fm.nodeLock.Unlock()

	return response.Detected, nil
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
			go fm.run(f, period, name)
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
