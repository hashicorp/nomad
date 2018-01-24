package client

import (
	"log"
	"time"

	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// DriverManager runs a client fingerprinters on a continuous basis, and
// updates the client when the node has changed
type DriverManager struct {
	fingerprinters map[string]fingerprint.Fingerprint
	client         *Client
	logger         *log.Logger
}

// Fingerprint starts the ongoing process of running a client's fingerprinters
// and continuously updating its attributes
func (dm *DriverManager) Fingerprint() {
	for name, f := range dm.fingerprinters {
		go dm.runFingerprint(f, name)
	}
}

// runFingerprint is an  interfal function which runs each fingerprinter
// individually on an ongoing basis
func (dm *DriverManager) runFingerprint(f fingerprint.Fingerprint, name string) {
	_, period := f.Periodic()
	dm.logger.Printf("[DEBUG] driver_manager: fingerprinting %s every %v", name, period)

	for {
		select {
		case <-time.After(period):
			request := &cstructs.FingerprintRequest{Config: dm.client.config, Node: dm.client.config.Node}
			response := &cstructs.FingerprintResponse{
				Attributes: make(map[string]string, 0),
				Links:      make(map[string]string, 0),
				Resources:  &structs.Resources{},
			}

			err := f.Fingerprint(request, response)
			if err != nil {
				dm.logger.Printf("[DEBUG] driver_manager: periodic fingerprinting for %v failed: %+v", name, err)
				continue
			}

			dm.client.updateNodeFromFingerprint(response)

		case <-dm.client.shutdownCh:
			return
		}
	}
}
