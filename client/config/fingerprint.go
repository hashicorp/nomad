// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// validEnvFingerprinters contains the fingerprinters that are valid
// environment fingerprinters and is used for input validation.
var validEnvFingerprinters = []string{
	"env_aws",
	"env_azure",
	"env_gce",
	"env_digitalocean",
}

// Fingerprint is an optional configuration block for environment fingerprinters
// can control retry behavior and failure handling.
type Fingerprint struct {

	// Name is the fingerprinter identifier that this configuration block
	// relates to. It is gathered from the HCL block label.
	Name string `hcl:",key"`

	// RetryInterval specifies the time to wait between fingerprint
	// attempts.
	RetryInterval    time.Duration
	RetryIntervalHCL string `hcl:"retry_interval,optional"`

	// RetryAttempts specifies the maximum number of fingerprint attempts to be
	// made before the failure is considered terminal.
	RetryAttempts int `hcl:"retry_attempts,optional"`

	// ExitOnFailure indicates whether the fingerprinter should cause the agent
	// to exit if it fails to correctly perform its fingerprint run. This is
	// useful if the fingerprinter provides critical information used by Nomad
	// workloads.
	ExitOnFailure *bool `hcl:"exit_on_failure,optional"`

	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

// Copy is used to satisfy to helper.Copyable interface, so we can perform
// copies of the fingerprint config slice.
func (f *Fingerprint) Copy() *Fingerprint {
	if f == nil {
		return nil
	}

	c := new(Fingerprint)
	*c = *f
	return c
}

// Merge is used to combine two fingerprint blocks with the block passed into
// the function taking precedence. The name is not overwritten as this is
// expected to match as it's the block label. It is the callers responsibility
// to ensure the two fingerprint blocks are for the same fingerprinter
// implementation.
func (f *Fingerprint) Merge(z *Fingerprint) *Fingerprint {
	if f == nil {
		return z
	}

	result := *f

	if z == nil {
		return &result
	}

	if z.RetryInterval != 0 {
		result.RetryInterval = z.RetryInterval
	}
	if z.RetryIntervalHCL != "" {
		result.RetryIntervalHCL = z.RetryIntervalHCL
	}
	if z.RetryAttempts != 0 {
		result.RetryAttempts = z.RetryAttempts
	}
	if z.ExitOnFailure != nil {
		result.ExitOnFailure = z.ExitOnFailure
	}

	return &result
}

// Validate the fingerprint block to ensure we do not have any values that
// cannot be handled.
func (f *Fingerprint) Validate() error {

	if f == nil {
		return nil
	}

	if f.Name == "" {
		return errors.New("fingerprint name cannot be empty")
	}
	if !slices.Contains(validEnvFingerprinters, f.Name) {
		return fmt.Errorf("fingerprint %q does not support configuration", f.Name)
	}
	if f.RetryInterval < 0 {
		return fmt.Errorf("fingerprint %q retry interval cannot be negative", f.Name)
	}
	if f.RetryAttempts < -1 {
		return fmt.Errorf("fingerprint %q retry attempts cannot be less than -1", f.Name)
	}
	if len(f.ExtraKeysHCL) > 0 {
		return fmt.Errorf("fingerprint %q contains unknown configuration options: %s",
			f.Name, strings.Join(f.ExtraKeysHCL, ","))
	}

	return nil
}
