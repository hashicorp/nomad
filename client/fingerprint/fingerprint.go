// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"sort"
	"time"

	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
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
		"arch":        NewArchFingerprint,
		"consul":      NewConsulFingerprint,
		"cni":         NewCNIFingerprint, // networks
		"cpu":         NewCPUFingerprint,
		"host":        NewHostFingerprint,
		"landlock":    NewLandlockFingerprint,
		"memory":      NewMemoryFingerprint,
		"network":     NewNetworkFingerprint,
		"nomad":       NewNomadFingerprint,
		"plugins_cni": NewPluginsCNIFingerprint,
		"signal":      NewSignalFingerprint,
		"storage":     NewStorageFingerprint,
		"vault":       NewVaultFingerprint,
	}

	// envFingerprinters contains the fingerprints that are environment specific.
	// This should run after the host fingerprinters as they may override specific
	// node resources with more detailed information.
	envFingerprinters = map[string]Factory{
		"env_aws":          NewEnvAWSFingerprint,
		"env_gce":          NewEnvGCEFingerprint,
		"env_azure":        NewEnvAzureFingerprint,
		"env_digitalocean": NewEnvDigitalOceanFingerprint,
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
func NewFingerprint(name string, logger log.Logger) (Fingerprint, error) {
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
type Factory func(log.Logger) Fingerprint

// HealthCheck is used for doing periodic health checks. On a given time
// interfal, a health check will be called by the fingerprint manager of the
// node.
type HealthCheck interface {
	// Check is used to update properties of the node on the status of the health
	// check
	HealthCheck(*cstructs.HealthCheckRequest, *cstructs.HealthCheckResponse) error

	// GetHealthCheckInterval is a mechanism for the health checker to indicate that
	// it should be run periodically. The return value is a boolean indicating
	// whether it should be done periodically, and the time interval at which
	// this check should happen.
	GetHealthCheckInterval(*cstructs.HealthCheckIntervalRequest, *cstructs.HealthCheckIntervalResponse) error
}

// Fingerprint is used for doing "fingerprinting" of the
// host to automatically determine attributes, resources,
// and metadata about it. Each of these is a heuristic, and
// many of them can be applied on a particular host.
type Fingerprint interface {
	// Fingerprint is used to update properties of the Node,
	// and returns a diff of updated node attributes and a potential error.
	Fingerprint(*FingerprintRequest, *FingerprintResponse) error

	// Periodic is a mechanism for the fingerprinter to indicate that it should
	// be run periodically. The return value is a boolean indicating if it
	// should be periodic, and if true, a duration.
	Periodic() (bool, time.Duration)
}

// ReloadableFingerprint can be implemented if the fingerprinter needs to be run during client reload.
// If implemented, the client will call Reload during client reload then immediately Fingerprint
type ReloadableFingerprint interface {
	Fingerprint
	Reload()
}

// StaticFingerprinter can be embedded in a struct that has a Fingerprint method
// to make it non-periodic.
type StaticFingerprinter struct{}

func (s *StaticFingerprinter) Periodic() (bool, time.Duration) {
	return false, EmptyDuration
}
