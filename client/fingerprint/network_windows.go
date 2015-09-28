// +build windows

package fingerprint

import (
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NetworkFingerprint is used to fingerprint the Network capabilities of a node
type NetworkFingerprint struct {
	logger *log.Logger
}

// NewNetworkFingerprint returns a new NetworkFingerprinter with the given
// logger
func NewNetworkFingerprinter(logger *log.Logger) Fingerprint {
	f := &NetworkFingerprint{logger: logger}
	return f
}

func (f *NetworkFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// return false, because we don't yet support Windows
	return false, nil
}
