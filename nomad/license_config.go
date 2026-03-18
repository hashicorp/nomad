// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"slices"
	"time"
)

// LicenseConfig allows for tunable licensing config
// primarily used for enterprise testing
type LicenseConfig struct {
	// BuildDate is the time of the git commit used to build the program.
	BuildDate time.Time

	// NonProduction is a config value passed to the license watcher
	NonProduction bool

	// Edition is scaffolding for a config value that could be
	// passed to the license watcher
	Edition string

	// AddOn  is scaffolding for a config value that could be
	// passed to the license watcher
	AddOn string

	// LicenseEnvBytes is the license bytes to use for the server's license
	LicenseEnvBytes string

	// LicensePath is the path to use for the server's license
	LicensePath string

	// AdditionalPubKeys is a set of public keys for testing
	AdditionalPubKeys []string
}

func (c *LicenseConfig) Copy() *LicenseConfig {
	if c == nil {
		return nil
	}

	nc := *c
	nc.AdditionalPubKeys = slices.Clone(c.AdditionalPubKeys)
	return &nc
}
