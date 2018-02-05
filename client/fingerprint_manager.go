package client

import (
	"log"
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
	shutdownCh chan struct{}

	// updateNode is a callback to the client to update the state of its
	// associated node
	updateNode func(*cstructs.FingerprintResponse)
	logger     *log.Logger
}

// run runs each fingerprinter individually on an ongoing basis
func (fm *FingerprintManager) run(f fingerprint.Fingerprint, period time.Duration, name string) {
	fm.logger.Printf("[DEBUG] fingerprint_manager: fingerprinting %s every %v", name, period)

	for {
		select {
		case <-time.After(period):
			_, err := fm.fingerprint(name, f)
			if err != nil {
				fm.logger.Printf("[DEBUG] fingerprint_manager: periodic fingerprinting for %v failed: %+v", name, err)
				continue
			}

		case <-fm.shutdownCh:
			return
		}
	}
}

// setupDrivers is used to fingerprint the node to see if these drivers are
// supported
func (fm *FingerprintManager) SetupDrivers(drivers []string) error {
	var availDrivers []string
	driverCtx := driver.NewDriverContext("", "", fm.getConfig(), fm.node, fm.logger, nil)
	for _, name := range drivers {

		d, err := driver.NewDriver(name, driverCtx)
		if err != nil {
			return err
		}

		detected, err := fm.fingerprint(name, d)
		if err != nil {
			fm.logger.Printf("[DEBUG] fingerprint_manager: fingerprinting for %v failed: %+v", name, err)
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

	fm.logger.Printf("[DEBUG] fingerprint_manager: available drivers %v", availDrivers)

	return nil
}

// fingerprint does an initial fingerprint of the client. If the fingerprinter
// is meant to be run continuously, a process is launched to perform this
// fingerprint on an ongoing basis in the background.
func (fm *FingerprintManager) fingerprint(name string, f fingerprint.Fingerprint) (bool, error) {
	request := &cstructs.FingerprintRequest{Config: fm.getConfig(), Node: fm.node}
	var response cstructs.FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		return false, err
	}

	fm.updateNode(&response)
	return response.Detected, nil
}

// setupFingerprints is used to fingerprint the node to see if these attributes are
// supported
func (fm *FingerprintManager) SetupFingerprinters(fingerprints []string) error {
	var appliedFingerprints []string

	for _, name := range fingerprints {
		f, err := fingerprint.NewFingerprint(name, fm.logger)

		if err != nil {
			fm.logger.Printf("[DEBUG] fingerprint_manager: fingerprinting for %v failed: %+v", name, err)
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

	fm.logger.Printf("[DEBUG] fingerprint_manager: applied fingerprints %v", appliedFingerprints)
	return nil
}
