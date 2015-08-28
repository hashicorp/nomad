package fingerprint

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// BuiltinFingerprints contains the built in registered fingerprints
// which are available
var BuiltinFingerprints = map[string]Factory{
	"arch":    NewArchFingerprint,
	"cpu":     NewCPUFingerprint,
	"host":    NewHostFingerprint,
	"memory":  NewMemoryFingerprint,
	"storage": NewStorageFingerprint,
}

// NewFingerprint is used to instantiate and return a new fingerprint
// given the name and a logger
func NewFingerprint(name string, logger *log.Logger) (Fingerprint, error) {
	// Lookup the factory function
	factory, ok := BuiltinFingerprints[name]
	if !ok {
		return nil, fmt.Errorf("unknown fingerprint '%s'", name)
	}

	// Instantiate the fingerprint
	f := factory(logger)
	return f, nil
}

// Factory is used to instantiate a new Fingerprint
type Factory func(*log.Logger) Fingerprint

// Fingerprint is used for doing "fingerprinting" of the
// host to automatically determine attributes, resources,
// and metadata about it. Each of these is a heuristic, and
// many of them can be applied on a particular host.
type Fingerprint interface {
	// Fingerprint is used to update properties of the Node,
	// and returns if the fingerprint was applicable and a potential error.
	Fingerprint(*config.Config, *structs.Node) (bool, error)
}
