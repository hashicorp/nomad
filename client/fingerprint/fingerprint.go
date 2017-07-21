package fingerprint

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// EmptyDuration is to be used by fingerprinters that are not periodic.
const (
	EmptyDuration = time.Duration(0)

	// TightenNetworkTimeoutsConfig is a config key that can be used during
	// tests to tighten the timeouts for fingerprinters that make network calls.
	TightenNetworkTimeoutsConfig = "test.tighten_network_timeouts"
)

func init() {

	// Initialize the list of available fingerprinters per platform.  Each
	// platform defines its own list of available fingerprinters.
	initPlatformFingerprints(hostFingerprinters)
}

var (
	// hostFingerprinters contains the host fingerprints which are available for a
	// given platform.
	hostFingerprinters = map[string]Factory{
		"arch":    NewArchFingerprint,
		"consul":  NewConsulFingerprint,
		"cpu":     NewCPUFingerprint,
		"host":    NewHostFingerprint,
		"memory":  NewMemoryFingerprint,
		"network": NewNetworkFingerprint,
		"nomad":   NewNomadFingerprint,
		"signal":  NewSignalFingerprint,
		"storage": NewStorageFingerprint,
		"vault":   NewVaultFingerprint,
	}

	// envFingerprinters contains the fingerprints that are environment specific.
	// This should run after the host fingerprinters as they may override specific
	// node resources with more detailed information.
	envFingerprinters = map[string]Factory{
		"env_aws": NewEnvAWSFingerprint,
		"env_gce": NewEnvGCEFingerprint,
	}
)

// BuiltinFingerprints is a slice containing the key names of all registered
// fingerprints available. The order of this slice should be preserved when
// fingerprinting.
func BuiltinFingerprints() []string {
	fingerprints := make([]string, 0, len(hostFingerprinters))
	for k := range hostFingerprinters {
		fingerprints = append(fingerprints, k)
	}
	sort.Strings(fingerprints)
	for k := range envFingerprinters {
		fingerprints = append(fingerprints, k)
	}
	return fingerprints
}

// NewFingerprint is used to instantiate and return a new fingerprint
// given the name and a logger
func NewFingerprint(name string, logger *log.Logger) (Fingerprint, error) {
	// Lookup the factory function
	factory, ok := hostFingerprinters[name]
	if !ok {
		factory, ok = envFingerprinters[name]
		if !ok {
			return nil, fmt.Errorf("unknown fingerprint '%s'", name)
		}
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
