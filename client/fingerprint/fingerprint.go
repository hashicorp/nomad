package fingerprint

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// EmptyDuration is to be used by fingerprinters that are not periodic.
const EmptyDuration = time.Duration(0)

// BuiltinFingerprints is a slice containing the key names of all registered
// fingerprints available, to provided an ordered iteration
var BuiltinFingerprints = []string{
	"arch",
	"cgroup",
	"consul",
	"cpu",
	"env_aws",
	"env_gce",
	"host",
	"memory",
	"network",
	"nomad",
	"storage",
}

// builtinFingerprintMap contains the built in registered fingerprints
// which are available, corresponding to a key found in BuiltinFingerprints
var builtinFingerprintMap = map[string]Factory{
	"arch":    NewArchFingerprint,
	"cgroup":  NewCGroupFingerprint,
	"consul":  NewConsulFingerprint,
	"cpu":     NewCPUFingerprint,
	"env_aws": NewEnvAWSFingerprint,
	"env_gce": NewEnvGCEFingerprint,
	"host":    NewHostFingerprint,
	"memory":  NewMemoryFingerprint,
	"network": NewNetworkFingerprint,
	"nomad":   NewNomadFingerprint,
	"storage": NewStorageFingerprint,
}

// NewFingerprint is used to instantiate and return a new fingerprint
// given the name and a logger
func NewFingerprint(name string, logger *log.Logger) (Fingerprint, error) {
	// Lookup the factory function
	factory, ok := builtinFingerprintMap[name]
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

	// Periodic is a mechanism for the fingerprinter to indicate that it should
	// be run periodically. The return value is a boolean indicating if it
	// should be periodic, and if true, a duration.
	Periodic() (bool, time.Duration)
}

// StaticFingerprinter can be embedded in a struct that has a Fingerprint method
// to make it non-periodic.
type StaticFingerprinter struct{}

func (s *StaticFingerprinter) Periodic() (bool, time.Duration) {
	return false, EmptyDuration
}
