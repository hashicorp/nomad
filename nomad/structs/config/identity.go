package config

import "slices"

// WorkloadIdentityConfig is the jobspec block which determines if and how a workload
// identity is exposed to tasks similar to the Vault block.
//
// This is a copy of WorkloadIdentityConfig from nomad/structs package in order to
// avoid import cycles.
type WorkloadIdentityConfig struct {
	// Audience is the valid recipients for this identity (the "aud" JWT claim)
	// and defaults to the identity's name.
	Audience []string `mapstructure:"aud"`

	// Env injects the Workload Identity into the Task's environment if
	// set.
	Env bool `mapstructure:"env"`

	// File writes the Workload Identity into the Task's secrets directory
	// if set.
	File bool `mapstructure:"file"`
}

func (wi *WorkloadIdentityConfig) Copy() *WorkloadIdentityConfig {
	if wi == nil {
		return nil
	}
	return &WorkloadIdentityConfig{
		Audience: slices.Clone(wi.Audience),
		Env:      wi.Env,
		File:     wi.File,
	}
}

func (wi *WorkloadIdentityConfig) Equal(other *WorkloadIdentityConfig) bool {
	if wi == nil || other == nil {
		return wi == other
	}

	if !slices.Equal(wi.Audience, other.Audience) {
		return false
	}
	if wi.Env != other.Env {
		return false
	}
	if wi.File != other.File {
		return false
	}

	return true
}
